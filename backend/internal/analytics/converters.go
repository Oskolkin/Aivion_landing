package analytics

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// int32FromPgInt4 extracts a stored integer metric; invalid pg values are "not present".

func textPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	value := v.String
	return &value
}

func int8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	value := v.Int64
	return &value
}

func timestamptzToRFC3339(v pgtype.Timestamptz) *string {
	if !v.Valid {
		return nil
	}
	s := v.Time.UTC().Format(time.RFC3339)
	return &s
}

func int32FromPgInt4(v pgtype.Int4) (int32, bool) {
	if !v.Valid {
		return 0, false
	}
	return v.Int32, true
}
