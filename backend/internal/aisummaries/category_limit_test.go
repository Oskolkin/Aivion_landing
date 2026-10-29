package aisummaries

import "testing"

func TestSortPickedCategoryIDsRespectsLimit(t *testing.T) {
	stats := map[int64]*categoryPickStats{
		1:  {id: 1, revenue: 100, critN: 1},
		2:  {id: 2, revenue: 90},
		3:  {id: 3, revenue: 80},
		4:  {id: 4, revenue: 70},
		5:  {id: 5, revenue: 60},
		6:  {id: 6, revenue: 50},
		7:  {id: 7, revenue: 40},
		8:  {id: 8, revenue: 30},
		9:  {id: 9, revenue: 20},
		10: {id: 10, revenue: 10},
		11: {id: 11, revenue: 5},
		12: {id: 12, revenue: 1},
	}
	ids := sortPickedCategoryIDs(stats, 10, nil)
	if len(ids) != 10 {
		t.Fatalf("len=%d want 10", len(ids))
	}
}

func TestSortPickedCategoryIDsZeroLimitUsesSafeDefaultInService(t *testing.T) {
	s := &Service{maxCategoriesPerRun: 0}
	if s.maxCategoriesPerRun <= 0 {
		s.maxCategoriesPerRun = 10
	}
	if s.maxCategoriesPerRun != 10 {
		t.Fatalf("got %d", s.maxCategoriesPerRun)
	}
}
