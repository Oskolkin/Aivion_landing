package dashboardhome

import "testing"

func TestStringSliceOrEmpty_nil(t *testing.T) {
	got := stringSliceOrEmpty(nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("got=%v want empty non-nil slice", got)
	}
}

func TestStringSliceOrEmpty_preserves(t *testing.T) {
	in := []string{"a", "b"}
	got := stringSliceOrEmpty(in)
	if len(got) != 2 || got[0] != "a" {
		t.Fatalf("got=%v", got)
	}
}
