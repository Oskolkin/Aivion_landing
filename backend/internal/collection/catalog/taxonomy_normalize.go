package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type CategoryNode struct {
	ExternalID       int64
	ParentExternalID int64
	Name             string
	Disabled         bool
	IsLeaf           bool
}

type ProductTypeNode struct {
	ExternalID         int64
	CategoryExternalID int64
	Name               string
	Disabled           bool
}

func NormalizeTaxonomy(roots []ozon.DescriptionCategoryTreeJSONNode) ([]CategoryNode, []ProductTypeNode, error) {
	categories := map[int64]CategoryNode{}
	types := map[[2]int64]ProductTypeNode{}
	var walk func(ozon.DescriptionCategoryTreeJSONNode, int64) error
	walk = func(node ozon.DescriptionCategoryTreeJSONNode, parentCategoryID int64) error {
		categoryID := parentCategoryID
		if node.DescriptionCategoryID > 0 {
			categoryID = node.DescriptionCategoryID
			categories[categoryID] = CategoryNode{ExternalID: categoryID, ParentExternalID: parentCategoryID, Name: strings.TrimSpace(node.CategoryName), Disabled: node.Disabled, IsLeaf: true}
			if parent, ok := categories[parentCategoryID]; ok {
				parent.IsLeaf = false
				categories[parentCategoryID] = parent
			}
		}
		if node.TypeID > 0 {
			if categoryID <= 0 {
				return fmt.Errorf("product type %d has no description category parent", node.TypeID)
			}
			name := strings.TrimSpace(node.TypeName)
			if name == "" {
				name = strings.TrimSpace(node.CategoryName)
			}
			types[[2]int64{categoryID, node.TypeID}] = ProductTypeNode{ExternalID: node.TypeID, CategoryExternalID: categoryID, Name: name, Disabled: node.Disabled}
		}
		for _, raw := range node.Children {
			var child ozon.DescriptionCategoryTreeJSONNode
			if err := json.Unmarshal(raw, &child); err != nil {
				return fmt.Errorf("decode taxonomy child: %w", err)
			}
			if err := walk(child, categoryID); err != nil {
				return err
			}
		}
		return nil
	}
	for _, root := range roots {
		if err := walk(root, 0); err != nil {
			return nil, nil, err
		}
	}
	categoryRows := make([]CategoryNode, 0, len(categories))
	for _, v := range categories {
		categoryRows = append(categoryRows, v)
	}
	typeRows := make([]ProductTypeNode, 0, len(types))
	for _, v := range types {
		typeRows = append(typeRows, v)
	}
	sort.Slice(categoryRows, func(i, j int) bool { return categoryRows[i].ExternalID < categoryRows[j].ExternalID })
	sort.Slice(typeRows, func(i, j int) bool {
		if typeRows[i].CategoryExternalID == typeRows[j].CategoryExternalID {
			return typeRows[i].ExternalID < typeRows[j].ExternalID
		}
		return typeRows[i].CategoryExternalID < typeRows[j].CategoryExternalID
	})
	return categoryRows, typeRows, nil
}

func normalizedHash(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
