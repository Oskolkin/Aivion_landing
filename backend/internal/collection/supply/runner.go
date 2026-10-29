package supply

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

const SupplyActPageLimit = 100

type API interface {
	ListSupplyOrders(context.Context, int64, ozon.SupplyOrderListRequest) (*ozon.TypedResponse[ozon.SupplyOrderListResult], error)
	GetSupplyOrder(context.Context, int64, ozon.SupplyOrderRequest) (*ozon.TypedResponse[ozon.SupplyOrder], error)
	GetSupplyOrderDetails(context.Context, int64, ozon.SupplyOrderRequest) (*ozon.TypedResponse[ozon.SupplyOrder], error)
	GetSupplyBundle(context.Context, int64, ozon.SupplyBundleRequest) (*ozon.TypedResponse[ozon.SupplyBundleResult], error)
	GetSupplyActSummary(context.Context, int64, ozon.SupplyActRequest) (*ozon.TypedResponse[ozon.SupplyActSummary], error)
	GetSupplyActProducts(context.Context, int64, ozon.SupplyActRequest) (*ozon.TypedResponse[ozon.SupplyActProductResult], error)
}
type PayloadRegistrar interface {
	StoreAndRegisterPayload(context.Context, *collection.SourcePayloadStore, collection.RegisterPayloadInput) (collection.SourcePayload, error)
}
type SKUResolver interface {
	ResolveSupplySKU(context.Context, int64, *int64, *int64, string, time.Time) (*int64, string, error)
}
type Runner struct {
	API        API
	Collection PayloadRegistrar
	Payloads   *collection.SourcePayloadStore
	SKUs       SKUResolver
	Now        func() time.Time
}

func (r *Runner) Collect(ctx context.Context, seller, run int64) (Result, error) {
	var out Result
	if r == nil || r.API == nil || r.Collection == nil || r.Payloads == nil {
		return out, fmt.Errorf("supply runner is not configured")
	}
	if r.Now == nil {
		r.Now = func() time.Time { return time.Now().UTC() }
	}
	out.StartedAt = r.Now().UTC()
	out.AcceptanceSummaryStatus = ""
	out.AcceptanceProductsStatus = ""
	last := ozon.ProviderID("")
	for page := int64(1); ; page++ {
		response, err := r.API.ListSupplyOrders(ctx, seller, ozon.SupplyOrderListRequest{LastID: last, Limit: 100})
		if err != nil {
			return out, err
		}
		orders := response.Data.Rows()
		payload, err := r.register(ctx, seller, run, ListEndpoint, StateContract, map[string]any{"last_id": last, "limit": 100}, response.Raw, page, response.Meta.StatusCode, len(orders))
		if err != nil {
			return out, err
		}
		out.ListPages++
		for _, listed := range orders {
			if err = r.collectOrder(ctx, seller, run, listed, payload.ID, &out); err != nil {
				out.CompletenessStatus = "PARTIAL"
			}
		}
		if !response.Data.HasNext {
			break
		}
		next := response.Data.LastID
		if next == "" && len(orders) > 0 {
			next = orders[len(orders)-1].OrderID
		}
		if len(orders) == 0 || next == "" || next == last {
			out.CompletenessStatus = "PARTIAL"
			break
		}
		last = next
	}
	out.CompletedAt = r.Now().UTC()
	if out.CompletenessStatus == "" {
		out.CompletenessStatus = "COMPLETE"
	}
	if out.AcceptanceSummaryStatus == "" {
		out.AcceptanceSummaryStatus = "UNAVAILABLE"
	}
	if out.AcceptanceProductsStatus == "" {
		out.AcceptanceProductsStatus = "UNAVAILABLE"
	}
	return out, nil
}

