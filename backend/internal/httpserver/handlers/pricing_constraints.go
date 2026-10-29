// HTTP handlers for pricing constraints — deprecated/future module.
// Not part of active seller-facing MVP; user-facing pricing UI redirects to Products.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/pricingconstraints"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type PricingConstraintsHandler struct {
	service *pricingconstraints.Service
}

func NewPricingConstraintsHandler(service *pricingconstraints.Service) *PricingConstraintsHandler {
	return &PricingConstraintsHandler{service: service}
}

type pricingRulePayload struct {
	MinPrice               *float64 `json:"min_price"`
	MaxPrice               *float64 `json:"max_price"`
	ReferenceMarginPercent *float64 `json:"reference_margin_percent"`
	ReferencePrice         *float64 `json:"reference_price"`
	ImpliedCost            *float64 `json:"implied_cost"`
	IsActive               *bool    `json:"is_active"`
}

type putGlobalRequest struct {
	pricingRulePayload
}

type upsertCategoryRuleRequest struct {
	DescriptionCategoryID int64   `json:"description_category_id"`
	CategoryCode          *string `json:"category_code"`
	pricingRulePayload
}

type upsertSKUOverrideRequest struct {
	SKU       *int64  `json:"sku"`
	ProductID *int64  `json:"product_id"`
	OfferID   *string `json:"offer_id"`
	pricingRulePayload
}

type deactivateRuleRequest struct {
	RuleID int64 `json:"rule_id"`
}

type pricingRuleResponse struct {
	ID                     int64    `json:"id"`
	ScopeType              string   `json:"scope_type"`
	ScopeTargetKind        *string  `json:"scope_target_kind"`
	ScopeTargetID          *int64   `json:"scope_target_id"`
	ScopeTargetCode        *string  `json:"scope_target_code"`
	MinPrice               *float64 `json:"min_price"`
	MaxPrice               *float64 `json:"max_price"`
	ReferenceMarginPercent *float64 `json:"reference_margin_percent"`
	ReferencePrice         *float64 `json:"reference_price"`
	ImpliedCost            *float64 `json:"implied_cost"`
	IsActive               bool     `json:"is_active"`
	UpdatedAt              string   `json:"updated_at"`
}

type pricingRulesResponse struct {
	GlobalDefault   *pricingRuleResponse         `json:"global_default"`
	CategoryRules   []pricingRuleResponse        `json:"category_rules"`
	SKUOverrides    []pricingRuleResponse        `json:"sku_overrides"`
	CategoryOptions []categoryOptionResponse     `json:"category_options"`
	ProductCatalog  []productCatalogItemResponse `json:"product_catalog"`
	Meta            pricingRulesMeta             `json:"meta"`
}

type categoryOptionResponse struct {
	DescriptionCategoryID int64 `json:"description_category_id"`
	ProductsCount         int   `json:"products_count"`
}

type productCatalogItemResponse struct {
	OzonProductID         int64    `json:"ozon_product_id"`
	SKU                   *int64   `json:"sku"`
	OfferID               *string  `json:"offer_id"`
	ProductName           string   `json:"product_name"`
	DescriptionCategoryID *int64   `json:"description_category_id"`
	CurrentPrice          *float64 `json:"current_price"`
}

type pricingRulesMeta struct {
	TotalRules            int     `json:"total_rules"`
	ActiveRules           int     `json:"active_rules"`
	CategoryRulesCount    int     `json:"category_rules_count"`
	SKUOverridesCount     int     `json:"sku_overrides_count"`
	LastRuleUpdateAt      *string `json:"last_rule_update_at"`
	LastRecomputeAt       *string `json:"last_recompute_at"`
	EffectiveRecordsCount int     `json:"effective_records_count"`
}

type upsertRuleResponse struct {
	Rule      pricingRuleResponse                `json:"rule"`
	Recompute pricingconstraints.RecomputeResult `json:"recompute"`
}

