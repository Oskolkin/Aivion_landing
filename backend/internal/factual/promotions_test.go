package factual

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestKnownToAsOfHidesFutureClosures(t *testing.T) {
	cutoff := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	if knownToAsOf(pgtype.Timestamptz{}, cutoff) != nil {
		t.Fatal("invalid known_to must stay nil")
	}
	future := pgtype.Timestamptz{Time: cutoff.Add(time.Hour), Valid: true}
	if knownToAsOf(future, cutoff) != nil {
		t.Fatal("closure after cutoff must not leak")
	}
	past := pgtype.Timestamptz{Time: cutoff.Add(-time.Hour), Valid: true}
	got := knownToAsOf(past, cutoff)
	if got == nil || !got.Equal(past.Time) {
		t.Fatalf("closure at/before cutoff must be visible: %v", got)
	}
}
