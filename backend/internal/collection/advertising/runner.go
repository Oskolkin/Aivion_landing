package advertising

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/sales"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon/performance"
)

type AdvertisingAPI interface {
	ListPerformanceCampaigns(context.Context, int64, performance.ListCampaignsRequest) (*performance.TypedResponse[performance.ListCampaignsResult], error)
	GetPerformanceDailyStatistics(context.Context, int64, performance.CampaignStatisticsRequest) (*performance.TypedResponse[performance.CampaignStatisticsResult], error)
	GetPerformanceCampaignProducts(context.Context, int64, performance.CampaignProductsRequest) (*performance.TypedResponse[performance.CampaignProductsResult], error)
	GetPerformanceCampaignProductStatistics(context.Context, int64, performance.CampaignStatisticsRequest) (*performance.TypedResponse[performance.CampaignProductStatisticsResult], error)
	GetPerformanceExpenseStatistics(context.Context, int64, performance.CampaignStatisticsRequest) (*performance.TypedResponse[performance.ExpenseStatisticsResult], error)
	SubmitPerformanceCPOOrdersReport(context.Context, int64, time.Time, time.Time) (*performance.TypedResponse[performance.ReportSubmitResult], error)
	GetPerformanceStatisticsReportStatus(context.Context, int64, string) (*performance.TypedResponse[performance.ReportStatus], error)
	DownloadPerformanceStatisticsReport(context.Context, int64, string) (*performance.RawResponse, error)
}
type PayloadRegistrar interface {
	StoreAndRegisterPayload(context.Context, *collection.SourcePayloadStore, collection.RegisterPayloadInput) (collection.SourcePayload, error)
}
type SKUResolver interface {
	ResolveAdvertisingSKU(context.Context, int64, int64, time.Time) (*int64, error)
}
type OrderResolver interface {
	ResolveAdvertisingOrder(context.Context, int64, int64, time.Time) (*int64, error)
}
type ReportRequestStore interface {
	LookupReportRequest(ctx context.Context, seller, run int64, fingerprint string) (ReportRequestObservation, bool, error)
	UpsertReportRequest(ctx context.Context, seller, run int64, rr ReportRequestObservation) error
}
type Runner struct {
	API        AdvertisingAPI
	Collection PayloadRegistrar
	Payloads   *collection.SourcePayloadStore
	Recovery   collection.ReplayRepository
	Reports    ReportRequestStore
	SKUs       SKUResolver
	Orders     OrderResolver
	Now        func() time.Time
	Sleep      func(time.Duration)
}