type effectiveConstraintResponse struct {
	OzonProductID          int64    `json:"ozon_product_id"`
	SKU                    *int64   `json:"sku"`
	OfferID                *string  `json:"offer_id"`
	ProductName            *string  `json:"product_name"`
	CurrentPrice           *float64 `json:"current_price"`
	ResolvedFromScopeType  string   `json:"resolved_from_scope_type"`
	RuleID                 int64    `json:"rule_id"`
	EffectiveMinPrice      *float64 `json:"effective_min_price"`
	EffectiveMaxPrice      *float64 `json:"effective_max_price"`
	ReferencePrice         *float64 `json:"reference_price"`
	ReferenceMarginPercent *float64 `json:"reference_margin_percent"`
	ImpliedCost            *float64 `json:"implied_cost"`
	ComputedAt             string   `json:"computed_at"`
}

type effectiveListResponse struct {
	Items  []effectiveConstraintResponse `json:"items"`
	Total  int                           `json:"total"`
	Limit  int                           `json:"limit"`
	Offset int                           `json:"offset"`
}

type previewRequest struct {
	ReferencePrice         float64  `json:"reference_price"`
	ReferenceMarginPercent float64  `json:"reference_margin_percent"`
	MinPrice               *float64 `json:"min_price"`
	MaxPrice               *float64 `json:"max_price"`
	InputPrice             *float64 `json:"input_price"`
}

type previewResponse struct {
	ReferencePrice             float64  `json:"reference_price"`
	ReferenceMarginPercent     float64  `json:"reference_margin_percent"`
	ImpliedCost                float64  `json:"implied_cost"`
	ExpectedMarginAtMinPrice   *float64 `json:"expected_margin_at_min_price"`
	ExpectedMarginAtMaxPrice   *float64 `json:"expected_margin_at_max_price"`
	ExpectedMarginAtInputPrice *float64 `json:"expected_margin_at_input_price"`
}

func (h *PricingConstraintsHandler) GetPricingConstraints(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rules, err := h.service.ListRuleSetsBySellerAccountID(r.Context(), sellerAccount.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list pricing constraints")
		return
	}
	effective, err := h.service.ListEffectiveConstraintsBySellerAccountID(r.Context(), sellerAccount.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to read effective constraints")
		return
	}
	products, err := h.service.ListProductsBySellerAccountID(r.Context(), sellerAccount.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to read product catalog")
		return
	}
	categoryCounts := make(map[int64]int)
	productCatalog := make([]productCatalogItemResponse, 0, len(products))
	for _, product := range products {
		var categoryID *int64
		if product.DescriptionCategoryID.Valid {
			categoryID = int8PtrValue(product.DescriptionCategoryID)
			if categoryID != nil {
				categoryCounts[*categoryID]++
			}
		}
		productCatalog = append(productCatalog, productCatalogItemResponse{
			OzonProductID:         product.OzonProductID,
			SKU:                   int8PtrValue(product.Sku),
			OfferID:               textPtrValue(product.OfferID),
			ProductName:           product.Name,
			DescriptionCategoryID: categoryID,
			CurrentPrice:          numericPtrValue(product.ReferencePrice),
		})
	}
	categoryOptions := make([]categoryOptionResponse, 0, len(categoryCounts))
	for categoryID, count := range categoryCounts {
		categoryOptions = append(categoryOptions, categoryOptionResponse{
			DescriptionCategoryID: categoryID,
			ProductsCount:         count,
		})
	}

	response := pricingRulesResponse{
		GlobalDefault:   mapRuleResponsePtr(rules.GlobalDefault),
		CategoryRules:   mapRules(rules.CategoryRules),
		SKUOverrides:    mapRules(rules.SKUOverrides),
		CategoryOptions: categoryOptions,
		ProductCatalog:  productCatalog,
		Meta: pricingRulesMeta{
			TotalRules:            rules.TotalRules,
			ActiveRules:           rules.ActiveRules,
			CategoryRulesCount:    len(rules.CategoryRules),
			SKUOverridesCount:     len(rules.SKUOverrides),
			LastRuleUpdateAt:      timePtrRFC3339(rules.LastRuleUpdate),
			EffectiveRecordsCount: len(effective),
		},
	}
	if len(effective) > 0 && effective[0].ComputedAt.Valid {
		t := effective[0].ComputedAt.Time.UTC()
		response.Meta.LastRecomputeAt = timePtrRFC3339(&t)
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *PricingConstraintsHandler) PutGlobalDefault(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req putGlobalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rule, recompute, err := h.upsertAndRecompute(r.Context(), sellerAccount.ID, req.pricingRulePayload, func(input pricingconstraints.UpsertRuleInput) (pricingconstraints.Rule, error) {
		return h.service.UpsertGlobalDefault(r.Context(), input)
	})
	if err != nil {
		writePricingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, upsertRuleResponse{Rule: mapRuleResponse(rule), Recompute: recompute})
}

func (h *PricingConstraintsHandler) PostCategoryRule(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req upsertCategoryRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DescriptionCategoryID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "description_category_id must be > 0")
		return
	}
	rule, recompute, err := h.upsertAndRecompute(r.Context(), sellerAccount.ID, req.pricingRulePayload, func(input pricingconstraints.UpsertRuleInput) (pricingconstraints.Rule, error) {
		targetID := req.DescriptionCategoryID
		input.ScopeTargetID = &targetID
		input.ScopeTargetCode = req.CategoryCode
		return h.service.UpsertCategoryRule(r.Context(), input)
	})
	if err != nil {
		writePricingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, upsertRuleResponse{Rule: mapRuleResponse(rule), Recompute: recompute})
}

