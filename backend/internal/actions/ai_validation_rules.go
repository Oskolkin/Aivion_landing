package actions

import "strings"

var genericTitles = []string{
	"проверьте остатки",
	"разберитесь с рекламой",
	"проверьте товары",
	"посмотрите воронку",
	"обратите внимание на sku",
	"проверьте воронку",
}

var genericNextSteps = []string{
	"проверьте данные",
	"разберитесь с проблемой",
	"посмотрите подробнее",
}

var forbiddenClaimPatterns = []string{
	"изменить цену",
	"снизить цену",
	"поднять цену",
	"маржинальность",
	"себестоимость",
	"прибыль",
	"гарантированно",
	"точно увеличит",
	"sku сливает бюджет",
	"товар сливает бюджет",
	"отключите sku",
}

var domainAllowedTypes = map[string][]ActionType{
	ActionDomainStock: {
		ActionTypeStockReplenishment,
		ActionTypeNoSalesWithStock,
	},
	ActionDomainSKUSales: {
		ActionTypeSKUOrdersDrop,
		ActionTypeKeySKURisk,
	},
	ActionDomainAdvertising: {
		ActionTypeAdHighDRR,
		ActionTypeAdSpendNoOrders,
	},
	ActionDomainFunnel: {
		ActionTypeFunnelWeakTransition,
	},
}

func validationRulesForDomain(domain string, forbiddenClaims []string) ValidationRules {
	return ValidationRules{
		Domain:              domain,
		AllowedTypes:        domainAllowedTypes[domain],
		ForbiddenClaims:     forbiddenClaims,
		RequiredLimitations: requiredLimitationMarkers(domain),
	}
}

func requiredLimitationMarkers(domain string) map[ActionType][]string {
	switch domain {
	case ActionDomainStock:
		return map[ActionType][]string{
			ActionTypeStockReplenishment: {"будущие поставки"},
			ActionTypeNoSalesWithStock:   {"подтверждённой строкой метрик"},
		}
	case ActionDomainAdvertising:
		return map[ActionType][]string{
			ActionTypeAdHighDRR:       {"campaign"},
			ActionTypeAdSpendNoOrders: {"campaign"},
		}
	case ActionDomainSKUSales:
		return map[ActionType][]string{
			ActionTypeKeySKURisk: {"маржинальность"},
		}
	default:
		return nil
	}
}

func isGenericTitle(title string) bool {
	t := strings.ToLower(strings.TrimSpace(title))
	for _, g := range genericTitles {
		if t == g || strings.HasPrefix(t, g) {
			return true
		}
	}
	return false
}

func isGenericNextStep(step string) bool {
	t := strings.ToLower(strings.TrimSpace(step))
	for _, g := range genericNextSteps {
		if t == g || strings.HasPrefix(t, g) {
			return true
		}
	}
	return false
}

func containsForbiddenClaim(text string, extraForbidden []string) (string, bool) {
	lower := strings.ToLower(text)
	for _, p := range forbiddenClaimPatterns {
		if strings.Contains(lower, p) {
			return p, true
		}
	}
	for _, claim := range extraForbidden {
		c := strings.ToLower(strings.TrimSpace(claim))
		if c == "" {
			continue
		}
		// Skip high-level guardrails that are not substring patterns.
		if strings.HasPrefix(c, "не ") && len(c) > 40 {
			continue
		}
		if strings.Contains(lower, strings.TrimPrefix(c, "не ")) {
			return claim, true
		}
	}
	return "", false
}

func isValidActionType(t ActionType) bool {
	return isLegacyCandidateActionType(t)
}

func isValidCategory(c ActionCategory) bool {
	switch c {
	case ActionCategoryStock, ActionCategorySKU, ActionCategoryAdvertising, ActionCategoryFunnel, ActionCategorySales:
		return true
	default:
		return false
	}
}

func isKnownActionType(t ActionType) bool {
	for _, known := range AllKnownActionTypes {
		if known == t {
			return true
		}
	}
	return false
}

func isLegacyCandidateActionType(t ActionType) bool {
	for _, allowed := range LegacyCandidateActionTypes {
		if allowed == t {
			return true
		}
	}
	return false
}

func isAIFirstActionType(t ActionType) bool {
	for _, allowed := range AIFirstAllowedActionTypes {
		if allowed == t {
			return true
		}
	}
	return false
}

func isKnownActionCategory(c ActionCategory) bool {
	for _, known := range AllKnownActionCategories {
		if known == c {
			return true
		}
	}
	return false
}

func isAIFirstCategory(c ActionCategory) bool {
	for _, allowed := range AIFirstAllowedCategories {
		if allowed == c {
			return true
		}
	}
	return false
}

func isValidPriority(p ActionPriority) bool {
	switch p {
	case ActionPriorityCritical, ActionPriorityImportant, ActionPriorityWatch:
		return true
	default:
		return false
	}
}

func typeAllowedInDomain(domain string, t ActionType) bool {
	for _, allowed := range domainAllowedTypes[domain] {
		if allowed == t {
			return true
		}
	}
	return false
}

func advertisingSKULevelClaim(text string) bool {
	lower := strings.ToLower(text)
	patterns := []string{
		"sku сливает",
		"товар сливает",
		"sku-level",
		"отключите sku",
		"sku в рекламе",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
