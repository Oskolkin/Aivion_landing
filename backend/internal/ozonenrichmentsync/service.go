package ozonenrichmentsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon/performance"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ozonCredentials interface {
	GetDecryptedCredentials(ctx context.Context, sellerAccountID int64) (ozon.DecryptedCredentials, error)
	GetDecryptedPerformanceBearerToken(ctx context.Context, sellerAccountID int64) (string, error)
}

type sellerAPI interface {
	GetSellerAPIRoles(ctx context.Context, clientID, apiKey string) (*ozon.TypedResponse[ozon.RolesResponse], error)
	GetProductInfoList(ctx context.Context, clientID, apiKey string, ids ozon.ProductInfoListRequestIDs) (*ozon.TypedResponse[ozon.ProductInfoListResult], error)
	GetRelatedSKUs(ctx context.Context, clientID, apiKey string, skus []int64) (*ozon.TypedResponse[ozon.RelatedSKUsResponse], error)
	GetAnalyticsStocks(ctx context.Context, clientID, apiKey string, req ozon.AnalyticsStocksRequest) (*ozon.TypedResponse[[]ozon.AnalyticsStockItem], error)
}

type performanceAPI interface {
	ListPerformanceCampaigns(ctx context.Context, clientID, bearerToken string, filter performance.CampaignListFilter) (*performance.TypedResponse[performance.ListCampaignsResult], error)
	GetPerformanceDailyStatisticsJSON(ctx context.Context, clientID, bearerToken string, campaignIDs []int64, dateFrom, dateTo time.Time) (*performance.TypedResponse[performance.CampaignStatisticsResult], error)
	GetPerformanceCampaignObjects(ctx context.Context, clientID, bearerToken string, campaignID int64, advObjectType string) (*performance.TypedResponse[performance.CampaignObjectsResult], error)
	GetPerformanceCampaignProductsV2(ctx context.Context, clientID, bearerToken string, campaignID int64, page, pageSize int64) (*performance.TypedResponse[performance.CampaignProductsResult], error)
}

type Service struct {
	queries     *dbgen.Queries
	credentials ozonCredentials
	sellerAPI   sellerAPI
	performance performanceAPI
}

func NewService(db *pgxpool.Pool, credentials ozonCredentials) *Service {
	return &Service{
		queries:     dbgen.New(db),
		credentials: credentials,
		sellerAPI:   ozon.NewClient(),
		performance: performance.NewClient(),
	}
}

func NewServiceWithClients(db *pgxpool.Pool, credentials ozonCredentials, seller sellerAPI, perf performanceAPI) *Service {
	return &Service{
		queries:     dbgen.New(db),
		credentials: credentials,
		sellerAPI:   seller,
		performance: perf,
	}
}

