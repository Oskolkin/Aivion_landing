package openaix

import (
	"errors"
	"net/http"
	"testing"
)

func TestWrapHTTPOutageMarksRetryable(t *testing.T) {
	err := WrapHTTPOutage(http.StatusServiceUnavailable, "boom")
	if !IsTemporarilyUnavailable(err) {
		t.Fatalf("expected temporarily unavailable")
	}
}

func TestWrapHTTPOutageClientErrorNotUnavailable(t *testing.T) {
	err := WrapHTTPOutage(http.StatusBadRequest, "bad")
	if IsTemporarilyUnavailable(err) {
		t.Fatalf("did not expect temporarily unavailable")
	}
	var h *HTTPOutageError
	if !errors.As(err, &h) || h.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected error: %v", err)
	}
}