func (r *Runner) Collect(ctx context.Context, seller, run int64, from, to time.Time) (Result, error) {
	var out Result
	if r == nil || r.API == nil || r.Collection == nil || r.Payloads == nil {
		return out, fmt.Errorf("advertising runner is not configured")
	}
	if r.Now == nil {
		r.Now = func() time.Time { return time.Now().UTC() }
	}
	if r.Sleep == nil {
		r.Sleep = time.Sleep
	}
	out.StartedAt = r.Now().UTC()
	out.RequestedFrom = date(from)
	out.RequestedTo = date(to)
	campaignReplay, e := collection.LoadReplayArtifacts(ctx, r.Recovery, r.Payloads, seller, run, CampaignEndpoint, RedactionProfile)
	if e != nil {
		return out, e
	}
	dailyReplay, e := collection.LoadReplayArtifacts(ctx, r.Recovery, r.Payloads, seller, run, DailyEndpoint, RedactionProfile)
	if e != nil {
		return out, e
	}
	productReplay, e := collection.LoadReplayArtifacts(ctx, r.Recovery, r.Payloads, seller, run, ProductDailyEndpoint, RedactionProfile)
	if e != nil {
		return out, e
	}
	expenseReplay, e := collection.LoadReplayArtifacts(ctx, r.Recovery, r.Payloads, seller, run, ExpenseEndpoint, RedactionProfile)
	if e != nil {
		return out, e
	}
	targetReplay, e := collection.LoadReplayArtifacts(ctx, r.Recovery, r.Payloads, seller, run, TargetEndpoint, RedactionProfile)
	if e != nil {
		return out, e
	}
	dailyReplayIndex, productReplayIndex, expenseReplayIndex, targetReplayIndex := 0, 0, 0, 0
	campaignByID := map[int64]CampaignObservation{}
	for page := int64(1); ; page++ {
		identity := map[string]any{"page": page, "page_size": CampaignPageLimit}
		var resp *performance.TypedResponse[performance.ListCampaignsResult]
		var payload collection.SourcePayload
		if int(page) <= len(campaignReplay) {
			artifact := campaignReplay[page-1]
			if e = validateAdvertisingReplay(artifact, CampaignEndpoint, CampaignContract, identity, page); e != nil {
				return out, e
			}
			result, parseErr := performance.ParseCampaignsResponse(artifact.Body)
			if parseErr != nil {
				return out, parseErr
			}
			resp, payload = &performance.TypedResponse[performance.ListCampaignsResult]{Data: result, Raw: artifact.Body}, artifact.Payload
		} else {
			resp, e = r.API.ListPerformanceCampaigns(ctx, seller, performance.ListCampaignsRequest{Page: page, PageSize: CampaignPageLimit})
			if e != nil {
				return out, e
			}
			seq := page
			status := int32(resp.Meta.StatusCode)
			metadata := meta(CampaignContract)
			metadata["returned_rows"] = len(resp.Data.Items)
			payload, e = r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: CampaignEndpoint, RequestIdentity: identity, Metadata: metadata, Body: resp.Raw, ContentType: "application/json", PageSeq: &seq, HTTPStatus: &status})
			if e != nil {
				return out, e
			}
		}
		out.CampaignPages++
		for _, c := range resp.Data.Items {
			obs, e := normalizeCampaign(c, payload.ID)
			if e != nil {
				return out, e
			}
			campaignByID[obs.ProviderCampaignID] = obs
		}
		if len(resp.Data.Items) < CampaignPageLimit {
			if int(page) < len(campaignReplay) {
				return out, fmt.Errorf("campaign replay contains artifacts after terminal page")
			}
			break
		}
	}
	ids := make([]int64, 0, len(campaignByID))
	for id, c := range campaignByID {
		ids = append(ids, id)
		out.Campaigns = append(out.Campaigns, c)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	sort.Slice(out.Campaigns, func(i, j int) bool { return out.Campaigns[i].ProviderCampaignID < out.Campaigns[j].ProviderCampaignID })
	for start := 0; start < len(ids); start += CampaignReportBatchLimit {
		end := start + CampaignReportBatchLimit
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]
		identity := map[string]any{"campaign_ids": batch, "date_from": out.RequestedFrom.Format("2006-01-02"), "date_to": out.RequestedTo.Format("2006-01-02")}
		var resp *performance.TypedResponse[performance.CampaignStatisticsResult]
		var payload collection.SourcePayload
		seq := int64(100000 + out.DailyBatches + 1)
		if dailyReplayIndex < len(dailyReplay) {
			artifact := dailyReplay[dailyReplayIndex]
			if e = validateAdvertisingReplay(artifact, DailyEndpoint, DailyContract, identity, seq); e != nil {
				return out, e
			}
			result, parseErr := performance.ParseDailyStatisticsResponse(artifact.Body)
			if parseErr != nil {
				return out, parseErr
			}
			resp, payload = &performance.TypedResponse[performance.CampaignStatisticsResult]{Data: result, Raw: artifact.Body}, artifact.Payload
			dailyReplayIndex++
		} else {
			resp, e = r.API.GetPerformanceDailyStatistics(ctx, seller, performance.CampaignStatisticsRequest{CampaignIDs: batch, DateFrom: out.RequestedFrom.Format("2006-01-02"), DateTo: out.RequestedTo.Format("2006-01-02")})
			if e != nil {
				return out, e
			}
			status := int32(resp.Meta.StatusCode)
			metadata := meta(DailyContract)
			metadata["returned_rows"] = len(resp.Data.Items)
			payload, e = r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: DailyEndpoint, RequestIdentity: identity, Metadata: metadata, Body: resp.Raw, ContentType: "application/json", PageSeq: &seq, HTTPStatus: &status})
			if e != nil {
				return out, e
			}
		}
		out.DailyBatches++
		for _, d := range resp.Data.Items {
			obs, e := normalizeDaily(d, payload.ID)
			if e != nil {
				return out, e
			}
			out.Daily = append(out.Daily, obs)
		}
	}
	if dailyReplayIndex < len(dailyReplay) {
		return out, fmt.Errorf("daily replay contains artifacts outside campaign universe")
	}

	productFamilyIDs := make([]int64, 0, len(out.Campaigns))
	for _, c := range out.Campaigns {
		if c.CampaignFamily == "PRODUCT_CPC" || c.CampaignFamily == "SEARCH_PROMO_CPO" {
			productFamilyIDs = append(productFamilyIDs, c.ProviderCampaignID)
		}
	}
	sort.Slice(productFamilyIDs, func(i, j int) bool { return productFamilyIDs[i] < productFamilyIDs[j] })
	for start := 0; start < len(productFamilyIDs); start += CampaignReportBatchLimit {
		end := start + CampaignReportBatchLimit
		if end > len(productFamilyIDs) {
			end = len(productFamilyIDs)
		}
		batch := productFamilyIDs[start:end]
		identity := map[string]any{"campaign_ids": batch, "date_from": out.RequestedFrom.Format("2006-01-02"), "date_to": out.RequestedTo.Format("2006-01-02")}
		seq := int64(300000 + out.ProductBatches + 1)
		var resp *performance.TypedResponse[performance.CampaignProductStatisticsResult]
		var payload collection.SourcePayload
		if productReplayIndex < len(productReplay) {
			artifact := productReplay[productReplayIndex]
			if e = validateAdvertisingReplay(artifact, ProductDailyEndpoint, ProductDailyContract, identity, seq); e != nil {
				return out, e
			}
			result, parseErr := performance.ParseCampaignProductStatisticsResponse(artifact.Body)
			if parseErr != nil {
				return out, parseErr
			}
			resp, payload = &performance.TypedResponse[performance.CampaignProductStatisticsResult]{Data: result, Raw: artifact.Body}, artifact.Payload
			productReplayIndex++
		} else {
			resp, e = r.API.GetPerformanceCampaignProductStatistics(ctx, seller, performance.CampaignStatisticsRequest{CampaignIDs: batch, DateFrom: out.RequestedFrom.Format("2006-01-02"), DateTo: out.RequestedTo.Format("2006-01-02")})
			if e != nil {
				out.CompletenessStatus = "PARTIAL"
				continue
			}
			status := int32(resp.Meta.StatusCode)
			metadata := meta(ProductDailyContract)
			metadata["returned_rows"] = len(resp.Data.Items)
			payload, e = r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: ProductDailyEndpoint, RequestIdentity: identity, Metadata: metadata, Body: resp.Raw, ContentType: "application/json", PageSeq: &seq, HTTPStatus: &status})
			if e != nil {
				return out, e
			}
		}
		out.ProductBatches++
		for _, d := range resp.Data.Items {
			obs, e := normalizeProductDaily(ctx, r, seller, d, payload.ID, out.StartedAt)
			if e != nil {
				return out, e
			}
			out.ProductDaily = append(out.ProductDaily, obs)
		}
	}
	if productReplayIndex < len(productReplay) {
		return out, fmt.Errorf("product daily replay contains artifacts outside campaign universe")
	}

	expenseIdentity := map[string]any{"date_from": out.RequestedFrom.Format("2006-01-02"), "date_to": out.RequestedTo.Format("2006-01-02")}
	expenseSeq := int64(400001)
	if expenseReplayIndex < len(expenseReplay) {
		artifact := expenseReplay[expenseReplayIndex]
		if e = validateAdvertisingReplay(artifact, ExpenseEndpoint, ExpenseContract, expenseIdentity, expenseSeq); e != nil {
			return out, e
		}
		result, parseErr := performance.ParseExpenseStatisticsResponse(artifact.Body)
		if parseErr != nil {
			return out, parseErr
		}
		out.ExpenseBatches++
		expenseReplayIndex++
		for _, row := range result.Items {
			obs, e := normalizeExpense(row, artifact.Payload.ID)
			if e != nil {
				return out, e
			}
			out.Expense = append(out.Expense, obs)
		}
	} else {
		resp, err := r.API.GetPerformanceExpenseStatistics(ctx, seller, performance.CampaignStatisticsRequest{DateFrom: out.RequestedFrom.Format("2006-01-02"), DateTo: out.RequestedTo.Format("2006-01-02")})
		if err != nil {
			out.CompletenessStatus = "PARTIAL"
		} else {
			status := int32(resp.Meta.StatusCode)
			metadata := meta(ExpenseContract)
			metadata["returned_rows"] = len(resp.Data.Items)
			payload, e := r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: ExpenseEndpoint, RequestIdentity: expenseIdentity, Metadata: metadata, Body: resp.Raw, ContentType: "application/json", PageSeq: &expenseSeq, HTTPStatus: &status})
			if e != nil {
				return out, e
			}
			out.ExpenseBatches++
			for _, row := range resp.Data.Items {
				obs, e := normalizeExpense(row, payload.ID)
				if e != nil {
					return out, e
				}
				out.Expense = append(out.Expense, obs)
			}
		}
	}
	if expenseReplayIndex < len(expenseReplay) {
		return out, fmt.Errorf("expense replay contains unused artifacts")
	}

	for i := range out.Campaigns {
		c := &out.Campaigns[i]
		if c.CampaignFamily != "PRODUCT_CPC" && c.CampaignFamily != "SEARCH_PROMO_CPO" {
			continue
		}
		enumerationOK := true
		for page := int64(1); ; page++ {
			identity := map[string]any{"campaign_id": c.ProviderCampaignID, "page": page, "page_size": TargetPageLimit}
			seq := int64(200000 + out.TargetPages + 1)
			var resp *performance.TypedResponse[performance.CampaignProductsResult]
			var payload collection.SourcePayload
			if targetReplayIndex < len(targetReplay) {
				artifact := targetReplay[targetReplayIndex]
				if e = validateAdvertisingReplay(artifact, TargetEndpoint, TargetContract, identity, seq); e != nil {
					return out, e
				}
				result, parseErr := performance.ParseCampaignProductsResponse(c.ProviderCampaignID, artifact.Body)
				if parseErr != nil {
					return out, parseErr
				}
				resp, payload = &performance.TypedResponse[performance.CampaignProductsResult]{Data: result, Raw: artifact.Body}, artifact.Payload
				targetReplayIndex++
			} else {
				resp, e = r.API.GetPerformanceCampaignProducts(ctx, seller, performance.CampaignProductsRequest{CampaignID: c.ProviderCampaignID, Page: page, PageSize: TargetPageLimit})
				if e != nil {
					out.CompletenessStatus = "PARTIAL"
					enumerationOK = false
					break
				}
				status := int32(resp.Meta.StatusCode)
				metadata := meta(TargetContract)
				metadata["returned_rows"] = len(resp.Data.Items)
				payload, e = r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: TargetEndpoint, RequestIdentity: identity, Metadata: metadata, Body: resp.Raw, ContentType: "application/json", PageSeq: &seq, HTTPStatus: &status})
				if e != nil {
					return out, e
				}
			}
			out.TargetPages++
			for _, p := range resp.Data.Items {
				sku := p.SKU
				resolution := "UNRESOLVED"
				var canonical *int64
				if r.SKUs != nil {
					canonical, e = r.SKUs.ResolveAdvertisingSKU(ctx, seller, sku, out.StartedAt)
					if e != nil {
						return out, e
					}
					if canonical != nil {
						resolution = "RESOLVED"
					}
				}
				bid, e := decimal(p.BidAmount)
				if e != nil {
					return out, e
				}
				out.Targets = append(out.Targets, TargetObservation{ProviderCampaignID: c.ProviderCampaignID, CanonicalSKUID: canonical, SourceSKU: &sku, SKUResolutionStatus: resolution, AssignmentState: p.Status, Bid: bid, BidUnit: p.BidUnitAssumption, SourcePayloadID: payload.ID})
			}
			if len(resp.Data.Items) < TargetPageLimit {
				break
			}
		}
		if enumerationOK {
			out.AuthoritativeTargetCampaignIDs = append(out.AuthoritativeTargetCampaignIDs, c.ProviderCampaignID)
			c.TargetScopeType = "EXPLICIT_SKU_SET"
			if existing, ok := campaignByID[c.ProviderCampaignID]; ok {
				existing.TargetScopeType = "EXPLICIT_SKU_SET"
				campaignByID[c.ProviderCampaignID] = existing
			}
		}
	}
	if targetReplayIndex < len(targetReplay) {
		return out, fmt.Errorf("target replay contains artifacts outside campaign universe")
	}

	if e = r.collectCPO(ctx, seller, run, &out); e != nil {
		out.CompletenessStatus = "PARTIAL"
	}

	out.CompletedAt = r.Now().UTC()
	if out.CompletenessStatus == "" {
		out.CompletenessStatus = "COMPLETE"
	}
	return out, nil
}

