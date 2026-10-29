package admin

import (
	"math"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestJSONMapFromRaw(t *testing.T) {
	got := jsonMapFromRaw([]byte(`{"a":1}`))
	if got["a"] != float64(1) {
		t.Fatalf("unexpected decoded map: %#v", got)
	}
}

func TestNumericConversion(t *testing.T) {
	n := numericFromFloat64(12.34)
	got := numericToFloat64(n)
	if math.Abs(got-12.34) > 0.0001 {
		t.Fatalf("unexpected numeric conversion: %f", got)
	}

	var empty pgtype.Numeric
	if numericToFloat64(empty) != 0 {
		t.Fatalf("expected zero for invalid numeric")
	}
}
