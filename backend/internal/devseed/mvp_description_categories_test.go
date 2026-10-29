package devseed

import "testing"

func TestMVPDescriptionCategoryIDs_matchCommerceMapping(t *testing.T) {
	sellerID := int64(1)
	for cat, wantName := range mvpCategoryDisplayNames {
		id := categoryDescriptionID(cat, sellerID)
		if id <= 0 {
			t.Fatalf("bad id for %s", cat)
		}
		if wantName == "" {
			t.Fatalf("missing display name for %s", cat)
		}
	}
}