func (r *Runner) collectCPO(ctx context.Context, seller, run int64, out *Result) error {
	identity := map[string]any{"date_from": out.RequestedFrom.Format(time.RFC3339), "date_to": out.RequestedTo.Add(24*time.Hour - time.Nanosecond).Format(time.RFC3339)}
	fingerprint, err := collection.RequestFingerprint(collection.ProviderOzon, CPOReportEndpoint, map[string]any{"collection_domain": string(collection.DomainAdvertising), "date_from": identity["date_from"], "date_to": identity["date_to"]})
	if err != nil {
		return err
	}
	report := ReportRequestObservation{SourceContractKey: CPOContract.ContractKey, RequestFingerprint: fingerprint, StatusObservedAt: r.Now().UTC()}
	var reportUUID string
	resumeExisting := false
	if r.Reports != nil {
		existing, found, lookupErr := r.Reports.LookupReportRequest(ctx, seller, run, fingerprint)
		if lookupErr != nil {
			return lookupErr
		}
		if found {
			switch existing.Status {
			case "SUBMITTED", "RUNNING":
				if existing.ProviderReportID == "" {
					return fmt.Errorf("cpo report resume missing uuid")
				}
				report = existing
				reportUUID = existing.ProviderReportID
				resumeExisting = true
			case "READY":
				report = existing
				if existing.ResultSourcePayloadID != nil && r.Payloads != nil && r.Recovery != nil {
					body, payload, loadErr := r.loadRegisteredPayloadBody(ctx, seller, run, *existing.ResultSourcePayloadID)
					if loadErr != nil {
						return loadErr
					}
					out.CPOReports++
					out.ReportRequests = append(out.ReportRequests, report)
					return r.appendCPORows(ctx, seller, out, body, payload.ID)
				}
				if existing.ProviderReportID == "" {
					return fmt.Errorf("cpo ready report missing uuid and payload")
				}
				reportUUID = existing.ProviderReportID
				resumeExisting = true
			case "FAILED":
				if existing.ProviderReportID != "" {
					out.ReportRequests = append(out.ReportRequests, existing)
					return fmt.Errorf("cpo report previously failed")
				}
			}
		}
	}
	if !resumeExisting {
		submit, submitErr := r.API.SubmitPerformanceCPOOrdersReport(ctx, seller, out.RequestedFrom, out.RequestedTo.Add(24*time.Hour-time.Nanosecond))
		if submitErr != nil {
			return submitErr
		}
		out.CPOReports++
		status := int32(submit.Meta.StatusCode)
		seq := int64(500001)
		_, err = r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: CPOReportEndpoint, RequestIdentity: identity, Metadata: meta(CPOContract), Body: submit.Raw, ContentType: "application/json", PageSeq: &seq, HTTPStatus: &status})
		if err != nil {
			return err
		}
		reportUUID = submit.Data.UUID
		report.ProviderReportID = reportUUID
		report.Status = "SUBMITTED"
		report.StatusObservedAt = r.Now().UTC()
		if reportUUID == "" {
			report.Status = "FAILED"
			_ = r.persistReportRequest(ctx, seller, run, report)
			out.ReportRequests = append(out.ReportRequests, report)
			return fmt.Errorf("cpo report uuid missing")
		}
		if err = r.persistReportRequest(ctx, seller, run, report); err != nil {
			return err
		}
	} else {
		out.CPOReports++
	}
	var ready *performance.TypedResponse[performance.ReportStatus]
	for attempt := 0; attempt < ReportPollAttempts; attempt++ {
		if attempt > 0 {
			r.Sleep(time.Duration(ReportPollIntervalMs) * time.Millisecond)
		}
		st, pollErr := r.API.GetPerformanceStatisticsReportStatus(ctx, seller, reportUUID)
		if pollErr != nil {
			report.Status = "FAILED"
			report.StatusObservedAt = r.Now().UTC()
			_ = r.persistReportRequest(ctx, seller, run, report)
			out.ReportRequests = append(out.ReportRequests, report)
			return pollErr
		}
		report.StatusObservedAt = r.Now().UTC()
		state := strings.ToUpper(st.Data.State)
		switch state {
		case "OK", "READY", "SUCCESS", "DONE":
			report.Status = "READY"
			ready = st
			_ = r.persistReportRequest(ctx, seller, run, report)
		case "FAILED", "ERROR", "CANCELLED":
			report.Status = "FAILED"
			_ = r.persistReportRequest(ctx, seller, run, report)
			out.ReportRequests = append(out.ReportRequests, report)
			return fmt.Errorf("cpo report failed: %s", state)
		default:
			report.Status = "RUNNING"
			_ = r.persistReportRequest(ctx, seller, run, report)
			continue
		}
		break
	}
	if ready == nil {
		report.Status = "FAILED"
		report.StatusObservedAt = r.Now().UTC()
		_ = r.persistReportRequest(ctx, seller, run, report)
		out.ReportRequests = append(out.ReportRequests, report)
		return fmt.Errorf("cpo report poll exhausted")
	}
	download, err := r.API.DownloadPerformanceStatisticsReport(ctx, seller, reportUUID)
	if err != nil {
		report.Status = "FAILED"
		report.StatusObservedAt = r.Now().UTC()
		_ = r.persistReportRequest(ctx, seller, run, report)
		out.ReportRequests = append(out.ReportRequests, report)
		return err
	}
	dlStatus := int32(download.Meta.StatusCode)
	dlSeq := int64(500002)
	resultPayload, err := r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: CPOReportEndpoint + ".result", RequestIdentity: map[string]any{"report_uuid": reportUUID}, Metadata: meta(CPOContract), Body: download.Body, ContentType: "application/json", PageSeq: &dlSeq, HTTPStatus: &dlStatus})
	if err != nil {
		return err
	}
	id := resultPayload.ID
	report.ResultSourcePayloadID = &id
	report.Status = "READY"
	report.StatusObservedAt = r.Now().UTC()
	_ = r.persistReportRequest(ctx, seller, run, report)
	out.ReportRequests = append(out.ReportRequests, report)
	return r.appendCPORows(ctx, seller, out, download.Body, resultPayload.ID)
}

