package actionview

import (
	"fmt"
	"testing"
)

func TestPageGroupsThreePagesUniqueStable(t *testing.T) {
	all := make([]GroupView, 8)
	for i := 0; i < 8; i++ {
		all[i] = GroupView{ActionViewID: EncodeGroup(fmt.Sprintf("g%d", i+1)), DisplayOrder: i + 1, DisplayTitle: fmt.Sprintf("G%d", i+1)}
	}
	sortGroupViews(all)
	f := ListFilter{Status: "active", Limit: 3}
	var seen []string
	cursor := ""
	pages := 0
	for {
		f.Cursor = cursor
		page, next, more, err := pageGroups(all, 9, f)
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, g := range page {
			seen = append(seen, g.ActionViewID)
		}
		if !more {
			break
		}
		cursor = next
		if pages > 10 {
			t.Fatal("loop")
		}
	}
	if pages != 3 {
		t.Fatalf("pages=%d seen=%d", pages, len(seen))
	}
	if len(seen) != 8 {
		t.Fatalf("n=%d", len(seen))
	}
	uniq := map[string]struct{}{}
	for _, id := range seen {
		if _, ok := uniq[id]; ok {
			t.Fatalf("dup %s", id)
		}
		uniq[id] = struct{}{}
	}
	mismatch := encodeCursor(pageCursor{Seller: 8, Status: "active", Family: "", Query: "", Sort: sortAivion, Order: 1, ID: seen[0]})
	if _, _, _, err := pageGroups(all, 9, ListFilter{Status: "active", Limit: 3, Cursor: mismatch}); err == nil {
		t.Fatal("want cursor mismatch")
	}
}