func (s *Service) SyncOzonEnrichmentP0(ctx context.Context, input SyncInput) (SyncResult, error) {
	scope := normalizeScope(input.Scope)
	if scope != scopeP0 {
		return SyncResult{}, fmt.Errorf("unsupported enrichment scope: %s", input.Scope)
	}
	metricDate := input.MetricDate.UTC()
	if metricDate.IsZero() {
		metricDate = time.Now().UTC()
	}
	metricDate = time.Date(metricDate.Year(), metricDate.Month(), metricDate.Day(), 0, 0, 0, 0, time.UTC)

	startedAt := time.Now().UTC()
	run, err := s.queries.CreateOzonEnrichmentSyncRun(ctx, dbgen.CreateOzonEnrichmentSyncRunParams{
		SellerAccountID: input.SellerAccountID,
		MetricDate:      pgDate(metricDate),
		Scope:           scope,
		StartedAt:       pgTimestamptz(startedAt),
	})
	if err != nil {
		return SyncResult{}, fmt.Errorf("create enrichment sync run: %w", err)
	}

	result := SyncResult{RunID: run.ID}
	var diagErrors []diagnosticEntry
	var diagWarnings []diagnosticEntry

	creds, credErr := s.credentials.GetDecryptedCredentials(ctx, input.SellerAccountID)
	sellerAPIAvailable := credErr == nil && strings.TrimSpace(creds.ClientID) != "" && strings.TrimSpace(creds.APIKey) != ""

	bearerToken, perfErr := s.credentials.GetDecryptedPerformanceBearerToken(ctx, input.SellerAccountID)
	performanceAPIAvailable := perfErr == nil && strings.TrimSpace(bearerToken) != ""
	if errors.Is(perfErr, ozon.ErrPerformanceTokenNotConfigured) {
		diagWarnings = append(diagWarnings, diagnosticEntry{
			Source:  "performance_api",
			Message: "performance token not configured",
		})
	}

	if credErr != nil {
		diagErrors = append(diagErrors, diagnosticEntry{Source: "seller_api", Message: sanitizeErrorMessage(credErr.Error())})
	} else if !sellerAPIAvailable {
		diagWarnings = append(diagWarnings, diagnosticEntry{Source: "seller_api", Message: "seller credentials missing"})
	}

	if sellerAPIAvailable {
		if count, warn, err := s.syncSellerAPIRoles(ctx, input.SellerAccountID, creds); err != nil {
			diagErrors = append(diagErrors, diagnosticEntry{Source: "roles", Message: sanitizeErrorMessage(err.Error())})
		} else if warn != "" {
			diagWarnings = append(diagWarnings, diagnosticEntry{Source: "roles", Message: warn})
		} else if count == 0 {
			_ = count
		}
	}

	products, err := s.queries.ListProductsBySellerAccountID(ctx, input.SellerAccountID)
	if err != nil {
		diagErrors = append(diagErrors, diagnosticEntry{Source: "products", Message: sanitizeErrorMessage(err.Error())})
	}

	productsBySKU := make(map[int64]int64, len(products))
	productIDs := make([]int64, 0, len(products))
	skuIDs := make([]int64, 0, len(products))
	for _, p := range products {
		if p.OzonProductID > 0 {
			productIDs = append(productIDs, p.OzonProductID)
		}
		if p.Sku.Valid && p.Sku.Int64 > 0 {
			skuIDs = append(skuIDs, p.Sku.Int64)
			productsBySKU[p.Sku.Int64] = p.OzonProductID
		}
	}

	if sellerAPIAvailable {
		count, warns, err := s.syncProductInfo(ctx, input.SellerAccountID, metricDate, creds, productIDs, skuIDs, products)
		result.ProductInfoCount = count
		for _, w := range warns {
			diagWarnings = append(diagWarnings, diagnosticEntry{Source: "product_info", Message: w})
		}
		if err != nil {
			diagErrors = append(diagErrors, diagnosticEntry{Source: "product_info", Message: sanitizeErrorMessage(err.Error())})
		}

		count, warns, err = s.syncRelatedSKUs(ctx, input.SellerAccountID, creds, skuIDs)
		result.RelatedSKUCount = count
		for _, w := range warns {
			diagWarnings = append(diagWarnings, diagnosticEntry{Source: "related_sku", Message: w})
		}
		if err != nil {
			diagErrors = append(diagErrors, diagnosticEntry{Source: "related_sku", Message: sanitizeErrorMessage(err.Error())})
		}

		count, warns, err = s.syncStockAnalytics(ctx, input.SellerAccountID, metricDate, creds, skuIDs)
		result.StockAnalyticsCount = count
		for _, w := range warns {
			diagWarnings = append(diagWarnings, diagnosticEntry{Source: "stock_analytics", Message: w})
		}
		if err != nil {
			diagErrors = append(diagErrors, diagnosticEntry{Source: "stock_analytics", Message: sanitizeErrorMessage(err.Error())})
		}
	}

	if performanceAPIAvailable {
		campaigns, count, warns, err := s.syncCampaigns(ctx, input.SellerAccountID, creds.ClientID, bearerToken)
		result.CampaignCount = count
		for _, w := range warns {
			diagWarnings = append(diagWarnings, diagnosticEntry{Source: "campaigns", Message: w})
		}
		if err != nil {
			diagErrors = append(diagErrors, diagnosticEntry{Source: "campaigns", Message: sanitizeErrorMessage(err.Error())})
		} else {
			statsFrom, statsTo := statsDateRange(metricDate)
			campaignIDs := make([]int64, 0, len(campaigns))
			for _, c := range campaigns {
				campaignIDs = append(campaignIDs, c.CampaignExternalID)
			}
			count, warns, err = s.syncCampaignDailyStats(ctx, input.SellerAccountID, creds.ClientID, bearerToken, campaignIDs, statsFrom, statsTo)
			result.CampaignDailyStatsCount = count
			for _, w := range warns {
				diagWarnings = append(diagWarnings, diagnosticEntry{Source: "campaign_stats", Message: w})
			}
			if err != nil {
				diagErrors = append(diagErrors, diagnosticEntry{Source: "campaign_stats", Message: sanitizeErrorMessage(err.Error())})
			}

			objectsCount, productsCount, warns, err := s.syncCampaignDetails(ctx, input.SellerAccountID, creds.ClientID, bearerToken, campaigns, productsBySKU)
			result.CampaignObjectsCount = objectsCount
			result.CampaignProductsCount = productsCount
			for _, w := range warns {
				diagWarnings = append(diagWarnings, diagnosticEntry{Source: "campaign_details", Message: w})
			}
			if err != nil {
				diagErrors = append(diagErrors, diagnosticEntry{Source: "campaign_details", Message: sanitizeErrorMessage(err.Error())})
			}
		}
	}

	result.SellerAPIAvailable = sellerAPIAvailable
	result.PerformanceAPIAvailable = performanceAPIAvailable
	result.Errors = flattenDiagnostics(diagErrors)
	result.Warnings = flattenDiagnostics(diagWarnings)

	importedAny := result.ProductInfoCount > 0 ||
		result.RelatedSKUCount > 0 ||
		result.StockAnalyticsCount > 0 ||
		result.CampaignCount > 0 ||
		result.CampaignDailyStatsCount > 0 ||
		result.CampaignObjectsCount > 0 ||
		result.CampaignProductsCount > 0

	result.Status = finalizeRunStatus(len(diagErrors), len(diagWarnings), importedAny)

	finishedAt := time.Now().UTC()
	if _, err := s.queries.FinishOzonEnrichmentSyncRun(ctx, dbgen.FinishOzonEnrichmentSyncRunParams{
		ID:                      run.ID,
		Status:                  result.Status,
		FinishedAt:              pgTimestamptz(finishedAt),
		SellerApiAvailable:      sellerAPIAvailable,
		PerformanceApiAvailable: performanceAPIAvailable,
		ProductInfoCount:        result.ProductInfoCount,
		RelatedSkuCount:         result.RelatedSKUCount,
		StockAnalyticsCount:     result.StockAnalyticsCount,
		CampaignCount:           result.CampaignCount,
		CampaignDailyStatsCount: result.CampaignDailyStatsCount,
		CampaignObjectsCount:    result.CampaignObjectsCount,
		CampaignProductsCount:   result.CampaignProductsCount,
		ErrorsJson:              mustJSON(diagErrors),
		WarningsJson:            mustJSON(diagWarnings),
		RawSummaryJson:          mustJSON(result),
	}); err != nil {
		return result, fmt.Errorf("finish enrichment sync run: %w", err)
	}

	return result, nil
}