func (r *Runner) persistReportRequest(ctx context.Context, seller, run int64, rr ReportRequestObservation) error {
	if r.Reports == nil {
		return nil
	}
	return r.Reports.UpsertReportRequest(ctx, seller, run, rr)
}

func (r *Runner) loadRegisteredPayloadBody(ctx context.Context, seller, run, payloadID int64) ([]byte, collection.SourcePayload, error) {
	rows, err := r.Recovery.ListPayloads(ctx, seller, run)
	if err != nil {
		return nil, collection.SourcePayload{}, err
	}
	for _, p := range rows {
		if p.ID != payloadID {
			continue
		}
		body, getErr := r.Payloads.Get(ctx, p.Bucket, p.ObjectKey)
		if getErr != nil {
			return nil, collection.SourcePayload{}, getErr
		}
		return body, p, nil
	}
	return nil, collection.SourcePayload{}, fmt.Errorf("cpo result payload %d not found", payloadID)
}

func (r *Runner) appendCPORows(ctx context.Context, seller int64, out *Result, body []byte, payloadID int64) error {
	rows, err := parseCPORows(body)
	if err != nil {
		return err
	}
	for _, row := range rows {
		obs, e := normalizeCPO(ctx, r, seller, row, payloadID, out.StartedAt)
		if e != nil {
			return e
		}
		out.CPO = append(out.CPO, obs)
	}
	return nil
}

