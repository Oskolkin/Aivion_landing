// Package canonicaljson implements the byte-level JSON contract used for hashes.
package canonicaljson

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// Date is serialized as an ISO calendar date.
type Date time.Time

// Decimal is a base-10 value. It is always encoded as a JSON string.
type Decimal string

var decimalPattern = regexp.MustCompile(`^[+-]?[0-9]+(?:\.[0-9]+)?$`)

func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	*d = Date(v)
	return nil
}

func (d Decimal) canonical() (string, error) {
	s := strings.TrimSpace(string(d))
	if s == "" {
		return "", fmt.Errorf("canonicaljson: empty decimal")
	}
	if !decimalPattern.MatchString(s) {
		return "", fmt.Errorf("canonicaljson: invalid decimal %q", s)
	}
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(strings.TrimPrefix(s, "-"), "+")
	parts := strings.SplitN(s, ".", 2)
	parts[0] = strings.TrimLeft(parts[0], "0")
	if parts[0] == "" {
		parts[0] = "0"
	}
	if len(parts) == 2 {
		parts[1] = strings.TrimRight(parts[1], "0")
		if parts[1] != "" {
			s = parts[0] + "." + parts[1]
		} else {
			s = parts[0]
		}
	} else {
		s = parts[0]
	}
	if negative && s != "0" {
		s = "-" + s
	}
	return s, nil
}

// Marshal returns deterministic UTF-8 JSON without a trailing newline.
func Marshal(v any) ([]byte, error) {
	n, err := normalize(reflect.ValueOf(v))
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "")
	if err = enc.Encode(n); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte{'\n'}), nil
}

// Hash returns the SHA-256 hexadecimal digest, or an empty string on invalid input.
func Hash(v any) string {
	b, err := Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func normalize(v reflect.Value) (any, error) {
	if !v.IsValid() || ((v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) && v.IsNil()) {
		return nil, nil
	}
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		return normalize(v.Elem())
	}
	if v.Type() == reflect.TypeOf(time.Time{}) {
		return v.Interface().(time.Time).UTC().Format(time.RFC3339Nano), nil
	}
	if v.Type() == reflect.TypeOf(Date{}) {
		return time.Time(v.Interface().(Date)).Format("2006-01-02"), nil
	}
	if v.Type() == reflect.TypeOf(Decimal("")) {
		return v.Interface().(Decimal).canonical()
	}
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		return nil, fmt.Errorf("canonicaljson: floats are forbidden")
	case reflect.Bool:
		return v.Bool(), nil
	case reflect.String:
		return v.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint(), nil
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			return nil, nil
		}
		out := make([]any, v.Len())
		for i := range out {
			x, err := normalize(v.Index(i))
			if err != nil {
				return nil, err
			}
			out[i] = x
		}
		return out, nil
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("canonicaljson: object keys must be strings")
		}
		if v.IsNil() {
			return nil, nil
		}
		out := make(map[string]any, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			x, err := normalize(iter.Value())
			if err != nil {
				return nil, err
			}
			out[iter.Key().String()] = x
		}
		return out, nil
	case reflect.Struct:
		out := map[string]any{}
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" {
				continue
			}
			name, opts := jsonField(f)
			if name == "-" {
				continue
			}
			fv := v.Field(i)
			if opts["omitempty"] && fv.IsZero() {
				continue
			}
			x, err := normalize(fv)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f.Name, err)
			}
			out[name] = x
		}
		return out, nil
	default:
		return nil, fmt.Errorf("canonicaljson: unsupported type %s", v.Type())
	}
}

func jsonField(f reflect.StructField) (string, map[string]bool) {
	parts := strings.Split(f.Tag.Get("json"), ",")
	name := parts[0]
	if name == "" {
		name = f.Name
	}
	opts := map[string]bool{}
	for _, p := range parts[1:] {
		opts[p] = true
	}
	return name, opts
}
