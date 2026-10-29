package ozon

import (
	"errors"
	"net/http"
	"strings"

	ozonperf "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon/performance"
)

// ConnectionError describes a user-facing validation failure for one API leg.
type ConnectionError struct {
	Source  string // seller_api | performance_api
	Code    string
	Message string
}

// IsCredentialOrAccessProblem returns true when the user must fix credentials or Ozon account access.
func IsCredentialOrAccessProblem(code string) bool {
	switch code {
	case "invalid_credentials", "forbidden", "offer_not_signed", "key_expired", "access_denied":
		return true
	default:
		return false
	}
}

func mapSellerCheckError(err error) ConnectionError {
	if err == nil {
		return ConnectionError{}
	}
	if errors.Is(err, ErrInvalidCredentials) {
		return ConnectionError{
			Source:  "seller_api",
			Code:    "invalid_credentials",
			Message: "Не удалось подключиться к Seller API. Проверьте Client ID и API Key.",
		}
	}
	if errors.Is(err, ErrNetworkError) {
		return ConnectionError{
			Source:  "seller_api",
			Code:    "temporary_unavailable",
			Message: "Ozon временно не отвечает. Попробуйте позже.",
		}
	}
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		body := strings.ToLower(providerErr.Body)
		if providerErr.StatusCode == http.StatusForbidden {
			if strings.Contains(body, "offer") || strings.Contains(body, "оферт") {
				return ConnectionError{
					Source:  "seller_api",
					Code:    "offer_not_signed",
					Message: "Доступ к Seller API ограничен. Проверьте, принята ли оферта Ozon в личном кабинете.",
				}
			}
			return ConnectionError{
				Source:  "seller_api",
				Code:    "forbidden",
				Message: "Доступ к Seller API ограничен. Проверьте права ключа в личном кабинете Ozon.",
			}
		}
		if providerErr.StatusCode == http.StatusTooManyRequests {
			return ConnectionError{
				Source:  "seller_api",
				Code:    "rate_limit",
				Message: "Превышен лимит запросов к Seller API. Попробуйте подключиться позже.",
			}
		}
		if providerErr.StatusCode >= 500 {
			return ConnectionError{
				Source:  "seller_api",
				Code:    "temporary_unavailable",
				Message: "Ozon временно не отвечает. Попробуйте позже.",
			}
		}
		if strings.Contains(body, "expired") || strings.Contains(body, "истёк") || strings.Contains(body, "истек") {
			return ConnectionError{
				Source:  "seller_api",
				Code:    "key_expired",
				Message: "Срок действия API-ключа Seller API истёк. Создайте новый ключ в личном кабинете Ozon и обновите данные подключения.",
			}
		}
	}
	return ConnectionError{
		Source:  "seller_api",
		Code:    "provider_error",
		Message: "Не удалось проверить Seller API. Попробуйте позже.",
	}
}

func mapPerformanceCheckError(err error) ConnectionError {
	if err == nil {
		return ConnectionError{}
	}
	if errors.Is(err, ozonperf.ErrInvalidCredentials) {
		return ConnectionError{
			Source:  "performance_api",
			Code:    "invalid_credentials",
			Message: "Не удалось подключиться к Performance API. Проверьте client_id и client_secret рекламного кабинета.",
		}
	}
	if errors.Is(err, ozonperf.ErrNetworkError) {
		return ConnectionError{
			Source:  "performance_api",
			Code:    "temporary_unavailable",
			Message: "Performance API временно недоступен. Попробуйте позже.",
		}
	}
	var providerErr *ozonperf.ProviderError
	if errors.As(err, &providerErr) {
		if providerErr.StatusCode == http.StatusForbidden {
			return ConnectionError{
				Source:  "performance_api",
				Code:    "access_denied",
				Message: "Доступ к Performance API ограничен. Проверьте права сервисного аккаунта в личном кабинете Ozon.",
			}
		}
		if providerErr.StatusCode == http.StatusTooManyRequests {
			return ConnectionError{
				Source:  "performance_api",
				Code:    "rate_limit",
				Message: "Превышен лимит запросов к Performance API. Попробуйте позже.",
			}
		}
		if providerErr.StatusCode >= 500 {
			return ConnectionError{
				Source:  "performance_api",
				Code:    "temporary_unavailable",
				Message: "Performance API временно недоступен. Попробуйте позже.",
			}
		}
	}
	return ConnectionError{
		Source:  "performance_api",
		Code:    "token_request_failed",
		Message: "Не удалось получить токен Performance API. Проверьте данные сервисного аккаунта.",
	}
}