func (h *PricingConstraintsHandler) PostSKUOverride(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req upsertSKUOverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SKU == nil && req.ProductID == nil && (req.OfferID == nil || strings.TrimSpace(*req.OfferID) == "") {
		writeJSONError(w, http.StatusBadRequest, "at least one target is required: sku or product_id or offer_id")
		return
	}
	rule, recompute, err := h.upsertAndRecompute(r.Context(), sellerAccount.ID, req.pricingRulePayload, func(input pricingconstraints.UpsertRuleInput) (pricingconstraints.Rule, error) {
		if req.SKU != nil {
			kind := pricingconstraints.ScopeTargetKindSKU
			input.ScopeTargetKind = &kind
			input.ScopeTargetID = req.SKU
		} else if req.ProductID != nil {
			kind := pricingconstraints.ScopeTargetKindProductID
			input.ScopeTargetKind = &kind
			input.ScopeTargetID = req.ProductID
		} else {
			kind := pricingconstraints.ScopeTargetKindOfferID
			input.ScopeTargetKind = &kind
		}
		input.ScopeTargetCode = req.OfferID
		return h.service.UpsertSKUOverride(r.Context(), input)
	})
	if err != nil {
		writePricingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, upsertRuleResponse{Rule: mapRuleResponse(rule), Recompute: recompute})
}

func (h *PricingConstraintsHandler) PostDeactivateCategoryRule(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req deactivateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RuleID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "rule_id must be > 0")
		return
	}
	rule, err := h.service.DeactivateCategoryRuleByID(r.Context(), sellerAccount.ID, req.RuleID)
	if err != nil {
		writePricingError(w, err)
		return
	}
	recompute, err := h.service.RecomputeEffectiveConstraintsForAccount(r.Context(), sellerAccount.ID)
	if err != nil {
		writePricingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, upsertRuleResponse{
		Rule:      mapRuleResponse(rule),
		Recompute: recompute,
	})
}

func (h *PricingConstraintsHandler) PostDeactivateSKUOverride(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req deactivateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RuleID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "rule_id must be > 0")
		return
	}
	rule, err := h.service.DeactivateSKUOverrideByID(r.Context(), sellerAccount.ID, req.RuleID)
	if err != nil {
		writePricingError(w, err)
		return
	}
	recompute, err := h.service.RecomputeEffectiveConstraintsForAccount(r.Context(), sellerAccount.ID)
	if err != nil {
		writePricingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, upsertRuleResponse{
		Rule:      mapRuleResponse(rule),
		Recompute: recompute,
	})
}

