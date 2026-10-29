package chat

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestRequestHashRejectsDifferentPayloadForSameKey(t *testing.T) {
	userID, sessionID := int64(7), int64(11)
	first := RequestHash(&userID, 42, &sessionID, "inventory", "  Покажи  остатки ", []string{"b", "a"}, "ask")
	reordered := RequestHash(&userID, 42, &sessionID, "inventory", "Покажи остатки", []string{"a", "b"}, "ask")
	reused := RequestHash(&userID, 42, &sessionID, "inventory", "Покажи продажи", []string{"a", "b"}, "ask")

	if first != reordered {
		t.Fatal("normalization and attachment ordering must not alter the request fingerprint")
	}
	if first == reused {
		t.Fatal("a reused idempotency key must have a distinct fingerprint for a different payload")
	}
}

func TestExpiredLeaseCanBeReclaimed(t *testing.T) {
	now := time.Now().UTC()
	if !leaseExpired(pgtype.Timestamptz{Time: now.Add(-time.Second), Valid: true}, now) {
		t.Fatal("expired lease should be reclaimable")
	}
	if leaseExpired(pgtype.Timestamptz{Time: now.Add(time.Second), Valid: true}, now) {
		t.Fatal("active lease must not be reclaimable")
	}
}