func flattenDiagnostics(entries []diagnosticEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, fmt.Sprintf("%s: %s", e.Source, e.Message))
	}
	return out
}

func (s *Service) syncSellerAPIRoles(ctx context.Context, sellerAccountID int64, creds ozon.DecryptedCredentials) (int, string, error) {
	capturedAt := time.Now().UTC()
	resp, err := s.sellerAPI.GetSellerAPIRoles(ctx, creds.ClientID, creds.APIKey)
	status, code, message := classifySellerAPIStatus(err)
	var roles ozon.RolesResponse
	var raw []byte
	if resp != nil {
		roles = resp.Data
		raw = resp.Raw
	}
	methods := dedupeMethods(roles)
	rolesJSON := mustJSON(roles.Roles)
	methodsJSON := mustJSON(methods)

	var expiresAt pgtype.Timestamptz
	if strings.TrimSpace(roles.ExpiresAt) != "" {
		expiresAt = parseOzonTimestamp(roles.ExpiresAt)
	}

	if _, err := s.queries.UpsertOzonAPICapability(ctx, dbgen.UpsertOzonAPICapabilityParams{
		SellerAccountID: sellerAccountID,
		Source:          sourceSellerAPI,
		CapturedAt:      pgTimestamptz(capturedAt),
		ExpiresAt:       expiresAt,
		RolesJson:       rolesJSON,
		MethodsJson:     methodsJSON,
		RawResponseJson: raw,
		Status:          status,
		ErrorCode:       nullableText(code),
		ErrorMessage:    nullableText(message),
	}); err != nil {
		return 0, "", fmt.Errorf("upsert seller api capability: %w", err)
	}
	if err != nil && status != statusAvailable {
		return 0, message, nil
	}
	return len(methods), "", nil
}

func (s *Service) syncProductInfo(
	ctx context.Context,
	sellerAccountID int64,
	snapshotDate time.Time,
	creds ozon.DecryptedCredentials,
	productIDs []int64,
	skuIDs []int64,
	products []dbgen.Product,
) (int32, []string, error) {
	capturedAt := time.Now().UTC()
	pgSnapshotDate := pgDate(snapshotDate)
	imported := int32(0)
	var warnings []string

	chunks := chunkProductIDs(productIDs, skuIDs, productInfoChunkSize)
	for _, chunk := range chunks {
		resp, err := s.sellerAPI.GetProductInfoList(ctx, creds.ClientID, creds.APIKey, chunk)
		if err != nil {
			return imported, warnings, err
		}
		for _, item := range resp.Data.Items {
			rawItem, _ := json.Marshal(item)
			params := buildProductInfoParams(sellerAccountID, capturedAt, pgSnapshotDate, item, rawItem)
			var upsertErr error
			if item.ID > 0 {
				_, upsertErr = s.queries.UpsertOzonProductInfoSnapshotByProductID(ctx, params.toByProductID())
			} else if item.SKU > 0 {
				_, upsertErr = s.queries.UpsertOzonProductInfoSnapshotBySKU(ctx, params.toBySKU())
			} else {
				warnings = append(warnings, "skipped product info item without product_id and sku")
				continue
			}
			if upsertErr != nil {
				warnings = append(warnings, fmt.Sprintf("product info upsert failed product_id=%d sku=%d", item.ID, item.SKU))
				continue
			}
			imported++
			if item.ID > 0 {
				if err := s.touchProductCanonical(ctx, sellerAccountID, item, rawItem); err != nil {
					warnings = append(warnings, fmt.Sprintf("product canonical update failed product_id=%d", item.ID))
				}
			}
		}
	}
	_ = products
	return imported, warnings, nil
}

