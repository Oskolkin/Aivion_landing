package sales

import "fmt"

const ProviderMarketCurrencyPolicyV1 = "provider_market_currency_policy_v1"

// ResolveCurrency is the explicit provider-market currency policy.
func ResolveCurrency(provider, market string) (string, error) {
	if provider == "OZON" && market == "RU" {
		return "RUB", nil
	}
	return "", fmt.Errorf("SALES_CURRENCY_UNRESOLVED: provider=%s market=%s", provider, market)
}
