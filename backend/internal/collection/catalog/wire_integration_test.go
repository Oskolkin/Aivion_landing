package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

func TestProductionProductListWireDecodesIntoCanonicalNormalization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"result":{"items":[{"product_id":101,"offer_id":"OFFER-A","sku":10001,"archived":false}],"last_id":""}}`))
	}))
	defer server.Close()

	client := ozon.NewClientWithConfig(server.Client(), server.URL, ozon.RetryConfig{MaxAttempts: 1})
	response, err := client.ListProducts(context.Background(), "client", "secret", NewFullSnapshotRequest("ALL"))
	if err != nil {
		t.Fatal(err)
	}
	observation, err := Normalize(response.Data.Items[0], ozon.ProductInfoListItem{ID: 101, SKU: 10001, OfferID: "OFFER-A"})
	if err != nil {
		t.Fatal(err)
	}
	if observation.ProductID != 101 || observation.OzonSKU != 10001 || observation.OfferID != "OFFER-A" || observation.State != "ACTIVE" {
		t.Fatalf("canonical observation=%+v", observation)
	}
}
