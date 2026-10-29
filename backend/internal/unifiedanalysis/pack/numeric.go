package pack

import (
	"math"

	"github.com/jackc/pgx/v5/pgtype"
)

// numericToFloat converts a pgtype.Numeric to float64, treating invalid/NaN/Inf as 0.
func numericToFloat(n pgtype.Numeric) float64 {
	if v := numericToFloatPtr(n); v != nil {
		return *v
	}
	return 0
}

// pgInt8 converts a pgtype.Int8 to int64, treating invalid as 0.
func pgInt8(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

// numericToFloatPtr converts a pgtype.Numeric to *float64, or nil when unavailable.
func numericToFloatPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return nil
	}
	if math.IsNaN(v.Float64) || math.IsInf(v.Float64, 0) {
		return nil
	}
	value := round2(v.Float64)
	return &value
}
