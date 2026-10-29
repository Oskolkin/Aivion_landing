package ozon

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type lifecycleToken struct{}

func (lifecycleToken) CanonicalAccessToken(context.Context, int64) (string, error) {
	return "canonical-lifecycle-token", nil
}

func TestCanonicalLifecycleTransportUsesCurrentEndpointsAndBearerOnly(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Method != "POST" {
			t.Errorf("method=%s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer canonical-lifecycle-token" || r.Header.Get("Client-Id") != "" || r.Header.Get("Api-Key") != "" {
			t.Errorf("headers=%v", r.Header)
		}
		_, _ = io.ReadAll(r.Body)
		switch r.URL.Path {
		case CanonicalFBOListPath, CanonicalFBSListPath:
			_, _ = w.Write([]byte(`{"result":{"postings":[],"has_next":false,"cursor":""}}`))
		case CanonicalFBODetailPath, CanonicalFBSDetailPath:
			_, _ = w.Write([]byte(`{"result":{"order_id":1,"order_number":"O","posting_number":"P"}}`))
		case CanonicalReturnsPath:
			_, _ = w.Write([]byte(`{"returns":[],"has_next":false,"last_id":0}`))
		default:
			t.Errorf("unexpected path=%s", r.URL.Path)
		}
	}))
	defer server.Close()
	transport := NewSellerBoundTransport(lifecycleToken{}, NewClientWithConfig(server.Client(), server.URL, RetryConfig{MaxAttempts: 1}), nil)
	ctx := context.Background()
	var list PostingListRequest
	var detail PostingDetailRequest
	detail.PostingNumber = "P"
	var returns ReturnsListRequest
	returns.Limit = 500
	if _, e := transport.ListCanonicalFBOPostings(ctx, 1, list); e != nil {
		t.Fatal(e)
	}
	if _, e := transport.GetCanonicalFBOPosting(ctx, 1, detail); e != nil {
		t.Fatal(e)
	}
	if _, e := transport.ListCanonicalFBSPostings(ctx, 1, list); e != nil {
		t.Fatal(e)
	}
	if _, e := transport.GetCanonicalFBSPosting(ctx, 1, detail); e != nil {
		t.Fatal(e)
	}
	if _, e := transport.ListCanonicalReturns(ctx, 1, returns); e != nil {
		t.Fatal(e)
	}
	want := []string{"/v3/posting/fbo/list", "/v2/posting/fbo/get", "/v4/posting/fbs/list", "/v3/posting/fbs/get", "/v1/returns/list"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths=%v", paths)
	}
}
