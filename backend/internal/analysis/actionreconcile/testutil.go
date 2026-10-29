package actionreconcile

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"

func money(v float64) actioncatalog.Money {
	return actioncatalog.Money{Amount: v, Currency: "RUB"}
}

func paramsPrice(m actioncatalog.Money) actioncatalog.Parameters {
	p := m
	return actioncatalog.Parameters{RecommendedPrice: &p}
}