type productInfoParams struct {
	dbgen.UpsertOzonProductInfoSnapshotByProductIDParams
}

func (p productInfoParams) toByProductID() dbgen.UpsertOzonProductInfoSnapshotByProductIDParams {
	return p.UpsertOzonProductInfoSnapshotByProductIDParams
}

func (p productInfoParams) toBySKU() dbgen.UpsertOzonProductInfoSnapshotBySKUParams {
	src := p.UpsertOzonProductInfoSnapshotByProductIDParams
	return dbgen.UpsertOzonProductInfoSnapshotBySKUParams{
		SellerAccountID:                 src.SellerAccountID,
		CapturedAt:                      src.CapturedAt,
		SnapshotDate:                    src.SnapshotDate,
		ProductID:                       src.ProductID,
		Sku:                             src.Sku,
		OfferID:                         src.OfferID,
		Name:                            src.Name,
		DescriptionCategoryID:           src.DescriptionCategoryID,
		TypeID:                          src.TypeID,
		CurrencyCode:                    src.CurrencyCode,
		Price:                           src.Price,
		OldPrice:                        src.OldPrice,
		MinPrice:                        src.MinPrice,
		Vat:                             src.Vat,
		IsArchived:                      src.IsArchived,
		IsAutoarchived:                  src.IsAutoarchived,
		IsDiscounted:                    src.IsDiscounted,
		HasDiscountedFboItem:            src.HasDiscountedFboItem,
		DiscountedFboStocks:             src.DiscountedFboStocks,
		IsKgt:                           src.IsKgt,
		IsSuper:                         src.IsSuper,
		Status:                          src.Status,
		StatusName:                      src.StatusName,
		StatusDescription:               src.StatusDescription,
		StatusFailed:                    src.StatusFailed,
		ModerateStatus:                  src.ModerateStatus,
		ValidationStatus:                src.ValidationStatus,
		StatusUpdatedAt:                 src.StatusUpdatedAt,
		VisibilityHasPrice:              src.VisibilityHasPrice,
		VisibilityHasStock:              src.VisibilityHasStock,
		PriceIndexColor:                 src.PriceIndexColor,
		ExternalMinimalPrice:            src.ExternalMinimalPrice,
		ExternalPriceIndexValue:         src.ExternalPriceIndexValue,
		OzonMinimalPrice:                src.OzonMinimalPrice,
		OzonPriceIndexValue:             src.OzonPriceIndexValue,
		SelfMarketplacesMinimalPrice:    src.SelfMarketplacesMinimalPrice,
		SelfMarketplacesPriceIndexValue: src.SelfMarketplacesPriceIndexValue,
		PrimaryImage:                    src.PrimaryImage,
		ImagesJson:                      src.ImagesJson,
		BarcodesJson:                    src.BarcodesJson,
		CommissionsJson:                 src.CommissionsJson,
		PromotionsJson:                  src.PromotionsJson,
		ErrorsJson:                      src.ErrorsJson,
		AvailabilitiesJson:              src.AvailabilitiesJson,
		StocksJson:                      src.StocksJson,
		RawResponseJson:                 src.RawResponseJson,
	}
}