type cpoRow struct {
	CampaignID, PromotedSKU, OrderedSKU, OrderID int64
	BusinessDate                                 time.Time
	Units                                        *int64
	Amount                                       string
}

func parseCPORows(body []byte) ([]cpoRow, error) {
	maps, err := performance.ParseCPOOrdersReport(body)
	if err != nil {
		return nil, err
	}
	return mapCPOMaps(maps), nil
}

func mapCPOMaps(rows []map[string]any) []cpoRow {
	out := make([]cpoRow, 0, len(rows))
	for _, decoded := range rows {
		campaign, ok := intFromAny(decoded["campaignId"], decoded["campaign_id"], decoded["id"])
		if !ok {
			continue
		}
		promoted, _ := intFromAny(decoded["sku"], decoded["promotedSku"], decoded["promoted_sku"], decoded["offerSku"])
		ordered, _ := intFromAny(decoded["orderedSku"], decoded["ordered_sku"], decoded["orderSku"], decoded["sku"])
		orderID, _ := intFromAny(decoded["orderId"], decoded["order_id"], decoded["ozonOrderId"])
		day := parseFlexibleDate(stringFromAny(decoded["date"], decoded["orderDate"], decoded["businessDate"]))
		if campaign == 0 || day.IsZero() {
			continue
		}
		var units *int64
		if u, uOK := intFromAny(decoded["quantity"], decoded["orders"], decoded["units"]); uOK || decoded["quantity"] != nil || decoded["orders"] != nil || decoded["units"] != nil {
			if decoded["quantity"] != nil || decoded["orders"] != nil || decoded["units"] != nil {
				v := u
				units = &v
			}
		}
		out = append(out, cpoRow{CampaignID: campaign, PromotedSKU: promoted, OrderedSKU: ordered, OrderID: orderID, BusinessDate: day, Units: units, Amount: stringFromAny(decoded["sales"], decoded["amount"], decoded["ordersMoney"], decoded["revenue"])})
	}
	return out
}

