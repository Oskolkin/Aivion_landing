package openaix

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
)

// ErrTemporarilyUnavailable indicates OpenAI or the network path is unavailable
// after retries (or a non-retryable outage). Callers may map this to user-safe messages.
var ErrTemporarilyUnavailable = errors.New("openai temporarily unavailable")

// WrapIfUnavailable wraps transport-level and typical HTTP outage errors.
func WrapIfUnavailable(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrTemporarilyUnavailable) {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrTemporarilyUnavailable, err)
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w: %w", ErrTemporarilyUnavailable, err)
	}
	return err
}

// HTTPOutageError carries a non-success OpenAI HTTP status.
type HTTPOutageError struct {
	StatusCode int
	Body       string
}

func (e *HTTPOutageError) Error() string {
	return fmt.Sprintf("openai http status=%d body=%s", e.StatusCode, e.Body)
}

// IsOutageStatus returns true for rate limits and server-side errors.
func IsOutageStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= http.StatusInternalServerError
}

// WrapHTTPOutage returns ErrTemporarilyUnavailable for outage-class HTTP codes.
func WrapHTTPOutage(status int, body string) error {
	if status >= 200 && status < 300 {
		return nil
	}
	if IsOutageStatus(status) {
		return fmt.Errorf("%w: %w", ErrTemporarilyUnavailable, &HTTPOutageError{StatusCode: status, Body: body})
	}
	return &HTTPOutageError{StatusCode: status, Body: body}
}

// IsTemporarilyUnavailable reports whether err should be surfaced as AI unavailable.
func IsTemporarilyUnavailable(err error) bool {
	return err != nil && errors.Is(err, ErrTemporarilyUnavailable)
}
