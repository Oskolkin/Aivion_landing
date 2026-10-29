package ozon

import (
	"context"
	"encoding/json"
)

// ProductInfoListRequestIDs identifies products for POST /v3/product/info/list.
type ProductInfoListRequestIDs struct {
	ProductID []int64  `json:"product_id,omitempty"`
	OfferID   []string `json:"offer_id,omitempty"`
	SKU       []int64  `json:"sku,omitempty"`
}

type ProductInfoStatus struct {
	IsCreated         bool   `json:"is_created"`
	ModerateStatus    string `json:"moderate_status"`
	State             string `json:"state"`
	Status            string `json:"status"`
	StatusDescription string `json:"status_description"`
	StatusFailed      string `json:"status_failed"`
	StatusName        string `json:"status_name"`
	StatusTooltip     string `json:"status_tooltip"`
	StatusUpdatedAt   string `json:"status_updated_at"`
	ValidationStatus  string `json:"validation_status"`
}

type ProductInfoVisibility struct {
	HasPrice bool `json:"has_price"`
	HasStock bool `json:"has_stock"`
}

type ProductInfoAvailability struct {
	Availability string   `json:"availability"`
	Reasons      []string `json:"reasons"`
	Source       string   `json:"source"`
	SKU          int64    `json:"sku"`
}

type ProductInfoPriceIndexData struct {
	MinimalPrice    string `json:"minimal_price"`
	PriceIndexValue string `json:"price_index_value"`
}

type ProductInfoPriceIndexes struct {
	ColorIndex                string                    `json:"color_index"`
	ExternalIndexData         ProductInfoPriceIndexData `json:"external_index_data"`
	OzonIndexData             ProductInfoPriceIndexData `json:"ozon_index_data"`
	SelfMarketplacesIndexData ProductInfoPriceIndexData `json:"self_marketplaces_index_data"`
}

type ProductInfoCommission struct {
	SaleSchema     string `json:"sale_schema"`
	Percent        int    `json:"percent"`
	Value          int    `json:"value"`
	DeliveryAmount int    `json:"delivery_amount"`
	ReturnAmount   int    `json:"return_amount"`
}

type ProductInfoPromotion struct {
	IsEnabled bool   `json:"is_enabled"`
	Type      string `json:"type"`
}

type ProductInfoErrorText struct {
	Message          string `json:"message"`
	Description      string `json:"description"`
	ShortDescription string `json:"short_description"`
	AttributeName    string `json:"attribute_name"`
}

type ProductInfoError struct {
	AttributeID int64                `json:"attribute_id"`
	Code        string               `json:"code"`
	Field       string               `json:"field"`
	Level       string               `json:"level"`
	State       string               `json:"state"`
	Texts       ProductInfoErrorText `json:"texts"`
}

type ProductInfoStockRow struct {
	Present  int32  `json:"present"`
	Reserved int32  `json:"reserved"`
	SKU      int64  `json:"sku"`
	Source   string `json:"source"`
}

type ProductInfoStocks struct {
	HasStock bool                  `json:"has_stock"`
	Stocks   []ProductInfoStockRow `json:"stocks"`
}

// ProductInfoListItem is the enriched product card from POST /v3/product/info/list.
type ProductInfoListItem struct {
	ID                    int64                     `json:"id"`
	OfferID               string                    `json:"offer_id"`
	SKU                   int64                     `json:"sku"`
	Name                  string                    `json:"name"`
	Description           string                    `json:"description"`
	DescriptionCategoryID int64                     `json:"description_category_id"`
	TypeID                int64                     `json:"type_id"`
	CreatedAt             string                    `json:"created_at"`
	UpdatedAt             string                    `json:"updated_at"`
	Price                 string                    `json:"price"`
	OldPrice              string                    `json:"old_price"`
	MinPrice              string                    `json:"min_price"`
	CurrencyCode          string                    `json:"currency_code"`
	Vat                   string                    `json:"vat"`
	IsArchived            bool                      `json:"is_archived"`
	IsAutoarchived        bool                      `json:"is_autoarchived"`
	IsDiscounted          bool                      `json:"is_discounted"`
	HasDiscountedFBOItem  bool                      `json:"has_discounted_fbo_item"`
	DiscountedFBOStocks   int                       `json:"discounted_fbo_stocks"`
	IsKGT                 bool                      `json:"is_kgt"`
	IsSuper               bool                      `json:"is_super"`
	Statuses              ProductInfoStatus         `json:"statuses"`
	VisibilityDetails     ProductInfoVisibility     `json:"visibility_details"`
	Availabilities        []ProductInfoAvailability `json:"availabilities"`
	PriceIndexes          ProductInfoPriceIndexes   `json:"price_indexes"`
	Commissions           []ProductInfoCommission   `json:"commissions"`
	Promotions            []ProductInfoPromotion    `json:"promotions"`
	PrimaryImage          FlexibleStringList        `json:"primary_image"`
	Images                FlexibleStringList        `json:"images"`
	Images360             FlexibleStringList        `json:"images360"`
	ColorImage            FlexibleStringList        `json:"color_image"`
	Stocks                ProductInfoStocks         `json:"stocks"`
	Errors                []ProductInfoError        `json:"errors"`
	Barcodes              []string                  `json:"barcodes"`
	ModelInfo             json.RawMessage           `json:"model_info"`
	Sources               json.RawMessage           `json:"sources"`
	VolumeWeight          float64                   `json:"volume_weight"`
	// Legacy field used by older parsers.
	Status struct {
		State string `json:"state"`
	} `json:"status"`
	Archived bool `json:"archived"`
}

type ProductInfoListResult struct {
	Items []ProductInfoListItem `json:"items"`
}

// GetProductInfoList fetches enriched product cards (max 1000 IDs per request).
func (c *Client) GetProductInfoList(
	ctx context.Context,
	clientID string,
	apiKey string,
	ids ProductInfoListRequestIDs,
) (*TypedResponse[ProductInfoListResult], error) {
	req := ProductInfoListRequest{
		ProductID: ids.ProductID,
		OfferID:   ids.OfferID,
		SKU:       ids.SKU,
	}
	return c.ListProductsInfo(ctx, clientID, apiKey, req)
}
