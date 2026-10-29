package ordersync

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/rawpayloads"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultInitialOrdersLookback = 30 * 24 * time.Hour

type Service struct {
	queries         *dbgen.Queries
	ozonService     *ozon.Service
	ozonClient      *ozon.Client
	rawPayloads     *rawpayloads.Service
	ordersPageLimit int
	salesPageSize   int
}

type RunInput struct {
	SellerAccountID int64
	ImportJobID     int64
	SourceCursor    string
}

type RunResult struct {
	RecordsReceived int32
	RecordsImported int32
	RecordsFailed   int32
	NextCursorValue string
}

func NewService(
	db *pgxpool.Pool,
	ozonService *ozon.Service,
	rawPayloads *rawpayloads.Service,
) *Service {
	return &Service{
		queries:         dbgen.New(db),
		ozonService:     ozonService,
		ozonClient:      ozon.NewClient(),
		rawPayloads:     rawPayloads,
		ordersPageLimit: 1000,
		salesPageSize:   1000,
	}
}

func (s *Service) Run(ctx context.Context, input RunInput) (RunResult, error) {
	creds, err := s.ozonService.GetDecryptedCredentials(ctx, input.SellerAccountID)
	if err != nil {
		return RunResult{}, fmt.Errorf("get decrypted ozon credentials: %w", err)
	}

	since, upperBound, err := s.resolveWindow(input.SourceCursor)
	if err != nil {
		return RunResult{}, fmt.Errorf("resolve orders window: %w", err)
	}

	var totalReceived int32
	var totalImported int32

	ordersResult, err := s.importOrders(ctx, creds, input, since, upperBound)
	if err != nil {
		return RunResult{}, err
	}
	totalReceived += ordersResult.RecordsReceived
	totalImported += ordersResult.RecordsImported

	salesResult, err := s.importSales(ctx, creds, input, since, upperBound)
	if err != nil {
		return RunResult{}, err
	}
	totalReceived += salesResult.RecordsReceived
	totalImported += salesResult.RecordsImported

	return RunResult{
		RecordsReceived: totalReceived,
		RecordsImported: totalImported,
		RecordsFailed:   0,
		NextCursorValue: upperBound.Format(time.RFC3339),
	}, nil
}

func (s *Service) resolveWindow(sourceCursor string) (time.Time, time.Time, error) {
	upperBound := time.Now().UTC().Truncate(time.Second)

	if sourceCursor == "" {
		return upperBound.Add(-defaultInitialOrdersLookback), upperBound, nil
	}

	since, err := time.Parse(time.RFC3339, sourceCursor)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	return since.UTC(), upperBound, nil
}

func (s *Service) importOrders(
	ctx context.Context,
	creds ozon.DecryptedCredentials,
	input RunInput,
	since time.Time,
	upperBound time.Time,
) (RunResult, error) {
	var received int32
	var imported int32

	offset := 0
	for {
		req := ozon.ListOrdersRequest{
			Limit:  s.ordersPageLimit,
			Offset: offset,
			Dir:    "ASC",
		}
		req.Filter.Since = since.Format(time.RFC3339)
		req.Filter.To = upperBound.Format(time.RFC3339)
		req.With.FinancialData = true

		resp, err := s.ozonClient.ListOrders(ctx, creds.ClientID, creds.APIKey, req)
		if err != nil {
			return RunResult{}, fmt.Errorf("fetch orders page offset=%d: %w", offset, err)
		}

		if _, err := s.rawPayloads.Save(ctx, rawpayloads.SaveInput{
			SellerAccountID: input.SellerAccountID,
			ImportJobID:     input.ImportJobID,
			Domain:          "orders",
			Source:          "ozon.v3.posting.fbs.list",
			RequestKey:      buildOrdersRequestKey(req),
			Body:            resp.Raw,
		}); err != nil {
			return RunResult{}, fmt.Errorf("save raw orders payload offset=%d: %w", offset, err)
		}

		pageItems := resp.Data.Postings
		received += int32(len(pageItems))

		for _, item := range pageItems {
			rawSubset, err := json.Marshal(item)
			if err != nil {
				return RunResult{}, fmt.Errorf("marshal order raw subset: %w", err)
			}

			createdAt, err := parseOptionalRFC3339(item.CreatedAt)
			if err != nil {
				return RunResult{}, fmt.Errorf("parse order created_at: %w", err)
			}

			processedAt, err := parseOptionalRFC3339(item.InProcessAt)
			if err != nil {
				return RunResult{}, fmt.Errorf("parse order in_process_at: %w", err)
			}

			totalAmount, currencyCode := deriveOrderAmountAndCurrency(item)

			if _, err := s.queries.UpsertOrder(ctx, dbgen.UpsertOrderParams{
				SellerAccountID:   input.SellerAccountID,
				OzonOrderID:       strconv.FormatInt(item.OrderID, 10),
				PostingNumber:     nullableText(item.PostingNumber),
				Status:            nullableText(item.Status),
				CreatedAtSource:   createdAt,
				ProcessedAtSource: processedAt,
				TotalAmount:       totalAmount,
				CurrencyCode:      currencyCode,
				RawAttributes:     rawSubset,
			}); err != nil {
				return RunResult{}, fmt.Errorf("upsert order order_id=%d: %w", item.OrderID, err)
			}

			imported++
		}

		if !resp.Data.HasNext {
			break
		}

		offset += s.ordersPageLimit
	}

	return RunResult{
		RecordsReceived: received,
		RecordsImported: imported,
		RecordsFailed:   0,
	}, nil
}

