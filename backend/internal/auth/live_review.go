package auth

import (
	"errors"
	"net"
	"net/http"
	"strings"
)

const (
	HeaderLiveReview      = "X-Aivion-Live-Review"
	HeaderLiveReviewToken = "X-Aivion-Live-Review-Token"
	HeaderLiveReviewHost  = "X-Aivion-Live-Review-Host"
)

var (
	ErrLiveReviewDisabled  = errors.New("live review disabled")
	ErrLiveReviewForbidden = errors.New("live review forbidden")
	ErrLiveReviewHost      = errors.New("live review host invalid")
)

// LiveReviewConfig configures header-based stateless live review authentication.
type LiveReviewConfig struct {
	Enabled                bool
	AllowedHostSuffix      string
	AllowedSellerAccountID int64
	// IsAllowedSeller reports whether seller_account_id is on the staging demo allow-list.
	IsAllowedSeller func(sellerID int64) bool
	// OnEvent receives safe observability events (no tokens/secrets).
	OnEvent func(event LiveReviewEvent)
}

// LiveReviewEvent is a safe diagnostic event for live review auth.
type LiveReviewEvent struct {
	Name            string
	InviteID        int64
	SellerAccountID int64
	Hostname        string
	Method          string
	Path            string
	Status          int
	Reason          string
}

// LiveReviewHeadersPresent reports whether the request carries the live-review marker.
func LiveReviewHeadersPresent(r *http.Request) bool {
	if r == nil {
		return false
	}
	return strings.TrimSpace(r.Header.Get(HeaderLiveReview)) == "1"
}

func liveReviewTokenFromRequest(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get(HeaderLiveReviewToken))
}

func liveReviewHostFromRequest(r *http.Request) string {
	host := strings.TrimSpace(r.Header.Get(HeaderLiveReviewHost))
	if host == "" {
		return ""
	}
	// Strip optional port if a misconfigured proxy included one.
	if h, _, err := net.SplitHostPort(host); err == nil {
		return strings.ToLower(h)
	}
	return strings.ToLower(host)
}

func hostMatchesLiveReviewSuffix(host, suffix string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	suffix = strings.ToLower(strings.TrimSpace(suffix))
	if host == "" || suffix == "" {
		return false
	}
	if !strings.HasPrefix(suffix, ".") {
		suffix = "." + suffix
	}
	// Reject exact apex equal to suffix without leading label (e.g. ".review.aivion.pro").
	if host == strings.TrimPrefix(suffix, ".") {
		return false
	}
	return strings.HasSuffix(host, suffix)
}

func setLiveReviewResponseHeaders(w http.ResponseWriter) {
	w.Header().Set(HeaderLiveReview, "true")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func emitLiveReviewEvent(cfg *LiveReviewConfig, event LiveReviewEvent) {
	if cfg == nil || cfg.OnEvent == nil {
		return
	}
	cfg.OnEvent(event)
}
