package descriptioncategories

import (
	"encoding/json"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

func TestFlattenTree_nestedCategoriesAndTypes(t *testing.T) {
	raw := `[
	  {"description_category_id":10,"category_name":"Root","disabled":false,"children":[
	    {"description_category_id":20,"category_name":"Child","disabled":false,"children":[
	      {"type_id":30,"type_name":"LeafType","disabled":false,"children":[]}
	    ]}
	  ]}
	]`
	var roots []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &roots); err != nil {
		t.Fatal(err)
	}
	var nodes []ozon.DescriptionCategoryTreeJSONNode
	for _, r := range roots {
		var n ozon.DescriptionCategoryTreeJSONNode
		if err := json.Unmarshal(r, &n); err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, n)
	}
	rows := FlattenTree(nodes)
	if len(rows) < 3 {
		t.Fatalf("expected >=3 rows, got %d", len(rows))
	}
	seen := map[int64]bool{}
	for _, row := range rows {
		seen[row.ID] = true
	}
	for _, id := range []int64{10, 20, 30} {
		if !seen[id] {
			t.Fatalf("missing id %d in %+v", id, rows)
		}
	}
}
