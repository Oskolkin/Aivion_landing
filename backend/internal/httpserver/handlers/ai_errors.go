package handlers

import (
	"errors"
	"net/http"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chat"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

// mapAIHandlerError writes a seller-facing HTTP status for known AI guard errors.
// Returns true when the error was handled.
func mapAIHandlerError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, aicost.ErrDailyLimitExceeded) {
		writeJSONError(w, http.StatusTooManyRequests, "AI usage limit exceeded for today; try again tomorrow")
		return true
	}
	if errors.Is(err, chat.ErrOpenAIRequestTooLarge) || errors.Is(err, recommendations.ErrOpenAIRequestTooLarge) {
		writeJSONError(w, http.StatusBadRequest, "request too large for AI context budget")
		return true
	}
	if errors.Is(err, chat.ErrAITemporarilyUnavailable) {
		writeJSONError(w, http.StatusServiceUnavailable, "AI temporarily unavailable, try again later")
		return true
	}
	return false
}
