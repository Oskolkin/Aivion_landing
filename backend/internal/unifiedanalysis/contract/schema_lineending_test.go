package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/lineendings"
)

func TestSchemaHash_CanonicalV10(t *testing.T) {
	h, err := SchemaHash()
	if err != nil {
		t.Fatal(err)
	}
	if h != CanonicalSchemaHashV10 {
		t.Fatalf("SchemaHash=%s want %s", h, CanonicalSchemaHashV10)
	}
}

func TestSchemaHash_LineEndingIndependent(t *testing.T) {
	raw, err := SchemaJSON()
	if err != nil {
		t.Fatal(err)
	}
	lf := append([]byte(nil), raw...)
	crlf := []byte(strings.ReplaceAll(string(raw), "\n", "\r\n"))
	bare := []byte(strings.ReplaceAll(string(raw), "\n", "\r"))

	hashOf := func(b []byte) string {
		n := lineendings.NormalizeLineEndingsBytes(b)
		sum := sha256.Sum256(n)
		return "sha256:" + hex.EncodeToString(sum[:])
	}
	lfH := hashOf(lf)
	crlfH := hashOf(crlf)
	bareH := hashOf(bare)
	if lfH != crlfH || lfH != bareH {
		t.Fatalf("LF=%s CRLF=%s bare=%s", lfH, crlfH, bareH)
	}
	if lfH != CanonicalSchemaHashV10 {
		t.Fatalf("normalized hash=%s want %s", lfH, CanonicalSchemaHashV10)
	}
}

func TestSchemaJSONBytesMatchSchemaHash(t *testing.T) {
	raw, err := SchemaJSON()
	if err != nil {
		t.Fatal(err)
	}
	h, err := SchemaHash()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	got := "sha256:" + hex.EncodeToString(sum[:])
	if got != h {
		t.Fatalf("sha256(SchemaJSON())=%s SchemaHash()=%s", got, h)
	}
	if strings.Contains(string(raw), "\r") {
		t.Fatal("SchemaJSON must return LF-normalized bytes")
	}
	var probe any
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("SchemaJSON not valid JSON: %v", err)
	}
}
