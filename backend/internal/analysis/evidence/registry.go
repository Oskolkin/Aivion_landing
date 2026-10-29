package evidence

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type Resolver interface {
	EvidenceType() string
	ResolveBatch(ctx context.Context, rc factualapi.FactualReadContext, requests []NormalizedEvidenceRequest) []MaterializationResult
}

type Registry struct {
	order []string
	by    map[string]Resolver
}

func NewRegistry() *Registry {
	return &Registry{by: map[string]Resolver{}}
}

func (r *Registry) Register(res Resolver) {
	k := res.EvidenceType()
	if _, ok := r.by[k]; ok {
		return
	}
	r.by[k] = res
	r.order = append(r.order, k)
}

func (r *Registry) Has(t string) bool { _, ok := r.by[t]; return ok }

func (r *Registry) Get(t string) (Resolver, bool) {
	res, ok := r.by[t]
	return res, ok
}

func (r *Registry) Types() []string {
	out := append([]string(nil), r.order...)
	return out
}

func ProductionRegistry(api Reads) *Registry {
	r := NewRegistry()
	registerSales(r, api)
	registerLifecycle(r, api)
	registerStock(r, api)
	registerPrice(r, api)
	registerAdvertising(r, api)
	registerFunnel(r, api)
	registerSearch(r, api)
	registerFinance(r, api)
	registerPromotions(r, api)
	registerHealth(r, api)
	registerSupply(r, api)
	return r
}
