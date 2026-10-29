package actionview

import "testing"

func TestViewIDRoundTrip(t *testing.T) {
	id := EncodeGroup("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	ref, err := Decode(id)
	if err != nil || ref.Kind != "g" || ref.ID == "" {
		t.Fatalf("%v %v", ref, err)
	}
	if _, err := Decode("not-valid"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCanonicalCatalogHas19(t *testing.T) {
	if n := len(CanonicalActionTypes()); n != 19 {
		t.Fatal(n)
	}
}