func (r *Runner) collectOrder(ctx context.Context, seller, run int64, listed ozon.SupplyOrder, listPayload int64, out *Result) error {
	if err := requireID(listed.OrderID, "supply order"); err != nil {
		return err
	}
	req := ozon.SupplyOrderRequest{OrderID: listed.OrderID}
	get, err := r.API.GetSupplyOrder(ctx, seller, req)
	if err != nil {
		return err
	}
	getPayload, err := r.register(ctx, seller, run, GetEndpoint, StateContract, map[string]any{"order_id": listed.OrderID}, get.Raw, 1, get.Meta.StatusCode, 1)
	if err != nil {
		return err
	}
	details, err := r.API.GetSupplyOrderDetails(ctx, seller, req)
	if err != nil {
		return err
	}
	detailPayload, err := r.register(ctx, seller, run, DetailsEndpoint, StateContract, map[string]any{"order_id": listed.OrderID}, details.Raw, 1, details.Meta.StatusCode, len(details.Data.Supplies))
	if err != nil {
		return err
	}
	out.DetailRequests += 2
	order := mergeOrder(listed, get.Data, details.Data)
	orderObs := normalizeOrder(order, detailPayload.ID)
	if orderObs.ProviderOrderID == "" {
		orderObs.SourcePayloadID = listPayload
	}
	out.Orders = append(out.Orders, orderObs)
	for _, supplied := range order.Supplies {
		if supplied.OrderID == "" {
			supplied.OrderID = order.OrderID
		}
		if supplied.OrderID != order.OrderID {
			return fmt.Errorf("supply %s belongs to order %s, expected %s", supplied.SupplyID, supplied.OrderID, order.OrderID)
		}
		if err = requireID(supplied.SupplyID, "supply"); err != nil {
			return err
		}
		out.Supplies = append(out.Supplies, normalizeSupply(supplied, getPayload.ID))
		bundle, complete := r.collectBundle(ctx, seller, run, supplied.SupplyID, out)
		out.Bundles = append(out.Bundles, bundle)
		if !complete {
			out.CompletenessStatus = "PARTIAL"
		}
		r.collectAcceptance(ctx, seller, run, supplied.SupplyID, out)
	}
	return nil
}

func (r *Runner) collectBundle(ctx context.Context, seller, run int64, supplyID ozon.ProviderID, out *Result) (BundleObservation, bool) {
	result := BundleObservation{ProviderSupplyID: string(supplyID), CompletenessStatus: "PARTIAL"}
	last := ozon.ProviderID("")
	for page := int64(1); ; page++ {
		response, err := r.API.GetSupplyBundle(ctx, seller, ozon.SupplyBundleRequest{SupplyID: supplyID, LastID: last, Limit: ozon.SupplyBundlePageLimit})
		if err != nil {
			return result, false
		}
		payload, err := r.register(ctx, seller, run, BundleEndpoint, BundleContract, map[string]any{"supply_id": supplyID, "last_id": last, "limit": ozon.SupplyBundlePageLimit}, response.Raw, page, response.Meta.StatusCode, len(response.Data.Items))
		if err != nil {
			return result, false
		}
		out.BundlePages++
		result.PageCount++
		id := payload.ID
		result.SourcePayloadID = &id
		if response.Data.BundleID != "" {
			result.ProviderBundleID = string(response.Data.BundleID)
		}
		for _, item := range response.Data.Items {
			obs := PlannedItem{SourceOfferID: item.OfferID, DeclaredUnits: item.Quantity, SKUResolutionStatus: "UNRESOLVED", ProviderFields: jsonOrEmpty(item.Raw), SourcePayloadID: payload.ID}
			if v := int64(item.SKU); v > 0 {
				obs.SourceSKU = &v
			}
			if v := int64(item.ProductID); v > 0 {
				obs.SourceProductID = &v
			}
			if r.SKUs != nil {
				obs.CanonicalSKUID, obs.SKUResolutionStatus, err = r.SKUs.ResolveSupplySKU(ctx, seller, obs.SourceProductID, obs.SourceSKU, obs.SourceOfferID, out.StartedAt)
				if err != nil {
					return result, false
				}
			}
			result.Items = append(result.Items, obs)
		}
		if !response.Data.HasNext {
			result.CompletenessStatus = "COMPLETE"
			return result, true
		}
		if len(response.Data.Items) == 0 || response.Data.LastID == "" || response.Data.LastID == last {
			return result, false
		}
		last = response.Data.LastID
	}
}