func (s *Service) importSales(
	ctx context.Context,
	creds ozon.DecryptedCredentials,
	input RunInput,
	since time.Time,
	upperBound time.Time,
) (RunResult, error) {
	var received int32
	var imported int32

	page := 1
	for {
		req := ozon.ListSalesRequest{
			From:     since.Format(time.RFC3339),
			To:       upperBound.Format(time.RFC3339),
			Page:     page,
			PageSize: s.salesPageSize,
		}

		resp, err := s.ozonClient.ListSales(ctx, creds.ClientID, creds.APIKey, req)
		if err != nil {
			return RunResult{}, fmt.Errorf("fetch sales page=%d: %w", page, err)
		}

		if _, err := s.rawPayloads.Save(ctx, rawpayloads.SaveInput{
			SellerAccountID: input.SellerAccountID,
			ImportJobID:     input.ImportJobID,
			Domain:          "orders",
			Source:          "ozon.v3.finance.transaction.list",
			RequestKey:      buildSalesRequestKey(req),
			Body:            resp.Raw,
		}); err != nil {
			return RunResult{}, fmt.Errorf("save raw sales payload page=%d: %w", page, err)
		}

		pageItems := resp.Data.Operations
		received += int32(len(pageItems))

		for _, item := range pageItems {
			rawSubset, err := json.Marshal(item)
			if err != nil {
				return RunResult{}, fmt.Errorf("marshal sale raw subset: %w", err)
			}

			saleID := item.OperationID.String()
			if saleID == "" {
				return RunResult{}, fmt.Errorf("empty operation_id in sales payload")
			}

			saleDate, err := parseOptionalRFC3339(item.OperationDate)
			if err != nil {
				return RunResult{}, fmt.Errorf("parse sale operation_date: %w", err)
			}

			if _, err := s.queries.UpsertSale(ctx, dbgen.UpsertSaleParams{
				SellerAccountID: input.SellerAccountID,
				OzonSaleID:      saleID,
				OzonOrderID:     nullableText(item.Posting.OrderID.String()),
				PostingNumber:   nullableText(item.Posting.PostingNumber),
				Quantity:        nullableInt32(sumSaleQuantity(item.Items)),
				Amount:          nullableNumeric(item.Amount),
				CurrencyCode:    nullableText(item.CurrencyCode),
				SaleDate:        saleDate,
				RawAttributes:   rawSubset,
			}); err != nil {
				return RunResult{}, fmt.Errorf("upsert sale operation_id=%s: %w", saleID, err)
			}

			imported++
		}

		if len(pageItems) < s.salesPageSize {
			break
		}

		page++
	}

	return RunResult{
		RecordsReceived: received,
		RecordsImported: imported,
		RecordsFailed:   0,
	}, nil
}

func buildOrdersRequestKey(req ozon.ListOrdersRequest) string {
	return fmt.Sprintf(
		"orders:dir:%s:since:%s:to:%s:offset:%d:limit:%d",
		req.Dir,
		req.Filter.Since,
		req.Filter.To,
		req.Offset,
		req.Limit,
	)
}

func buildSalesRequestKey(req ozon.ListSalesRequest) string {
	return fmt.Sprintf(
		"sales:from:%s:to:%s:page:%d:size:%d",
		req.From,
		req.To,
		req.Page,
		req.PageSize,
	)
}

func deriveOrderAmountAndCurrency(item ozon.OrderItem) (pgtype.Numeric, pgtype.Text) {
	var total float64
	currency := ""

	for _, p := range item.FinancialData.Products {
		price, err := strconv.ParseFloat(p.Price, 64)
		if err != nil {
			continue
		}
		total += price * float64(p.Quantity)
		if currency == "" {
			currency = p.CurrencyCode
		}
	}

	if total == 0 {
		return pgtype.Numeric{Valid: false}, nullableText(currency)
	}

	return nullableNumeric(fmt.Sprintf("%.2f", total)), nullableText(currency)
}

func sumSaleQuantity(items []ozon.SalesItem) int32 {
	var total int32
	for _, item := range items {
		total += item.Quantity
	}
	return total
}

func parseOptionalRFC3339(v string) (pgtype.Timestamptz, error) {
	if v == "" {
		return pgtype.Timestamptz{Valid: false}, nil
	}

	parsed, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return pgtype.Timestamptz{}, err
	}

	return pgtype.Timestamptz{
		Time:  parsed,
		Valid: true,
	}, nil
}

func nullableText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{
		String: v,
		Valid:  true,
	}
}

func nullableInt32(v int32) pgtype.Int4 {
	if v == 0 {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{
		Int32: v,
		Valid: true,
	}
}

func nullableNumeric(v string) pgtype.Numeric {
	if v == "" {
		return pgtype.Numeric{Valid: false}
	}

	var n pgtype.Numeric
	if err := n.Scan(v); err != nil {
		return pgtype.Numeric{Valid: false}
	}
	return n
}
