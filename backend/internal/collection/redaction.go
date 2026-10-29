package collection

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type PayloadRedactor interface {
	Profile() string
	Redact(context.Context, string, []byte) ([]byte, error)
}

type JSONPayloadRedactor struct {
	profile string
	denied  map[string]struct{}
}

func NewJSONPayloadRedactor(profile string, deniedKeys ...string) *JSONPayloadRedactor {
	denied := make(map[string]struct{}, len(deniedKeys))
	for _, key := range deniedKeys {
		denied[strings.ToLower(strings.TrimSpace(key))] = struct{}{}
	}
	return &JSONPayloadRedactor{profile: profile, denied: denied}
}

func (r *JSONPayloadRedactor) Profile() string { return r.profile }

func (r *JSONPayloadRedactor) Redact(_ context.Context, _ string, body []byte) ([]byte, error) {
	if len(body) == 0 {
		return body, nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("%s expects JSON payload: %w", r.profile, err)
	}
	redactJSONValue(value, r.denied)
	redacted, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal redacted payload: %w", err)
	}
	return redacted, nil
}

func redactJSONValue(value any, denied map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, sensitive := denied[strings.ToLower(key)]; sensitive {
				delete(typed, key)
				continue
			}
			redactJSONValue(child, denied)
		}
	case []any:
		for _, child := range typed {
			redactJSONValue(child, denied)
		}
	}
}

func defaultRedactors() map[CollectionDomain]PayloadRedactor {
	// Catalog and taxonomy payloads contain business metadata, but the shared
	// boundary still strips security material structurally before object write.
	redactor := NewJSONPayloadRedactor(
		"OZON_CATALOG_REDACTION_V1",
		"authorization", "access_token", "refresh_token", "oauth_token",
		"api_key", "client_secret", "cookie", "set-cookie",
	)
	return map[CollectionDomain]PayloadRedactor{
		DomainCatalog:  redactor,
		DomainTaxonomy: redactor,
		DomainSalesDemand: NewJSONPayloadRedactor(
			"OZON_SALES_DEMAND_REDACTION_V1",
			"authorization", "access_token", "refresh_token", "oauth_token",
			"api_key", "client_secret", "cookie", "set-cookie",
		),
		DomainStock: NewJSONPayloadRedactor(
			"OZON_STOCK_REDACTION_V1",
			"authorization", "access_token", "refresh_token", "oauth_token", "api_key", "client_secret", "cookie", "set-cookie",
		),
		DomainPrice: NewJSONPayloadRedactor(
			"OZON_PRICE_REDACTION_V1",
			"authorization", "access_token", "refresh_token", "oauth_token", "api_key", "client_secret", "cookie", "set-cookie",
		),
		DomainAdvertising: NewJSONPayloadRedactor(
			"OZON_ADVERTISING_REDACTION_V1",
			"authorization", "access_token", "refresh_token", "oauth_token", "api_key", "client_secret", "cookie", "set-cookie",
		),
		DomainFunnel: NewJSONPayloadRedactor(
			"OZON_FUNNEL_REDACTION_V1",
			"authorization", "access_token", "refresh_token", "oauth_token", "api_key", "client_secret", "cookie", "set-cookie",
		),
		DomainFinance: NewJSONPayloadRedactor(
			"OZON_FINANCE_REDACTION_V1",
			"authorization", "access_token", "refresh_token", "oauth_token", "api_key", "client_secret", "cookie", "set-cookie",
		),
		DomainSearch: NewJSONPayloadRedactor(
			"OZON_SEARCH_REDACTION_V1",
			"authorization", "access_token", "refresh_token", "oauth_token", "api_key", "client_secret", "cookie", "set-cookie",
		),
		DomainPromotions: NewJSONPayloadRedactor(
			"OZON_PROMOTIONS_REDACTION_V1",
			"authorization", "access_token", "refresh_token", "oauth_token", "api_key", "client_secret", "cookie", "set-cookie",
		),
		DomainAccountHealth: NewJSONPayloadRedactor(
			"OZON_ACCOUNT_HEALTH_REDACTION_V1",
			"authorization", "access_token", "refresh_token", "oauth_token", "api_key", "client_secret", "cookie", "set-cookie",
			"phone", "email", "address", "customer", "addressee",
		),
		DomainSupply: NewJSONPayloadRedactor(
			"OZON_SUPPLY_REDACTION_V1",
			"authorization", "access_token", "refresh_token", "oauth_token", "api_key", "client_secret", "cookie", "set-cookie",
			"phone", "email", "address", "contact", "legal_details",
		),
	}
}

func defaultEndpointRedactors() map[string]PayloadRedactor {
	denied := []string{
		"authorization", "access_token", "refresh_token", "oauth_token", "api_key", "api-key", "client_secret", "cookie", "set-cookie",
		"customer", "addressee", "phone", "email", "address", "legal_details", "tracking_number", "barcodes", "barcode", "imei", "digital_codes", "digital_code", "activation_code", "exemplars",
	}
	return map[string]PayloadRedactor{
		"ozon.postings.fbo.list": NewJSONPayloadRedactor("OZON_POSTINGS_FBO_REDACTION_V1", denied...),
		"ozon.postings.fbo.get":  NewJSONPayloadRedactor("OZON_POSTINGS_FBO_REDACTION_V1", denied...),
		"ozon.postings.fbs.list": NewJSONPayloadRedactor("OZON_POSTINGS_FBS_REDACTION_V1", denied...),
		"ozon.postings.fbs.get":  NewJSONPayloadRedactor("OZON_POSTINGS_FBS_REDACTION_V1", denied...),
		"ozon.returns.list":      NewJSONPayloadRedactor("OZON_RETURNS_REDACTION_V1", denied...),
	}
}