// collectAcceptance is optional beta. Soft failures never fail core Supply.
func (r *Runner) collectAcceptance(ctx context.Context, seller, run int64, supplyID ozon.ProviderID, out *Result) {
	summary, err := r.API.GetSupplyActSummary(ctx, seller, ozon.SupplyActRequest{SupplyID: supplyID})
	if err != nil {
		status := classifyAcceptanceErr(err)
		out.AcceptanceSummaryStatus = worseAcceptanceStatus(out.AcceptanceSummaryStatus, status)
		out.AcceptanceProductsStatus = worseAcceptanceStatus(out.AcceptanceProductsStatus, status)
		return
	}
	payload, err := r.register(ctx, seller, run, ActSummaryEndpoint, AcceptanceContract, map[string]any{"supply_id": supplyID}, summary.Raw, 1, summary.Meta.StatusCode, 1)
	if err != nil {
		out.AcceptanceSummaryStatus = worseAcceptanceStatus(out.AcceptanceSummaryStatus, "PARTIAL")
		out.AcceptanceProductsStatus = worseAcceptanceStatus(out.AcceptanceProductsStatus, "PARTIAL")
		return
	}
	out.AcceptancePages++
	actID := summary.Data.ActID
	if actID == "" || actID == "0" {
		out.AcceptanceSummaryStatus = worseAcceptanceStatus(out.AcceptanceSummaryStatus, "UNAVAILABLE")
		out.AcceptanceProductsStatus = worseAcceptanceStatus(out.AcceptanceProductsStatus, "UNAVAILABLE")
		return
	}
	obs := AcceptanceObservation{
		ProviderSupplyID:   string(supplyID),
		ProviderActID:      string(actID),
		ProviderState:      strings.ToUpper(strings.TrimSpace(summary.Data.State)),
		TotalAmount:        string(summary.Data.TotalAmount),
		CurrencyCode:       strings.ToUpper(strings.TrimSpace(summary.Data.CurrencyCode)),
		CompletenessStatus: "PARTIAL",
		SourcePayloadID:    payload.ID,
	}
	if strings.Contains(obs.ProviderState, "PENDING") {
		out.AcceptanceSummaryStatus = worseAcceptanceStatus(out.AcceptanceSummaryStatus, "PENDING_ACCEPTANCE")
	} else {
		out.AcceptanceSummaryStatus = worseAcceptanceStatus(out.AcceptanceSummaryStatus, "COMPLETE")
	}
	items, productsStatus, pages, complete := r.collectAcceptanceProducts(ctx, seller, run, supplyID, actID, out)
	obs.Items = items
	obs.PageCount = pages
	out.AcceptanceProductsStatus = worseAcceptanceStatus(out.AcceptanceProductsStatus, productsStatus)
	if complete {
		obs.CompletenessStatus = "COMPLETE"
	} else {
		obs.CompletenessStatus = "PARTIAL"
	}
	out.Acceptances = append(out.Acceptances, obs)
}

func (r *Runner) collectAcceptanceProducts(ctx context.Context, seller, run int64, supplyID, actID ozon.ProviderID, out *Result) ([]AcceptanceItem, string, int, bool) {
	var items []AcceptanceItem
	pages := 0
	last := ozon.ProviderID("")
	for page := int64(1); ; page++ {
		response, err := r.API.GetSupplyActProducts(ctx, seller, ozon.SupplyActRequest{SupplyID: supplyID, ActID: actID, LastID: last, Limit: SupplyActPageLimit})
		if err != nil {
			return items, classifyAcceptanceErr(err), pages, false
		}
		payload, err := r.register(ctx, seller, run, ActProductEndpoint, AcceptanceContract, map[string]any{"supply_id": supplyID, "act_id": actID, "last_id": last, "limit": SupplyActPageLimit}, response.Raw, page, response.Meta.StatusCode, len(response.Data.Items))
		if err != nil {
			return items, "PARTIAL", pages, false
		}
		out.AcceptancePages++
		pages++
		for _, item := range response.Data.Items {
			obs := AcceptanceItem{
				SourceOfferID:       item.OfferID,
				AcceptedUnits:       item.Accepted,
				ShortageUnits:       item.Shortage,
				SurplusUnits:        item.Surplus,
				Amount:              string(item.Amount),
				CurrencyCode:        strings.ToUpper(strings.TrimSpace(item.CurrencyCode)),
				SKUResolutionStatus: "UNRESOLVED",
				ProviderFields:      jsonOrEmpty(item.Raw),
				SourcePayloadID:     payload.ID,
			}
			if v := int64(item.SKU); v > 0 {
				obs.SourceSKU = &v
			}
			// Authoritative SKU = ozon_sku only; never fall back to ProductID.
			if r.SKUs != nil && obs.SourceSKU != nil {
				var resolveErr error
				obs.CanonicalSKUID, obs.SKUResolutionStatus, resolveErr = r.SKUs.ResolveSupplySKU(ctx, seller, nil, obs.SourceSKU, "", out.StartedAt)
				if resolveErr != nil {
					return items, "PARTIAL", pages, false
				}
			}
			items = append(items, obs)
		}
		if !response.Data.HasNext {
			return items, "COMPLETE", pages, true
		}
		if len(response.Data.Items) == 0 || response.Data.LastID == "" || response.Data.LastID == last {
			return items, "PARTIAL", pages, false
		}
		last = response.Data.LastID
	}
}

