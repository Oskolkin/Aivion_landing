package collection

import (
	"context"
	"strings"
	"testing"
)

func TestJSONPayloadRedactorPreservesExactJSONNumbersForReplay(t *testing.T) {
	redactor := NewJSONPayloadRedactor("test", "token")
	input := []byte(`{"amount":9007199254740993.123456,"token":"remove-me"}`)
	out, err := redactor.Redact(context.Background(), "analytics", input)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, `9007199254740993.123456`) || strings.Contains(text, "remove-me") {
		t.Fatalf("redacted payload=%s", text)
	}
}

func TestLifecycleEndpointRedactorsRemovePIIAndSecretsBeforePersistence(t *testing.T) {
	input := []byte(`{"customer":{"name":"Alice"},"addressee":"Bob","phone":"+79990000000","email":"buyer@example.test","address":"Secret street","imei":"123","digital_codes":["code"],"barcodes":["barcode"],"Authorization":"Bearer secret","Api-Key":"secret","access_token":"secret","products":[{"name":"Safe product","sku":1}]}`)
	for endpoint, profile := range map[string]string{"ozon.postings.fbo.list": "OZON_POSTINGS_FBO_REDACTION_V1", "ozon.postings.fbs.list": "OZON_POSTINGS_FBS_REDACTION_V1", "ozon.returns.list": "OZON_RETURNS_REDACTION_V1"} {
		redactor := defaultEndpointRedactors()[endpoint]
		out, err := redactor.Redact(context.Background(), endpoint, input)
		if err != nil {
			t.Fatal(err)
		}
		if redactor.Profile() != profile {
			t.Fatalf("profile=%s", redactor.Profile())
		}
		text := string(out)
		for _, forbidden := range []string{"Alice", "Bob", "+79990000000", "buyer@example.test", "Secret street", "123", "code", "barcode", "Bearer secret"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s leaked in %s", forbidden, text)
			}
		}
		if !strings.Contains(text, "Safe product") {
			t.Fatalf("non-PII product name removed: %s", text)
		}
	}
}

func TestPrompt6DomainsHaveDedicatedSecretRedactionProfiles(t *testing.T) {
	profiles := map[CollectionDomain]string{
		DomainStock: "OZON_STOCK_REDACTION_V1", DomainPrice: "OZON_PRICE_REDACTION_V1", DomainAdvertising: "OZON_ADVERTISING_REDACTION_V1",
		DomainFinance:       "OZON_FINANCE_REDACTION_V1",
		DomainSearch:        "OZON_SEARCH_REDACTION_V1",
		DomainPromotions:    "OZON_PROMOTIONS_REDACTION_V1",
		DomainAccountHealth: "OZON_ACCOUNT_HEALTH_REDACTION_V1",
	}
	for domain, profile := range profiles {
		redactor := defaultRedactors()[domain]
		if redactor == nil || redactor.Profile() != profile {
			t.Fatalf("%s redactor=%v", domain, redactor)
		}
		out, err := redactor.Redact(context.Background(), "test", []byte(`{"access_token":"secret","value":1}`))
		if err != nil || strings.Contains(string(out), "secret") || !strings.Contains(string(out), `"value":1`) {
			t.Fatalf("%s redaction=%s err=%v", domain, out, err)
		}
	}
}

func TestSupplyRedactionRemovesSecretsAndContactData(t *testing.T) {
	redactor := defaultRedactors()[DomainSupply]
	if redactor == nil || redactor.Profile() != "OZON_SUPPLY_REDACTION_V1" {
		t.Fatalf("supply redactor=%v", redactor)
	}
	out, err := redactor.Redact(context.Background(), "ozon.supply.order.details", []byte(`{"authorization":"secret","phone":"+7000","address":"private","supply_id":"s1"}`))
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if strings.Contains(text, "secret") || strings.Contains(text, "+7000") || strings.Contains(text, "private") || !strings.Contains(text, "s1") {
		t.Fatalf("supply redaction=%s", text)
	}
}