func intFromAny(vals ...any) (int64, bool) {
	for _, v := range vals {
		if v == nil {
			continue
		}
		switch x := v.(type) {
		case float64:
			continue // refuse float64 IDs/money paths
		case int64:
			return x, true
		case int:
			return int64(x), true
		case json.Number:
			i, err := x.Int64()
			if err != nil {
				continue
			}
			return i, true
		case string:
			var n int64
			if _, err := fmt.Sscan(strings.TrimSpace(x), &n); err != nil {
				continue
			}
			return n, true
		}
	}
	return 0, false
}
func stringFromAny(vals ...any) string {
	for _, v := range vals {
		if v == nil {
			continue
		}
		switch x := v.(type) {
		case string:
			return strings.TrimSpace(x)
		case json.Number:
			return x.String()
		}
	}
	return ""
}
func parseFlexibleDate(v string) time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, v); err == nil {
			return date(t)
		}
	}
	return time.Time{}
}

func validateAdvertisingReplay(a collection.ReplayArtifact, endpoint string, contract collection.SourceContract, identity map[string]any, sequence int64) error {
	if a.Payload.PageSeq == nil || *a.Payload.PageSeq != sequence {
		return fmt.Errorf("advertising replay sequence mismatch for %s", endpoint)
	}
	request := map[string]any{"collection_domain": string(collection.DomainAdvertising)}
	for k, v := range identity {
		request[k] = v
	}
	fingerprint, err := collection.RequestFingerprint(collection.ProviderOzon, endpoint, request)
	if err != nil || fingerprint != a.Payload.RequestFingerprint {
		return fmt.Errorf("advertising replay request fingerprint mismatch for %s", endpoint)
	}
	var metadata map[string]any
	if json.Unmarshal(a.Payload.Metadata, &metadata) != nil || metadata["semantic_mapping_hash"] != contract.SemanticMappingHash {
		return fmt.Errorf("advertising replay source contract mismatch for %s", endpoint)
	}
	return nil
}