func classifyAcceptanceErr(err error) string {
	var pe *ozon.ProviderError
	if errors.As(err, &pe) {
		switch {
		case pe.StatusCode == http.StatusForbidden, pe.StatusCode == http.StatusUnauthorized:
			return "FORBIDDEN"
		case pe.StatusCode == http.StatusNotFound, pe.StatusCode == http.StatusBadRequest:
			return "UNAVAILABLE"
		case pe.StatusCode == http.StatusTooManyRequests, pe.StatusCode >= 500:
			return "PARTIAL"
		}
	}
	msg := strings.ToUpper(err.Error())
	switch {
	case strings.Contains(msg, "FORBIDDEN"), strings.Contains(msg, "UNAUTHORIZED"):
		return "FORBIDDEN"
	case strings.Contains(msg, "PENDING"):
		return "PENDING_ACCEPTANCE"
	default:
		return "UNAVAILABLE"
	}
}

func worseAcceptanceStatus(current, next string) string {
	rank := map[string]int{
		"": 0, "COMPLETE": 1, "PENDING_ACCEPTANCE": 2, "PARTIAL": 3, "UNAVAILABLE": 4, "FORBIDDEN": 5,
	}
	if rank[next] >= rank[current] {
		return next
	}
	if current == "" {
		return next
	}
	return current
}

func (r *Runner) register(ctx context.Context, seller, run int64, endpoint string, contract collection.SourceContract, identity map[string]any, body []byte, page int64, status, rows int) (collection.SourcePayload, error) {
	httpStatus := int32(status)
	return r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: endpoint, RequestIdentity: identity, Metadata: map[string]any{"contract_key": contract.ContractKey, "contract_version": contract.ContractVersion, "semantic_mapping_hash": contract.SemanticMappingHash, "redaction_profile": RedactionProfile, "returned_rows": rows}, Body: body, ContentType: "application/json", PageSeq: &page, HTTPStatus: &httpStatus})
}
func normalizeOrder(v ozon.SupplyOrder, payload int64) OrderObservation {
	canonical, status := MapState(v.State)
	out := OrderObservation{ProviderOrderID: string(v.OrderID), ProviderState: strings.ToUpper(v.State), CanonicalState: canonical, StateMappingStatus: status, TimeslotFrom: parseTime(v.TimeslotFrom), TimeslotTo: parseTime(v.TimeslotTo), ProviderFields: jsonOrEmpty(v.Raw), SourcePayloadID: payload}
	if id := int64(v.DropoffWarehouseID); id > 0 {
		out.DropoffProviderLocationID = &id
	}
	return out
}
func normalizeSupply(v ozon.Supply, payload int64) SupplyObservation {
	canonical, status := MapState(v.State)
	out := SupplyObservation{ProviderOrderID: string(v.OrderID), ProviderSupplyID: string(v.SupplyID), ProviderState: strings.ToUpper(v.State), CanonicalState: canonical, StateMappingStatus: status, ProviderFields: jsonOrEmpty(v.Raw), SourcePayloadID: payload}
	if id := int64(v.StorageWarehouseID); id > 0 {
		out.StorageProviderLocationID = &id
	}
	return out
}
func mergeOrder(values ...ozon.SupplyOrder) ozon.SupplyOrder {
	var out ozon.SupplyOrder
	for _, v := range values {
		if v.OrderID != "" {
			out.OrderID = v.OrderID
		}
		if v.State != "" {
			out.State = v.State
		}
		if v.DropoffWarehouseID != 0 {
			out.DropoffWarehouseID = v.DropoffWarehouseID
		}
		if v.TimeslotFrom != "" {
			out.TimeslotFrom = v.TimeslotFrom
		}
		if v.TimeslotTo != "" {
			out.TimeslotTo = v.TimeslotTo
		}
		if len(v.Supplies) > 0 {
			out.Supplies = v.Supplies
		}
		if len(v.Raw) > 0 {
			out.Raw = v.Raw
		}
	}
	return out
}
func requireID(v ozon.ProviderID, kind string) error {
	if strings.TrimSpace(string(v)) == "" || v == "0" {
		return fmt.Errorf("%s provider id is required", kind)
	}
	return nil
}
func parseTime(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	v, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	v = v.UTC()
	return &v
}
func jsonOrEmpty(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