func buildProductInfoParams(
	sellerAccountID int64,
	capturedAt time.Time,
	snapshotDate pgtype.Date,
	item ozon.ProductInfoListItem,
	rawItem []byte,
) productInfoParams {
	status := item.Statuses.Status
	if status == "" {
		status = item.Statuses.State
	}
	if status == "" {
		status = item.Status.State
	}
	return productInfoParams{
		UpsertOzonProductInfoSnapshotByProductIDParams: dbgen.UpsertOzonProductInfoSnapshotByProductIDParams{
			SellerAccountID:                 sellerAccountID,
			CapturedAt:                      pgTimestamptz(capturedAt),
			SnapshotDate:                    snapshotDate,
			ProductID:                       nullableInt64(item.ID),
			Sku:                             nullableInt64(item.SKU),
			OfferID:                         nullableText(item.OfferID),
			Name:                            nullableText(item.Name),
			DescriptionCategoryID:           nullableInt64(item.DescriptionCategoryID),
			TypeID:                          nullableInt64(item.TypeID),
			CurrencyCode:                    nullableText(item.CurrencyCode),
			Price:                           parseDecimalString(item.Price),
			OldPrice:                        parseDecimalString(item.OldPrice),
			MinPrice:                        parseDecimalString(item.MinPrice),
			Vat:                             nullableText(item.Vat),
			IsArchived:                      nullableBool(item.IsArchived || item.Archived),
			IsAutoarchived:                  nullableBool(item.IsAutoarchived),
			IsDiscounted:                    nullableBool(item.IsDiscounted),
			HasDiscountedFboItem:            nullableBool(item.HasDiscountedFBOItem),
			DiscountedFboStocks:             nullableInt32(int32(item.DiscountedFBOStocks)),
			IsKgt:                           nullableBool(item.IsKGT),
			IsSuper:                         nullableBool(item.IsSuper),
			Status:                          nullableText(status),
			StatusName:                      nullableText(item.Statuses.StatusName),
			StatusDescription:               nullableText(item.Statuses.StatusDescription),
			StatusFailed:                    nullableText(item.Statuses.StatusFailed),
			ModerateStatus:                  nullableText(item.Statuses.ModerateStatus),
			ValidationStatus:                nullableText(item.Statuses.ValidationStatus),
			StatusUpdatedAt:                 parseOzonTimestamp(item.Statuses.StatusUpdatedAt),
			VisibilityHasPrice:              nullableBool(item.VisibilityDetails.HasPrice),
			VisibilityHasStock:              nullableBool(item.VisibilityDetails.HasStock),
			PriceIndexColor:                 nullableText(item.PriceIndexes.ColorIndex),
			ExternalMinimalPrice:            parseDecimalString(item.PriceIndexes.ExternalIndexData.MinimalPrice),
			ExternalPriceIndexValue:         parseDecimalIndex(item.PriceIndexes.ExternalIndexData.PriceIndexValue),
			OzonMinimalPrice:                parseDecimalString(item.PriceIndexes.OzonIndexData.MinimalPrice),
			OzonPriceIndexValue:             parseDecimalIndex(item.PriceIndexes.OzonIndexData.PriceIndexValue),
			SelfMarketplacesMinimalPrice:    parseDecimalString(item.PriceIndexes.SelfMarketplacesIndexData.MinimalPrice),
			SelfMarketplacesPriceIndexValue: parseDecimalIndex(item.PriceIndexes.SelfMarketplacesIndexData.PriceIndexValue),
			PrimaryImage:                    nullableText(primaryImageURL(item.PrimaryImage)),
			ImagesJson:                      mustJSON(item.Images),
			BarcodesJson:                    mustJSON(item.Barcodes),
			CommissionsJson:                 mustJSON(item.Commissions),
			PromotionsJson:                  mustJSON(item.Promotions),
			ErrorsJson:                      mustJSON(item.Errors),
			AvailabilitiesJson:              mustJSON(item.Availabilities),
			StocksJson:                      mustJSON(item.Stocks),
			RawResponseJson:                 rawItem,
		},
	}
}

func (s *Service) touchProductCanonical(ctx context.Context, sellerAccountID int64, item ozon.ProductInfoListItem, rawItem []byte) error {
	status := item.Statuses.Status
	if status == "" {
		status = item.Statuses.State
	}
	_, err := s.queries.UpsertProduct(ctx, dbgen.UpsertProductParams{
		SellerAccountID:       sellerAccountID,
		OzonProductID:         item.ID,
		OfferID:               nullableText(item.OfferID),
		Sku:                   nullableInt64(item.SKU),
		Name:                  item.Name,
		Status:                nullableText(status),
		ReferencePrice:        parseDecimalString(item.Price),
		OldPrice:              parseDecimalString(item.OldPrice),
		OzonMinPrice:          parseDecimalString(item.MinPrice),
		DescriptionCategoryID: nullableInt64(item.DescriptionCategoryID),
		IsArchived:            item.IsArchived || item.Archived,
		RawAttributes:         rawItem,
		PrimaryImageUrl:       nullableText(primaryImageURL(item.PrimaryImage)),
		ImageUrls:             mustJSON(item.Images),
		SourceUpdatedAt:       parseOzonTimestamp(item.UpdatedAt),
	})
	return err
}

func chunkProductIDs(productIDs []int64, skuIDs []int64, size int) []ozon.ProductInfoListRequestIDs {
	if len(productIDs) > 0 {
		return chunkInt64IDs(productIDs, size, func(chunk []int64) ozon.ProductInfoListRequestIDs {
			return ozon.ProductInfoListRequestIDs{ProductID: chunk}
		})
	}
	return chunkInt64IDs(skuIDs, size, func(chunk []int64) ozon.ProductInfoListRequestIDs {
		return ozon.ProductInfoListRequestIDs{SKU: chunk}
	})
}

