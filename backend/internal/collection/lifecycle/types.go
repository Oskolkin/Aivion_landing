package lifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/sales"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

var ErrSourceContractMismatch = errors.New("SOURCE_CONTRACT_MISMATCH")
var ErrPaginationNonAdvancing = errors.New("PAGINATION_NON_ADVANCING")
var ErrResumeArtifactIntegrity = errors.New("RESUME_ARTIFACT_INTEGRITY_FAILURE")
var ErrOrderIdentityConflict = errors.New("ORDER_IDENTITY_CONFLICT")
var ErrPostingSchemeConflict = errors.New("POSTING_SCHEME_CONFLICT")
var ErrPostingLineageCycle = errors.New("POSTING_LINEAGE_CYCLE")
var ErrInvalidPostingQuantity = errors.New("INVALID_POSTING_QUANTITY")

func IsResumableInterruption(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ozon.ErrNetworkError) {
		return true
	}
	var p *ozon.ProviderError
	return errors.As(err, &p) && (p.StatusCode == 429 || p.StatusCode >= 500)
}
func ErrorCode(err error) string {
	var provider *ozon.ProviderError
	switch {
	case errors.As(err, &provider) && provider.StatusCode == 401:
		return "AUTH_FAILED"
	case errors.As(err, &provider) && provider.StatusCode == 403:
		return "PROVIDER_FORBIDDEN"
	case IsResumableInterruption(err):
		return "LIFECYCLE_COLLECTION_INTERRUPTED"
	case errors.Is(err, ErrResumeArtifactIntegrity):
		return "RESUME_ARTIFACT_INTEGRITY_FAILURE"
	case errors.Is(err, ErrPaginationNonAdvancing):
		return "PAGINATION_NON_ADVANCING"
	case errors.Is(err, ErrOrderIdentityConflict):
		return "ORDER_IDENTITY_CONFLICT"
	case errors.Is(err, ErrPostingSchemeConflict):
		return "POSTING_SCHEME_CONFLICT"
	case errors.Is(err, ErrPostingLineageCycle):
		return "POSTING_LINEAGE_CYCLE"
	case errors.Is(err, ErrInvalidPostingQuantity):
		return "INVALID_POSTING_QUANTITY"
	case errors.Is(err, ErrSourceContractMismatch):
		return "SOURCE_CONTRACT_MISMATCH"
	default:
		return "LIFECYCLE_COLLECTION_FAILED"
	}
}

type ProductCandidate struct {
	SourceSKU     int64
	OfferID, Name string
	Quantity      int64
	UnitPrice     sales.Decimal
	Currency      string
	PayloadID     int64
}
type PostingCandidate struct {
	OrderID                                                                    int64
	OrderNumber, PostingNumber, Scheme, Status, Substatus, ParentPostingNumber string
	ProviderCreatedAt, InProcessAt                                             *time.Time
	DeliveringDate, ShipmentDate                                               *time.Time
	CancelReasonID                                                             *int64
	CancelReason, CancellationType, CancellationInitiator                      string
	CancelledAfterShip                                                         *bool
	WarehouseID, ProviderID                                                    *int64
	Products                                                                   []ProductCandidate
	StateHash                                                                  string
	PayloadID                                                                  int64
	SourceReceivedAt                                                           time.Time
}
type ReturnCandidate struct {
	ProviderReturnID, SourceID, SourceOrderID, SourceSKU                                                             int64
	SourcePostingNumber, ProviderType, ProviderSchema, Reason, VisualStatus, VisualStatusSysName, CompensationStatus string
	Quantity                                                                                                         int64
	UnitPrice                                                                                                        sales.Decimal
	Currency                                                                                                         string
	ProviderReturnDate, VisualChangeAt, TechnicalReturnMoment, FinalMoment, CancelledWithCompensationMoment          *time.Time
	StateHash                                                                                                        string
	PayloadID                                                                                                        int64
	SourceReceivedAt                                                                                                 time.Time
}
type StreamResult struct {
	Status    string `json:"status"`
	ErrorCode string `json:"error_code,omitempty"`
	Pages     int    `json:"pages"`
	Records   int    `json:"records"`
	Details   int    `json:"details"`
	Resumed   bool   `json:"resumed"`
}
type PostingsResult struct {
	RequestedFrom, RequestedTo                                                                                                                                      time.Time `json:"-"`
	Mode                                                                                                                                                            string
	FBO, FBS                                                                                                                                                        StreamResult
	OrdersCreated, OrdersReused, PostingIdentitiesCreated, Postings, Relations, VersionsWritten, VersionsReused, ProductLines, ResolvedSKUs, UnresolvedSKUs, Events int
	postings                                                                                                                                                        []PostingCandidate
}