func normalizeCampaign(c performance.Campaign, payload int64) (CampaignObservation, error) {
	currency, e := sales.ResolveCurrency("OZON", "RU")
	if e != nil {
		return CampaignObservation{}, e
	}
	budget, e := decimal(c.BudgetAmount)
	if e != nil {
		return CampaignObservation{}, e
	}
	weekly, e := decimal(c.BudgetWeekly)
	if e != nil {
		return CampaignObservation{}, e
	}
	family, scope := campaignSemantics(c)
	raw := c.Raw
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var isAutocreated *bool
	if c.IsAutocreatedPresent {
		value := c.IsAutocreated
		isAutocreated = &value
	}
	return CampaignObservation{ProviderCampaignID: c.CampaignExternalID, Title: c.CampaignName, ProviderState: c.Status, PaymentType: c.PaymentType, AdvObjectType: c.CampaignType, CampaignFamily: family, PlacementJSON: c.PlacementJSON, Budget: budget, WeeklyBudget: weekly, CurrencyCode: currency, FromDate: datePtr(c.FromDate), ToDate: datePtr(c.ToDate), Strategy: c.ProductCampaignMode, ProductAutopilotStrategy: c.ProductAutopilotStrategy, IsAutocreated: isAutocreated, AutostopStatus: c.AutostopStatus, TargetScopeType: scope, ProviderFields: raw, SourcePayloadID: payload}, nil
}
func campaignSemantics(c performance.Campaign) (string, string) {
	kind := strings.ToUpper(c.CampaignType)
	payment := strings.ToUpper(c.PaymentType)
	switch {
	case strings.Contains(kind, "MEDIA"):
		return "MEDIA_CPM", "NON_SKU_OBJECTS"
	case strings.Contains(kind, "SEARCH") || strings.Contains(payment, "CPO"):
		return "SEARCH_PROMO_CPO", "UNKNOWN"
	case strings.Contains(kind, "SKU") || strings.Contains(payment, "CPC"):
		return "PRODUCT_CPC", "UNKNOWN"
	default:
		return "UNKNOWN", "UNKNOWN"
	}
}
func normalizeDaily(d performance.CampaignDailyMetric, payload int64) (DailyObservation, error) {
	currency, e := sales.ResolveCurrency("OZON", "RU")
	if e != nil {
		return DailyObservation{}, e
	}
	spend, e := decimal(d.Spend)
	if e != nil {
		return DailyObservation{}, e
	}
	bonus, e := decimal(d.BonusSpend)
	if e != nil {
		return DailyObservation{}, e
	}
	amount, e := decimal(d.Revenue)
	if e != nil {
		return DailyObservation{}, e
	}
	drr, e := decimal(d.ProviderDRR)
	if e != nil {
		return DailyObservation{}, e
	}
	var impressions, clicks, orders *int64
	if d.ImpressionsPresent {
		v := d.Impressions
		impressions = &v
	}
	if d.ClicksPresent {
		v := d.Clicks
		clicks = &v
	}
	if d.OrdersCountPresent {
		v := d.OrdersCount
		orders = &v
	}
	trafficState := "USABLE"
	if impressions == nil || clicks == nil {
		trafficState = "PARTIAL"
	}
	return DailyObservation{ProviderCampaignID: d.CampaignExternalID, BusinessDate: date(d.MetricDate), Impressions: impressions, Clicks: clicks, PaidSpendAmount: spend, BonusSpendAmount: bonus, AttributedOrderAmount: amount, AttributedOrderUnits: orders, ProviderDRR: drr, CurrencyCode: currency, TrafficState: trafficState, AttributionState: "PENDING_ACCEPTANCE", AttributionKind: "PROVIDER_DEFAULT", SourcePayloadID: payload}, nil
}
func normalizeProductDaily(ctx context.Context, r *Runner, seller int64, d performance.CampaignProductDailyMetric, payload int64, cutoff time.Time) (ProductDailyObservation, error) {
	currency, e := sales.ResolveCurrency("OZON", "RU")
	if e != nil {
		return ProductDailyObservation{}, e
	}
	spend, e := decimal(d.Spend)
	if e != nil {
		return ProductDailyObservation{}, e
	}
	bonus, e := decimal(d.BonusSpend)
	if e != nil {
		return ProductDailyObservation{}, e
	}
	amount, e := decimal(d.Revenue)
	if e != nil {
		return ProductDailyObservation{}, e
	}
	drr, e := decimal(d.ProviderDRR)
	if e != nil {
		return ProductDailyObservation{}, e
	}
	var impressions, clicks, orders *int64
	if d.ImpressionsPresent {
		v := d.Impressions
		impressions = &v
	}
	if d.ClicksPresent {
		v := d.Clicks
		clicks = &v
	}
	if d.OrdersCountPresent {
		v := d.OrdersCount
		orders = &v
	}
	resolution := "UNRESOLVED"
	var canonical *int64
	if r.SKUs != nil {
		canonical, e = r.SKUs.ResolveAdvertisingSKU(ctx, seller, d.SKU, cutoff)
		if e != nil {
			return ProductDailyObservation{}, e
		}
		if canonical != nil {
			resolution = "RESOLVED"
		}
	}
	kind := d.AttributionKind
	if kind == "" {
		kind = "PROVIDER_DEFAULT"
	}
	return ProductDailyObservation{ProviderCampaignID: d.CampaignExternalID, BusinessDate: date(d.MetricDate), SourcePromotedSKU: d.SKU, CanonicalSKUID: canonical, SKUResolutionStatus: resolution, Impressions: impressions, Clicks: clicks, PaidSpendAmount: spend, BonusSpendAmount: bonus, AttributedOrderUnits: orders, AttributedOrderAmount: amount, ProviderDRR: drr, CurrencyCode: currency, AttributionKind: kind, SourcePayloadID: payload}, nil
}
func normalizeExpense(d performance.ExpenseDailyMetric, payload int64) (ExpenseObservation, error) {
	currency, e := sales.ResolveCurrency("OZON", "RU")
	if e != nil {
		return ExpenseObservation{}, e
	}
	if d.CurrencyHint != "" {
		currency = strings.ToUpper(strings.TrimSpace(d.CurrencyHint))
	}
	paid, e := decimal(d.PaidSpend)
	if e != nil {
		return ExpenseObservation{}, e
	}
	bonus, e := decimal(d.BonusSpend)
	if e != nil {
		return ExpenseObservation{}, e
	}
	return ExpenseObservation{BusinessDate: date(d.MetricDate), PaidSpendAmount: paid, BonusSpendAmount: bonus, CurrencyCode: currency, SourcePayloadID: payload}, nil
}
func normalizeCPO(ctx context.Context, r *Runner, seller int64, row cpoRow, payload int64, cutoff time.Time) (CPOObservation, error) {
	currency, e := sales.ResolveCurrency("OZON", "RU")
	if e != nil {
		return CPOObservation{}, e
	}
	amount, e := decimal(row.Amount)
	if e != nil {
		return CPOObservation{}, e
	}
	obs := CPOObservation{ProviderCampaignID: row.CampaignID, BusinessDate: date(row.BusinessDate), AttributedOrderUnits: row.Units, AttributedOrderAmount: amount, CurrencyCode: currency, AttributionKind: "PROVIDER_DEFAULT", SourcePayloadID: payload, PromotedSKUResolutionStatus: "UNRESOLVED", OrderedSKUResolutionStatus: "UNRESOLVED", OrderResolutionStatus: "UNRESOLVED"}
	if row.PromotedSKU > 0 {
		sku := row.PromotedSKU
		obs.SourcePromotedSKU = &sku
		if r.SKUs != nil {
			if id, e := r.SKUs.ResolveAdvertisingSKU(ctx, seller, sku, cutoff); e != nil {
				return CPOObservation{}, e
			} else if id != nil {
				obs.PromotedCanonicalSKUID = id
				obs.PromotedSKUResolutionStatus = "RESOLVED"
			}
		}
	}
	if row.OrderedSKU > 0 {
		sku := row.OrderedSKU
		obs.SourceOrderedSKU = &sku
		if r.SKUs != nil {
			if id, e := r.SKUs.ResolveAdvertisingSKU(ctx, seller, sku, cutoff); e != nil {
				return CPOObservation{}, e
			} else if id != nil {
				obs.OrderedCanonicalSKUID = id
				obs.OrderedSKUResolutionStatus = "RESOLVED"
			}
		}
	}
	if row.OrderID > 0 {
		oid := row.OrderID
		obs.SourceOrderID = &oid
		if r.Orders != nil {
			if id, e := r.Orders.ResolveAdvertisingOrder(ctx, seller, oid, cutoff); e != nil {
				return CPOObservation{}, e
			} else if id != nil {
				obs.CanonicalOrderID = id
				obs.OrderResolutionStatus = "RESOLVED"
			}
		}
	}
	return obs, nil
}
func decimal(raw string) (*sales.Decimal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	v, e := sales.ParseAmount(json.Number(raw))
	if e != nil {
		return nil, e
	}
	return &v, nil
}
func date(v time.Time) time.Time {
	if v.IsZero() {
		return time.Time{}
	}
	return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
}
func datePtr(v time.Time) *time.Time {
	if v.IsZero() {
		return nil
	}
	x := date(v)
	return &x
}
func meta(c collection.SourceContract) map[string]any {
	return map[string]any{"contract_key": c.ContractKey, "contract_version": c.ContractVersion, "semantic_mapping_hash": c.SemanticMappingHash, "redaction_profile": RedactionProfile}
}