func chunkInt64IDs(ids []int64, size int, build func([]int64) ozon.ProductInfoListRequestIDs) []ozon.ProductInfoListRequestIDs {
	if len(ids) == 0 {
		return nil
	}
	out := make([]ozon.ProductInfoListRequestIDs, 0)
	for start := 0; start < len(ids); start += size {
		end := start + size
		if end > len(ids) {
			end = len(ids)
		}
		out = append(out, build(ids[start:end]))
	}
	return out
}

func (s *Service) syncRelatedSKUs(ctx context.Context, sellerAccountID int64, creds ozon.DecryptedCredentials, skuIDs []int64) (int32, []string, error) {
	if len(skuIDs) == 0 {
		return 0, nil, nil
	}
	capturedAt := time.Now().UTC()
	imported := int32(0)
	var warnings []string

	for start := 0; start < len(skuIDs); start += relatedSKUChunkSize {
		end := start + relatedSKUChunkSize
		if end > len(skuIDs) {
			end = len(skuIDs)
		}
		batch := skuIDs[start:end]
		resp, err := s.sellerAPI.GetRelatedSKUs(ctx, creds.ClientID, creds.APIKey, batch)
		if err != nil {
			return imported, warnings, err
		}
		for _, item := range resp.Data.Items {
			if item.SKU == 0 {
				continue
			}
			requestedSKU := item.RequestedSKU
			if requestedSKU == 0 {
				if len(batch) == 1 {
					requestedSKU = batch[0]
				} else {
					requestedSKU = item.SKU
				}
			}
			if _, err := s.queries.UpsertOzonRelatedSKU(ctx, dbgen.UpsertOzonRelatedSKUParams{
				SellerAccountID: sellerAccountID,
				RequestedSku:    requestedSKU,
				RelatedSku:      item.SKU,
				ProductID:       nullableInt64(item.ProductID),
				DeliverySchema:  nullableText(item.DeliverySchema),
				Availability:    nullableText(item.Availability),
				DeletedAt:       parseOzonTimestamp(item.DeletedAt),
				CapturedAt:      pgTimestamptz(capturedAt),
				RawResponseJson: resp.Raw,
			}); err != nil {
				warnings = append(warnings, fmt.Sprintf("related sku upsert failed requested=%d related=%d", requestedSKU, item.SKU))
				continue
			}
			imported++
		}
		for _, itemErr := range resp.Data.Errors {
			if _, err := s.queries.InsertOzonRelatedSKUError(ctx, dbgen.InsertOzonRelatedSKUErrorParams{
				SellerAccountID: sellerAccountID,
				RequestedSku:    itemErr.SKU,
				ErrorSku:        nullableInt64(itemErr.SKU),
				ErrorCode:       nullableText(itemErr.Code),
				ErrorMessage:    nullableText(itemErr.Message),
				CapturedAt:      pgTimestamptz(capturedAt),
			}); err != nil {
				warnings = append(warnings, fmt.Sprintf("related sku error persist failed sku=%d", itemErr.SKU))
			}
		}
	}
	return imported, warnings, nil
}

