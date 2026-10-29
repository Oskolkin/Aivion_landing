package devseed

import (
	"testing"
	"time"
)

func TestParseMVPAnchorDate(t *testing.T) {
	ref := time.Date(2026, 5, 14, 15, 30, 0, 0, time.UTC)
	got, err := ParseMVPAnchorDate("", ref)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("empty anchor: got %v want %v", got, want)
	}
	got, err = ParseMVPAnchorDate("today", ref)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(want) {
		t.Fatalf("today: got %v want %v", got, want)
	}
	got, err = ParseMVPAnchorDate("2026-01-02", ref)
	if err != nil {
		t.Fatal(err)
	}
	want2 := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want2) {
		t.Fatalf("fixed date: got %v want %v", got, want2)
	}
	if _, err := ParseMVPAnchorDate("not-a-date", ref); err == nil {
		t.Fatal("expected error")
	}
}
