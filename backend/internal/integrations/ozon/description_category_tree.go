package ozon

import (
	"context"
	"encoding/json"
	"net/http"
)

// DescriptionCategoryTreeJSONNode mirrors Ozon Seller API /v1/description-category/tree nodes.
// Nodes may represent description categories or product types (leaves use type_id/type_name).
type DescriptionCategoryTreeJSONNode struct {
	DescriptionCategoryID int64             `json:"description_category_id"`
	CategoryName          string            `json:"category_name"`
	TypeID                int64             `json:"type_id"`
	TypeName              string            `json:"type_name"`
	Disabled              bool              `json:"disabled"`
	Children              []json.RawMessage `json:"children"`
}

type descriptionCategoryTreeEnvelope struct {
	Result []json.RawMessage `json:"result"`
}

// GetDescriptionCategoryTree calls POST /v1/description-category/tree.
func (c *Client) GetDescriptionCategoryTree(ctx context.Context, clientID, apiKey string) (*TypedResponse[[]DescriptionCategoryTreeJSONNode], error) {
	var env descriptionCategoryTreeEnvelope
	rawResp, err := c.doJSON(ctx, clientID, apiKey, http.MethodPost, "/v1/description-category/tree", map[string]any{}, &env)
	if err != nil {
		return nil, err
	}
	nodes := make([]DescriptionCategoryTreeJSONNode, 0, len(env.Result))
	for _, raw := range env.Result {
		var n DescriptionCategoryTreeJSONNode
		if err := json.Unmarshal(raw, &n); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return &TypedResponse[[]DescriptionCategoryTreeJSONNode]{
		Raw:  rawResp.Body,
		Data: nodes,
		Meta: rawResp.Meta,
	}, nil
}

func (c *Client) descriptionCategoryTreeBearer(ctx context.Context, accessToken string) (*TypedResponse[[]DescriptionCategoryTreeJSONNode], error) {
	var env descriptionCategoryTreeEnvelope
	rawResp, err := c.doJSONBearer(ctx, accessToken, http.MethodPost, "/v1/description-category/tree", map[string]any{}, &env)
	if err != nil {
		return nil, err
	}
	nodes := make([]DescriptionCategoryTreeJSONNode, 0, len(env.Result))
	for _, raw := range env.Result {
		var node DescriptionCategoryTreeJSONNode
		if err := json.Unmarshal(raw, &node); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return &TypedResponse[[]DescriptionCategoryTreeJSONNode]{Raw: rawResp.Body, Data: nodes, Meta: rawResp.Meta}, nil
}