func (s *Service) syncStockAnalytics(ctx context.Context, sellerAccountID int64, snapshotDate time.Time, creds ozon.DecryptedCredentials, skuIDs []int64) (int32, []string, error) {
	if len(skuIDs) == 0 {
		return 0, nil, nil
	}
	capturedAt := time.Now().UTC()
	pgSnapshotDate := pgDate(snapshotDate)
	imported := int32(0)
	var warnings []string

	for start := 0; start < len(skuIDs); start += analyticsStocksChunkSize {
		end := start + analyticsStocksChunkSize
		if end > len(skuIDs) {
			end = len(skuIDs)
		}
		batchSKUs := make([]string, 0, end-start)
		for _, sku := range skuIDs[start:end] {
			batchSKUs = append(batchSKUs, strconv.FormatInt(sku, 10))
		}
		resp, err := s.sellerAPI.GetAnalyticsStocks(ctx, creds.ClientID, creds.APIKey, ozon.AnalyticsStocksRequest{SKUs: batchSKUs})
		if err != nil {
			return imported, warnings, err
		}
		for _, item := range resp.Data {
			if item.SKU <= 0 {
				continue
			}
			rawItem, _ := json.Marshal(item)
			clusterID := item.ClusterID
			if _, err := s.queries.UpsertStockAnalyticsSnapshot(ctx, dbgen.UpsertStockAnalyticsSnapshotParams{
				SellerAccountID:              sellerAccountID,
				SnapshotDate:                 pgSnapshotDate,
				Sku:                          item.SKU,
				OfferID:                      nullableText(item.OfferID),
				ProductName:                  nullableText(item.Name),
				WarehouseID:                  item.WarehouseID,
				WarehouseName:                nullableText(item.WarehouseName),
				ClusterID:                    clusterID,
				ClusterName:                  nullableText(item.ClusterName),
				MacrolocalClusterID:          nullableInt64(item.MacrolocalClusterID),
				AvailableStockCount:          item.AvailableStockCount,
				ExcessStockCount:             item.ExcessStockCount,
				Ads:                          nullableNumericFromFloat(item.ADS),
				AdsCluster:                   nullableNumericFromFloat(item.ADSCluster),
				Idc:                          nullableInt32FromFloat(item.IDC),
				IdcCluster:                   nullableNumericFromFloat(item.IDCCluster),
				TurnoverGrade:                nullableText(item.TurnoverGrade),
				TurnoverGradeCluster:         nullableText(item.TurnoverGradeCluster),
				DaysWithoutSales:             nullableInt32(item.DaysWithoutSales),
				DaysWithoutSalesCluster:      nullableInt32(item.DaysWithoutSalesCluster),
				RequestedStockCount:          nullableInt32(item.RequestedStockCount),
				TransitStockCount:            nullableInt32(item.TransitStockCount),
				ValidStockCount:              nullableInt32(item.ValidStockCount),
				WaitingDocsStockCount:        nullableInt32(item.WaitingDocsStockCount),
				ReturnFromCustomerStockCount: nullableInt32(item.ReturnFromCustomerStockCount),
				ReturnToSellerStockCount:     nullableInt32(item.ReturnToSellerStockCount),
				ExpiringStockCount:           nullableInt32(item.ExpiringStockCount),
				OtherStockCount:              nullableInt32(item.OtherStockCount),
				StockDefectStockCount:        nullableInt32(item.StockDefectStockCount),
				TransitDefectStockCount:      nullableInt32(item.TransitDefectStockCount),
				ItemTagsJson:                 mustJSON(item.ItemTags),
				CapturedAt:                   pgTimestamptz(capturedAt),
				RawResponseJson:              rawItem,
			}); err != nil {
				warnings = append(warnings, fmt.Sprintf("stock analytics upsert failed sku=%d", item.SKU))
				continue
			}
			imported++
		}
	}
	return imported, warnings, nil
}

func (s *Service) syncCampaigns(ctx context.Context, sellerAccountID int64, clientID, bearerToken string) ([]performance.Campaign, int32, []string, error) {
	var all []performance.Campaign
	var warnings []string
	for page := int64(1); ; page++ {
		resp, err := s.performance.ListPerformanceCampaigns(ctx, clientID, bearerToken, performance.CampaignListFilter{
			Page:     page,
			PageSize: campaignPageSize,
		})
		if err != nil {
			return all, int32(len(all)), warnings, err
		}
		for _, campaign := range resp.Data.Items {
			if _, err := s.queries.UpsertAdCampaign(ctx, buildAdCampaignParams(sellerAccountID, campaign)); err != nil {
				warnings = append(warnings, fmt.Sprintf("campaign upsert failed id=%d", campaign.CampaignExternalID))
				continue
			}
			all = append(all, campaign)
		}
		if len(resp.Data.Items) < campaignPageSize {
			break
		}
	}
	return all, int32(len(all)), warnings, nil
}

func buildAdCampaignParams(sellerAccountID int64, campaign performance.Campaign) dbgen.UpsertAdCampaignParams {
	capturedAt := time.Now().UTC()
	return dbgen.UpsertAdCampaignParams{
		SellerAccountID:          sellerAccountID,
		CampaignExternalID:       campaign.CampaignExternalID,
		CampaignName:             campaignName(campaign),
		CampaignType:             nullableText(campaign.CampaignType),
		PlacementType:            nullableText(campaign.PlacementType),
		Status:                   nullableText(campaign.Status),
		BudgetAmount:             nullableNumericString(campaign.BudgetAmount),
		BudgetDaily:              nullableNumericString(campaign.BudgetDaily),
		PaymentType:              nullableText(campaign.PaymentType),
		PlacementJson:            campaign.PlacementJSON,
		BudgetRawMicro:           nullableInt64(campaign.BudgetRawMicro),
		DailyBudgetRawMicro:      nullableInt64(campaign.DailyBudgetRawMicro),
		WeeklyBudgetRawMicro:     nullableInt64(campaign.WeeklyBudgetRawMicro),
		BudgetWeekly:             nullableNumericString(campaign.BudgetWeekly),
		FromDate:                 pgDate(campaign.FromDate),
		ToDate:                   pgDate(campaign.ToDate),
		AutostopStatus:           nullableText(campaign.AutostopStatus),
		IsAutocreated:            nullableBool(campaign.IsAutocreated),
		ProductAutopilotStrategy: nullableText(campaign.ProductAutopilotStrategy),
		ProductCampaignMode:      nullableText(campaign.ProductCampaignMode),
		PerformanceCreatedAt:     pgTimestamptz(campaign.CreatedAt),
		PerformanceUpdatedAt:     pgTimestamptz(campaign.UpdatedAt),
		CapturedAt:               pgTimestamptz(capturedAt),
		RawAttributes:            campaign.Raw,
	}
}

