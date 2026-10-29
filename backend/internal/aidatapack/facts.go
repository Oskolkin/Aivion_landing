package aidatapack

import (
	"fmt"
	"reflect"
	"strings"
)

// Builder accumulates facts and table references during pack construction.
type Builder struct {
	windows      DataWindows
	factRegistry map[string]Fact
}

func NewBuilder(windows DataWindows) *Builder {
	return &Builder{
		windows:      windows,
		factRegistry: make(map[string]Fact),
	}
}

// AddFact validates, formats and stores a fact. Returns fact_id or error on conflict.
func (b *Builder) AddFact(f Fact) (string, error) {
	id, err := normalizeFactID(f.FactID)
	if err != nil {
		return "", err
	}
	f.FactID = id
	if f.FormattedValue == "" && f.Unit != "" {
		f.FormattedValue = FormatValue(f.Unit, f.Value)
	}
	if existing, ok := b.factRegistry[id]; ok {
		if !factValuesEqual(existing.Value, f.Value) {
			return "", fmt.Errorf("conflicting fact_id %q: existing=%v new=%v", id, existing.Value, f.Value)
		}
		return id, nil
	}
	if err := validateFactValue(f.Value); err != nil {
		return "", fmt.Errorf("fact %q: %w", id, err)
	}
	b.factRegistry[id] = f
	return id, nil
}

func factValuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)
	if av.Kind() == reflect.Float64 && bv.Kind() == reflect.Float64 {
		return av.Float() == bv.Float()
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func validateFactValue(v any) error {
	switch x := v.(type) {
	case float64:
		if isBadFloat(x) {
			return fmt.Errorf("invalid float value")
		}
	case float32:
		if isBadFloat(float64(x)) {
			return fmt.Errorf("invalid float value")
		}
	case *float64:
		if x != nil && isBadFloat(*x) {
			return fmt.Errorf("invalid float pointer value")
		}
	}
	return nil
}

func isBadFloat(v float64) bool {
	return v != v || v > 1e308 || v < -1e308
}

func (b *Builder) FactRegistry() map[string]Fact {
	return b.factRegistry
}

func (b *Builder) Windows() DataWindows {
	return b.windows
}

// addNumericFact is a helper for common numeric facts.
func (b *Builder) addNumericFact(entityType, entityKey, metric, window string, value float64, unit string, source FactSource, dateFrom, dateTo string) (string, error) {
	parts := []string{entityType}
	if entityType == "sku" || entityType == "campaign" || entityType == "category" {
		parts = []string{entityType, entityKey}
	} else if entityType == "account" {
		parts = []string{"account"}
	} else if entityType == "funnel" {
		parts = []string{"funnel", "account"}
	}
	if window != "" {
		parts = append(parts, window)
	}
	parts = append(parts, metric)
	id := strings.Join(parts, ".")
	return b.AddFact(Fact{
		FactID:     id,
		EntityType: entityType,
		EntityID:   entityIDForType(entityType, entityKey),
		Metric:     metric,
		Window:     window,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
		Value:      value,
		Unit:       unit,
		Source:     source,
	})
}

func entityIDForType(entityType, key string) string {
	switch entityType {
	case "account":
		return "account:seller"
	case "funnel":
		return "funnel:account"
	case "sku":
		return skuEntityID(key)
	case "campaign":
		return campaignEntityID(key)
	case "category":
		return categoryEntityID(key)
	default:
		return entityType + ":" + key
	}
}
