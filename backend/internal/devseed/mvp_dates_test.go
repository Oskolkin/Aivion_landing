package devseed

import (
	"testing"
	"time"
)

func TestSeedDateRangeEndsAtAnchorDate(t *testing.T) {
	anchor := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	days := 90
	rng, err := SeedDateRange(anchor, days)
	if err != nil {
		t.Fatal(err)
	}
	if len(rng) != days {
		t.Fatalf("len: got %d want %d", len(rng), days)
	}
	wantStart := time.Date(2026, 2, 21, 0, 0, 0, 0, time.UTC)
	if !rng[0].Equal(wantStart) {
		t.Fatalf("first: got %v want %v", rng[0], wantStart)
	}
	if !rng[len(rng)-1].Equal(anchor) {
		t.Fatalf("last: got %v want %v", rng[len(rng)-1], anchor)
	}
}

func TestSeedDateRangeToday(t *testing.T) {
	ref := time.Date(2026, 5, 21, 15, 30, 0, 0, time.UTC)
	anchor, err := ParseMVPAnchorDate("today", ref)
	if err != nil {
		t.Fatal(err)
	}
	wantAnchor := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	if !anchor.Equal(wantAnchor) {
		t.Fatalf("anchor: got %v want %v", anchor, wantAnchor)
	}
	rng, err := SeedDateRange(anchor, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !rng[len(rng)-1].Equal(wantAnchor) {
		t.Fatalf("range end: got %v want %v", rng[len(rng)-1], wantAnchor)
	}
}

func TestSeedDateRangeNoFutureDates(t *testing.T) {
	anchor := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	rng, err := SeedDateRange(anchor, 90)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range rng {
		if d.After(anchor) {
			t.Fatalf("date %v after anchor %v", d, anchor)
		}
	}
}

func TestSeedStartDate(t *testing.T) {
	anchor := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	got := SeedStartDate(anchor, 90)
	want := time.Date(2026, 2, 21, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestMVPClampToCalendarDayUTC(t *testing.T) {
	day := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	spill := day.Add(30 * time.Hour)
	got := MVPClampToCalendarDayUTC(spill, day)
	want := day.Add(24*time.Hour - time.Second)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
