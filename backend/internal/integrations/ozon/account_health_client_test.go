package ozon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccountHealthTransportUsesBearerPostAndExactNumbers(t *testing.T) {
	paths := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer seller-bound-token" {
			t.Errorf("authorization=%q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Client-Id") != "" || r.Header.Get("Api-Key") != "" {
			t.Errorf("legacy auth headers leaked: %v", r.Header)
		}
		paths[r.URL.Path] = true
		_, _ = w.Write([]byte(`{"result":{"value":123456789012345.123456}}`))
	}))
	defer server.Close()
	transport := NewSellerBoundTransport(&fixedTokenProvider{}, NewClientWithConfig(server.Client(), server.URL, RetryConfig{MaxAttempts: 1}), nil)
	responses := make([]*TypedResponse[AccountHealthDocument], 0, 4)
	for _, call := range []func() (*TypedResponse[AccountHealthDocument], error){
		func() (*TypedResponse[AccountHealthDocument], error) {
			return transport.GetAccountRatingSummary(context.Background(), 7, AccountHealthRequest{})
		},
		func() (*TypedResponse[AccountHealthDocument], error) {
			return transport.GetAccountRatingHistory(context.Background(), 7, AccountHealthRequest{})
		},
		func() (*TypedResponse[AccountHealthDocument], error) {
			return transport.GetFBSRatingIndex(context.Background(), 7, AccountHealthRequest{})
		},
		func() (*TypedResponse[AccountHealthDocument], error) {
			return transport.ListFBSPostingErrors(context.Background(), 7, AccountHealthRequest{})
		},
	} {
		response, err := call()
		if err != nil {
			t.Fatal(err)
		}
		responses = append(responses, response)
	}
	for _, path := range []string{RatingSummaryPath, RatingHistoryPath, FBSIndexInfoPath, FBSPostingErrorListPath} {
		if !paths[path] {
			t.Errorf("endpoint not called: %s", path)
		}
	}
	value := responses[0].Data["result"].(map[string]any)["value"]
	number, ok := value.(json.Number)
	if !ok || number.String() != "123456789012345.123456" {
		t.Fatalf("numeric precision lost: %#v", value)
	}
}