const (
	PostingModeInitialRange             = "INITIAL_RANGE"
	PostingModeExplicitRange            = "EXPLICIT_RANGE"
	PostingModeIncrementalChangedStatus = "INCREMENTAL_CHANGED_STATUS"
)

type ReturnsResult struct {
	RequestedFrom, RequestedTo                                                                                                        time.Time `json:"-"`
	Stream                                                                                                                            StreamResult
	Returns, ReturnsCreated, VersionsWritten, VersionsReused, ResolvedOrders, ResolvedPostings, ResolvedSKUs, UnresolvedLinks, Events int
	returns                                                                                                                           []ReturnCandidate
}

func parseTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05Z07:00", "2006-01-02"} {
		if v, err := time.Parse(layout, raw); err == nil {
			v = v.UTC()
			return &v
		}
	}
	return nil
}
func parseDate(raw string) *time.Time {
	v := parseTime(raw)
	if v == nil {
		return nil
	}
	d := time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
	return &d
}
func ptrInt(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}
func hash(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(sum[:])
}

func normalizePosting(raw ozon.CanonicalPosting, stream string, payloadID int64, received time.Time) (PostingCandidate, error) {
	c := PostingCandidate{OrderID: int64(raw.OrderID), OrderNumber: strings.TrimSpace(raw.OrderNumber), PostingNumber: strings.TrimSpace(raw.PostingNumber), Status: strings.TrimSpace(raw.Status), Substatus: strings.TrimSpace(raw.Substatus), ParentPostingNumber: strings.TrimSpace(raw.ParentPostingNumber), ProviderCreatedAt: parseTime(raw.CreatedAt), InProcessAt: parseTime(raw.InProcessAt), DeliveringDate: parseDate(raw.DeliveringDate), ShipmentDate: parseDate(raw.ShipmentDate), PayloadID: payloadID, SourceReceivedAt: received, WarehouseID: ptrInt(int64(raw.WarehouseID)), ProviderID: ptrInt(int64(raw.DeliveryMethod.ID))}
	if c.PostingNumber == "" {
		return c, fmt.Errorf("%w: posting requires posting_number", ErrSourceContractMismatch)
	}
	if stream == "FBO" {
		c.Scheme = "FBO"
	} else {
		schema := strings.ToUpper(strings.TrimSpace(raw.DeliveryMethod.Schema))
		switch {
		case schema == "FBS" || strings.Contains(schema, "FBS_STANDARD"):
			c.Scheme = "FBS"
		case schema == "RFBS" || strings.Contains(schema, "R_FBS") || strings.Contains(schema, "RFBS"):
			c.Scheme = "RFBS"
		default:
			c.Scheme = "UNKNOWN"
		}
	}
	if int64(raw.Cancellation.CancelReasonID) > 0 {
		v := int64(raw.Cancellation.CancelReasonID)
		c.CancelReasonID = &v
	}
	c.CancelReason, c.CancellationType, c.CancellationInitiator = raw.Cancellation.CancelReason, raw.Cancellation.CancellationType, raw.Cancellation.CancellationInitiator
	if c.CancelReason != "" || c.CancelReasonID != nil || c.CancellationType != "" {
		v := raw.Cancellation.CancelledAfterShip
		c.CancelledAfterShip = &v
	}
	for _, p := range raw.Products {
		sku := int64(p.SKU)
		q, err := sales.ParseUnits(json.Number(strings.TrimSpace(string(p.Quantity))))
		if err != nil {
			return c, fmt.Errorf("%w: %v", ErrInvalidPostingQuantity, err)
		}
		amount, err := sales.ParseAmount(json.Number(strings.TrimSpace(string(p.Price))))
		if err != nil {
			return c, fmt.Errorf("%w: posting unit price: %v", ErrSourceContractMismatch, err)
		}
		currency := strings.ToUpper(strings.TrimSpace(p.CurrencyCode))
		if currency == "" {
			currency, err = sales.ResolveCurrency("OZON", "RU")
			if err != nil {
				return c, err
			}
		}
		c.Products = append(c.Products, ProductCandidate{sku, p.OfferID, p.Name, q, amount, currency, payloadID})
	}
	sort.Slice(c.Products, func(i, j int) bool {
		if c.Products[i].SourceSKU == c.Products[j].SourceSKU {
			return c.Products[i].OfferID < c.Products[j].OfferID
		}
		return c.Products[i].SourceSKU < c.Products[j].SourceSKU
	})
	productParts := make([]string, len(c.Products))
	for i, p := range c.Products {
		productParts[i] = fmt.Sprintf("%d|%s|%d|%s|%s", p.SourceSKU, p.OfferID, p.Quantity, p.UnitPrice.String(), p.Currency)
	}
	c.StateHash = hash(fmt.Sprint(c.OrderID), c.OrderNumber, c.ParentPostingNumber, c.Status, c.Substatus, c.Scheme, timeText(c.ProviderCreatedAt), timeText(c.InProcessAt), timeText(c.DeliveringDate), timeText(c.ShipmentDate), fmt.Sprint(c.CancelReasonID), c.CancelReason, c.CancellationType, c.CancellationInitiator, fmt.Sprint(c.CancelledAfterShip), fmt.Sprint(c.WarehouseID), fmt.Sprint(c.ProviderID), strings.Join(productParts, ";"))
	return c, nil
}
func normalizeReturn(raw ozon.CanonicalReturn, payloadID int64, received time.Time) (ReturnCandidate, error) {
	c := ReturnCandidate{ProviderReturnID: int64(raw.ID), SourceID: int64(raw.SourceID), SourceOrderID: int64(raw.OrderID), SourcePostingNumber: strings.TrimSpace(raw.PostingNumber), SourceSKU: int64(raw.SKU), ProviderType: raw.Type, ProviderSchema: raw.Schema, Reason: raw.ReturnReasonName, Quantity: 0, Currency: strings.ToUpper(raw.CurrencyCode), ProviderReturnDate: parseTime(raw.Logistic.ReturnDate), VisualStatus: raw.Visual.Status, VisualStatusSysName: raw.Visual.StatusSysName, VisualChangeAt: parseTime(raw.Visual.ChangeMoment), TechnicalReturnMoment: parseTime(raw.Logistic.TechnicalReturnMoment), FinalMoment: parseTime(raw.Logistic.FinalMoment), CancelledWithCompensationMoment: parseTime(raw.Logistic.CancelledWithCompensationMoment), CompensationStatus: raw.CompensationStatus, PayloadID: payloadID, SourceReceivedAt: received}
	if c.ProviderSchema == "" {
		c.ProviderSchema = raw.ReturnSchema
	}
	if c.ProviderReturnID <= 0 {
		return c, fmt.Errorf("%w: return id missing", ErrSourceContractMismatch)
	}
	q, err := sales.ParseUnits(json.Number(strings.TrimSpace(string(raw.Quantity))))
	if err != nil {
		return c, fmt.Errorf("%w: return quantity: %v", ErrInvalidPostingQuantity, err)
	}
	c.Quantity = q
	price, err := sales.ParseAmount(json.Number(strings.TrimSpace(string(raw.Price))))
	if err != nil {
		return c, fmt.Errorf("%w: return price: %v", ErrSourceContractMismatch, err)
	}
	c.UnitPrice = price
	if c.Currency == "" {
		c.Currency, err = sales.ResolveCurrency("OZON", "RU")
		if err != nil {
			return c, err
		}
	}
	c.StateHash = hash(fmt.Sprint(c.SourceID), fmt.Sprint(c.SourceOrderID), c.SourcePostingNumber, fmt.Sprint(c.SourceSKU), c.ProviderType, c.ProviderSchema, c.Reason, fmt.Sprint(c.Quantity), c.UnitPrice.String(), c.Currency, timeText(c.ProviderReturnDate), c.VisualStatus, c.VisualStatusSysName, timeText(c.VisualChangeAt), timeText(c.TechnicalReturnMoment), timeText(c.FinalMoment), timeText(c.CancelledWithCompensationMoment), c.CompensationStatus)
	return c, nil
}
func timeText(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.UTC().Format(time.RFC3339Nano)
}
