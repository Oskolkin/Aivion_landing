package aisummaries

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func pgUUID(u uuid.UUID) pgtype.UUID {
	var p pgtype.UUID
	p.Valid = true
	copy(p.Bytes[:], u[:])
	return p
}

func uuidStringFromPg(v pgtype.UUID) (string, error) {
	if !v.Valid {
		return "", fmt.Errorf("invalid uuid")
	}
	u, err := uuid.FromBytes(v.Bytes[:])
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
