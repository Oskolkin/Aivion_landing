package aisummaries

import "testing"

func TestSortPickedCategoryIDs_priorityOverRevenue(t *testing.T) {
	t.Parallel()
	stats := map[int64]*categoryPickStats{
		1: {id: 1, revenue: 1000, orders: 1, withStock: 1, active: 1},
		2: {id: 2, revenue: 1, orders: 1, withStock: 1, active: 1, highN: 1},
	}
	out := sortPickedCategoryIDs(stats, 10, nil)
	if len(out) != 2 || out[0] != 2 {
		t.Fatalf("got %v want [2 1]", out)
	}
}

func TestSortPickedCategoryIDs_truncationNote(t *testing.T) {
	t.Parallel()
	stats := map[int64]*categoryPickStats{}
	for i := int64(1); i <= 5; i++ {
		stats[i] = &categoryPickStats{id: i, revenue: float64(100 - i)}
	}
	var notes []string
	out := sortPickedCategoryIDs(stats, 3, &notes)
	if len(out) != 3 {
		t.Fatalf("len %d", len(out))
	}
	if len(notes) != 1 {
		t.Fatalf("expected truncation note, got %+v", notes)
	}
}
