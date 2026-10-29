package config

import "testing"

func TestResolveSummaryMaxCategoriesPerRun(t *testing.T) {
	if resolveSummaryMaxCategoriesPerRun(0) != 10 {
		t.Fatal("zero => 10")
	}
	if resolveSummaryMaxCategoriesPerRun(-1) != 10 {
		t.Fatal("negative => 10")
	}
	if resolveSummaryMaxCategoriesPerRun(15) != 15 {
		t.Fatal("positive preserved")
	}
}