func (h *PricingConstraintsHandler) GetEffectiveConstraints(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if rawSKU := strings.TrimSpace(r.URL.Query().Get("sku")); rawSKU != "" {
		sku, err := strconv.ParseInt(rawSKU, 10, 64)
		if err != nil || sku <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid sku")
			return
		}
		item, err := h.service.GetEffectiveConstraintBySellerAndSKU(r.Context(), sellerAccount.ID, sku)
		if err != nil {
			if err == pgx.ErrNoRows {
				writeJSONError(w, http.StatusNotFound, "effective constraint not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "failed to get effective constraint")
			return
		}
		product, _ := h.service.GetProductBySellerAndProductID(r.Context(), sellerAccount.ID, item.OzonProductID)
		writeJSON(w, http.StatusOK, mapEffective(item, product))
		return
	}

	if rawProduct := strings.TrimSpace(r.URL.Query().Get("product_id")); rawProduct != "" {
		productID, err := strconv.ParseInt(rawProduct, 10, 64)
		if err != nil || productID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid product_id")
			return
		}
		item, err := h.service.GetEffectiveConstraintBySellerAndProduct(r.Context(), sellerAccount.ID, productID)
		if err != nil {
			if err == pgx.ErrNoRows {
				writeJSONError(w, http.StatusNotFound, "effective constraint not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "failed to get effective constraint")
			return
		}
		product, _ := h.service.GetProductBySellerAndProductID(r.Context(), sellerAccount.ID, productID)
		writeJSON(w, http.StatusOK, mapEffective(item, product))
		return
	}

	limit, offset := parsePaginationParams(r)
	page, err := h.service.ListEffectiveConstraintsPage(r.Context(), sellerAccount.ID, limit, offset)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list effective constraints")
		return
	}
	products, err := h.service.ListProductsBySellerAccountID(r.Context(), sellerAccount.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to read products")
		return
	}
	productByID := make(map[int64]dbgen.Product, len(products))
	for _, product := range products {
		productByID[product.OzonProductID] = product
	}
	items := make([]effectiveConstraintResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, mapEffective(item, productByID[item.OzonProductID]))
	}
	writeJSON(w, http.StatusOK, effectiveListResponse{
		Items:  items,
		Total:  page.Total,
		Limit:  page.Limit,
		Offset: page.Offset,
	})
}

