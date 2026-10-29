package catalog

import (
	"encoding/json"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

func taxonomyChild(t *testing.T, node ozon.DescriptionCategoryTreeJSONNode) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestNormalizeTaxonomyHierarchyLeafAndProductTypes(t *testing.T) {
	typeLeaf := ozon.DescriptionCategoryTreeJSONNode{TypeID: 900, TypeName: "Shoes"}
	child := ozon.DescriptionCategoryTreeJSONNode{DescriptionCategoryID: 20, CategoryName: "Footwear", Children: []json.RawMessage{taxonomyChild(t, typeLeaf)}}
	root := ozon.DescriptionCategoryTreeJSONNode{DescriptionCategoryID: 10, CategoryName: "Fashion", Children: []json.RawMessage{taxonomyChild(t, child)}}

	categories, productTypes, err := NormalizeTaxonomy([]ozon.DescriptionCategoryTreeJSONNode{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 2 || categories[0].ExternalID != 10 || categories[0].IsLeaf || categories[1].ParentExternalID != 10 || !categories[1].IsLeaf {
		t.Fatalf("categories=%+v", categories)
	}
	if len(productTypes) != 1 || productTypes[0].ExternalID != 900 || productTypes[0].CategoryExternalID != 20 {
		t.Fatalf("product types=%+v", productTypes)
	}
	if productTypes[0].ExternalID == categories[1].ExternalID {
		t.Fatal("product type must remain supporting context, not category identity")
	}
}

func TestTaxonomyHashDeterministicAndSensitiveToChange(t *testing.T) {
	a := []ozon.DescriptionCategoryTreeJSONNode{{DescriptionCategoryID: 2, CategoryName: "B"}, {DescriptionCategoryID: 1, CategoryName: "A"}}
	b := []ozon.DescriptionCategoryTreeJSONNode{{DescriptionCategoryID: 1, CategoryName: "A"}, {DescriptionCategoryID: 2, CategoryName: "B"}}
	if TaxonomyHash(a) != TaxonomyHash(b) {
		t.Fatal("equivalent tree order produced different normalized hash")
	}
	b[0].CategoryName = "Changed"
	if TaxonomyHash(a) == TaxonomyHash(b) {
		t.Fatal("taxonomy change did not change hash")
	}
}