func (s *Service) syncCampaignDailyStats(
	ctx context.Context,
	sellerAccountID int64,
	clientID, bearerToken string,
	campaignIDs []int64,
	from, to time.Time,
) (int32, []string, error) {
	if len(campaignIDs) == 0 {
		return 0, nil, nil
	}
	resp, err := s.performance.GetPerformanceDailyStatisticsJSON(ctx, clientID, bearerToken, campaignIDs, from, to)
	if err != nil {
		return 0, nil, err
	}
	imported := int32(0)
	var warnings []string
	for _, metric := range resp.Data.Items {
		if _, err := s.queries.UpsertAdMetricDaily(ctx, dbgen.UpsertAdMetricDailyParams{
			SellerAccountID:    sellerAccountID,
			CampaignExternalID: metric.CampaignExternalID,
			MetricDate:         pgDate(metric.MetricDate),
			CampaignName:       nullableText(metric.CampaignName),
			Impressions:        metric.Impressions,
			Clicks:             metric.Clicks,
			Spend:              nullableNumericString(metric.Spend),
			OrdersCount:        int32(metric.OrdersCount),
			Revenue:            nullableNumericString(metric.Revenue),
			RawAttributes:      metric.Raw,
		}); err != nil {
			warnings = append(warnings, fmt.Sprintf("campaign stats upsert failed campaign=%d date=%s", metric.CampaignExternalID, metric.MetricDate.Format("2006-01-02")))
			continue
		}
		imported++
	}
	return imported, warnings, nil
}

func (s *Service) syncCampaignDetails(
	ctx context.Context,
	sellerAccountID int64,
	clientID, bearerToken string,
	campaigns []performance.Campaign,
	productsBySKU map[int64]int64,
) (int32, int32, []string, error) {
	objectsImported := int32(0)
	productsImported := int32(0)
	var warnings []string
	capturedAt := time.Now().UTC()

	for _, campaign := range campaigns {
		objectsResp, err := s.performance.GetPerformanceCampaignObjects(ctx, clientID, bearerToken, campaign.CampaignExternalID, campaign.CampaignType)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("campaign objects fetch failed campaign=%d: %s", campaign.CampaignExternalID, sanitizeErrorMessage(err.Error())))
		} else {
			for _, obj := range objectsResp.Data.Items {
				if _, err := s.queries.UpsertAdCampaignObject(ctx, dbgen.UpsertAdCampaignObjectParams{
					SellerAccountID:    sellerAccountID,
					CampaignExternalID: campaign.CampaignExternalID,
					ObjectExternalID:   obj.ObjectExternalID,
					ObjectType:         nullableText(obj.ObjectType),
					CapturedAt:         pgTimestamptz(capturedAt),
					RawResponseJson:    obj.Raw,
				}); err != nil {
					warnings = append(warnings, fmt.Sprintf("campaign object upsert failed campaign=%d object=%s", campaign.CampaignExternalID, obj.ObjectExternalID))
					continue
				}
				objectsImported++
			}
		}

		for page := int64(1); ; page++ {
			productsResp, err := s.performance.GetPerformanceCampaignProductsV2(ctx, clientID, bearerToken, campaign.CampaignExternalID, page, campaignProductsPageSize)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("campaign products fetch failed campaign=%d: %s", campaign.CampaignExternalID, sanitizeErrorMessage(err.Error())))
				break
			}
			for _, product := range productsResp.Data.Items {
				linkID := resolveProductLinkID(product.SKU, productsBySKU)
				if _, err := s.queries.UpsertAdCampaignSKU(ctx, dbgen.UpsertAdCampaignSKUParams{
					SellerAccountID:    sellerAccountID,
					CampaignExternalID: product.CampaignExternalID,
					OzonProductID:      linkID,
					OfferID:            pgtype.Text{Valid: false},
					Sku:                nullableInt64(product.SKU),
					IsActive:           product.IsActive,
					Status:             nullableText(product.Status),
					ProductTitle:       nullableText(product.Title),
					BidRawMicro:        nullableInt64(product.BidRawMicro),
					BidAmount:          nullableNumericString(product.BidAmount),
					BidUnitAssumption:  nullableText(product.BidUnitAssumption),
					CapturedAt:         pgTimestamptz(capturedAt),
					RawAttributes:      product.Raw,
				}); err != nil {
					warnings = append(warnings, fmt.Sprintf("campaign product upsert failed campaign=%d sku=%d", campaign.CampaignExternalID, product.SKU))
					continue
				}
				productsImported++
			}
			if len(productsResp.Data.Items) < int(campaignProductsPageSize) {
				break
			}
		}
	}
	return objectsImported, productsImported, warnings, nil
}