func (h *PricingConstraintsHandler) PostPreview(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req previewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	impliedCost, err := h.service.PreviewImpliedCost(req.ReferencePrice, req.ReferenceMarginPercent)
	if err != nil {
		writePricingError(w, err)
		return
	}

	resp := previewResponse{
		ReferencePrice:         req.ReferencePrice,
		ReferenceMarginPercent: req.ReferenceMarginPercent,
		ImpliedCost:            impliedCost,
	}
	if req.MinPrice != nil {
		margin, err := h.service.PreviewExpectedMargin(*req.MinPrice, impliedCost)
		if err != nil {
			writePricingError(w, err)
			return
		}
		resp.ExpectedMarginAtMinPrice = &margin
	}
	if req.MaxPrice != nil {
		margin, err := h.service.PreviewExpectedMargin(*req.MaxPrice, impliedCost)
		if err != nil {
			writePricingError(w, err)
			return
		}
		resp.ExpectedMarginAtMaxPrice = &margin
	}
	if req.InputPrice != nil {
		margin, err := h.service.PreviewExpectedMargin(*req.InputPrice, impliedCost)
		if err != nil {
			writePricingError(w, err)
			return
		}
		resp.ExpectedMarginAtInputPrice = &margin
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *PricingConstraintsHandler) upsertAndRecompute(ctx context.Context, sellerAccountID int64, payload pricingRulePayload, upsert func(input pricingconstraints.UpsertRuleInput) (pricingconstraints.Rule, error)) (pricingconstraints.Rule, pricingconstraints.RecomputeResult, error) {
	isActive := true
	if payload.IsActive != nil {
		isActive = *payload.IsActive
	}
	rule, err := upsert(pricingconstraints.UpsertRuleInput{
		SellerAccountID:        sellerAccountID,
		MinPrice:               payload.MinPrice,
		MaxPrice:               payload.MaxPrice,
		ReferenceMarginPercent: payload.ReferenceMarginPercent,
		ReferencePrice:         payload.ReferencePrice,
		ImpliedCost:            payload.ImpliedCost,
		IsActive:               isActive,
	})
	if err != nil {
		return pricingconstraints.Rule{}, pricingconstraints.RecomputeResult{}, err
	}

	recompute, err := h.service.RecomputeEffectiveConstraintsForAccount(ctx, sellerAccountID)
	if err != nil {
		return pricingconstraints.Rule{}, pricingconstraints.RecomputeResult{}, err
	}
	return rule, recompute, nil
}

func writePricingError(w http.ResponseWriter, err error) {
	if _, ok := err.(*pricingconstraints.ValidationError); ok {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONError(w, http.StatusInternalServerError, err.Error())
}

func mapRules(rules []pricingconstraints.Rule) []pricingRuleResponse {
	out := make([]pricingRuleResponse, 0, len(rules))
	for _, rule := range rules {
		out = append(out, mapRuleResponse(rule))
	}
	return out
}

func mapRuleResponsePtr(rule *pricingconstraints.Rule) *pricingRuleResponse {
	if rule == nil {
		return nil
	}
	mapped := mapRuleResponse(*rule)
	return &mapped
}

func mapRuleResponse(rule pricingconstraints.Rule) pricingRuleResponse {
	return pricingRuleResponse{
		ID:                     rule.ID,
		ScopeType:              string(rule.ScopeType),
		ScopeTargetKind:        scopeTargetKindStringPtr(rule.ScopeTargetKind),
		ScopeTargetID:          rule.ScopeTargetID,
		ScopeTargetCode:        rule.ScopeTargetCode,
		MinPrice:               rule.MinPrice,
		MaxPrice:               rule.MaxPrice,
		ReferenceMarginPercent: rule.ReferenceMarginPercent,
		ReferencePrice:         rule.ReferencePrice,
		ImpliedCost:            rule.ImpliedCost,
		IsActive:               rule.IsActive,
		UpdatedAt:              rule.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func scopeTargetKindStringPtr(v *pricingconstraints.ScopeTargetKind) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}

func mapEffective(row dbgen.SkuEffectiveConstraint, product dbgen.Product) effectiveConstraintResponse {
	var productName *string
	if product.Name != "" {
		value := product.Name
		productName = &value
	}
	return effectiveConstraintResponse{
		OzonProductID:          row.OzonProductID,
		SKU:                    int8PtrValue(row.Sku),
		OfferID:                textPtrValue(row.OfferID),
		ProductName:            productName,
		CurrentPrice:           numericPtrValue(product.ReferencePrice),
		ResolvedFromScopeType:  row.ResolvedFromScopeType,
		RuleID:                 row.RuleID,
		EffectiveMinPrice:      numericPtrValue(row.EffectiveMinPrice),
		EffectiveMaxPrice:      numericPtrValue(row.EffectiveMaxPrice),
		ReferencePrice:         numericPtrValue(row.ReferencePrice),
		ReferenceMarginPercent: numericPtrValue(row.ReferenceMarginPercent),
		ImpliedCost:            numericPtrValue(row.ImpliedCost),
		ComputedAt:             row.ComputedAt.Time.UTC().Format(time.RFC3339),
	}
}

func timePtrRFC3339(v *time.Time) *string {
	if v == nil {
		return nil
	}
	s := v.UTC().Format(time.RFC3339)
	return &s
}

func textPtrValue(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func int8PtrValue(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	i := v.Int64
	return &i
}

func numericPtrValue(v pgtype.Numeric) *float64 {
	if !v.Valid {
		return nil
	}
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	x := f.Float64
	return &x
}
