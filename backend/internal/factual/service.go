package factual

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool              *pgxpool.Pool
	signer            factualapi.TokenSigner
	maxPage, maxBatch int
}

func NewService(pool *pgxpool.Pool, signer factualapi.TokenSigner, maxPage, maxBatch int) *Service {
	if maxPage <= 0 {
		maxPage = 200
	}
	if maxBatch <= 0 {
		maxBatch = 1000
	}
	return &Service{pool: pool, signer: signer, maxPage: maxPage, maxBatch: maxBatch}
}
func (s *Service) validate(c factualapi.FactualReadContext) error {
	if s == nil || s.pool == nil || c.SellerAccountID() <= 0 || c.ContextHash() == "" {
		return factualapi.NewError(factualapi.CodeContextInvalid, "valid factual context and repository are required")
	}
	return nil
}
func notFound(kind string) error {
	return factualapi.NewError(factualapi.CodeNotFound, kind+" not found")
}
func dbError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return factualapi.NewError(factualapi.CodeInternalInvariantViolation, "canonical read failed")
}
func ptrTime(v time.Time, valid bool) *time.Time {
	if !valid {
		return nil
	}
	return &v
}
func pageSize(req factualapi.PageRequest, max int) (int, error) {
	n := req.PageSize
	if n == 0 {
		n = 100
	}
	if n < 1 || n > max {
		return 0, factualapi.NewError(factualapi.CodeInvalidArgument, "invalid page size")
	}
	return n, nil
}
func filterHash(v any) string { b, _ := json.Marshal(v); return factualapi.StableHash(string(b)) }
func (s *Service) cursor(c factualapi.FactualReadContext, method string, filter any, req factualapi.PageRequest) (int64, string, error) {
	if req.PageToken == "" {
		return 0, filterHash(filter), nil
	}
	h := filterHash(filter)
	p, e := s.signer.ParsePageToken(req.PageToken, c, method, h)
	return p.LastID, h, e
}
func (s *Service) next(c factualapi.FactualReadContext, method, h string, last int64, has bool) (string, error) {
	if !has {
		return "", nil
	}
	return s.signer.PageToken(factualapi.PageCursor{API: c.PolicyVersions().API, ContextHash: c.ContextHash(), UniverseID: c.UniverseID(), Method: method, FilterHash: h, Ordering: "id_asc_v1", LastID: last})
}
func readiness(c factualapi.FactualReadContext, key string) (string, []factualapi.Limitation) {
	if b, ok := c.CapabilityBinding(key, "HISTORICAL_QUERY"); ok {
		if b.FinalState != "READY" && b.ReasonCode != "" {
			return b.FinalState, []factualapi.Limitation{{Code: b.ReasonCode, Message: "frozen capability assessment limits this fact"}}
		}
		return b.FinalState, nil
	}
	return "UNAVAILABLE", []factualapi.Limitation{{Code: "CAPABILITY_NOT_BOUND", Message: "capability is not present in the frozen context"}}
}
func (s *Service) meta(c factualapi.FactualReadContext, key, kind string, entity int64, p *factualapi.Period, resultHash string) (factualapi.FactMeta, error) {
	return s.metaWithAux(c, key, kind, entity, 0, p, resultHash)
}
func (s *Service) metaWithAux(c factualapi.FactualReadContext, key, kind string, entity, auxiliary int64, p *factualapi.Period, resultHash string) (factualapi.FactMeta, error) {
	r, l := readiness(c, key)
	ref, e := s.signer.FactRef(c, factualapi.TraceInput{Kind: kind, EntityID: entity, AuxiliaryID: auxiliary, Period: p, ResultHash: resultHash})
	m := factualapi.FactMeta{Readiness: r, Period: p, Limitations: l, FactRef: ref}
	if p == nil {
		m.AsOfAt = timePtr(c.CutoffAt())
	}
	// Coverage must use exact (domain, factFamily) — never ambiguous DomainBinding.
	if family, ok := factualapi.FactFamilyForCapability(key); ok {
		if b, ok := c.Binding(family.Domain, family.FactFamily); ok {
			m.CoverageFrom, m.CoverageTo = b.CoverageFrom, b.CoverageTo
		}
	}
	return m, e
}

// requireFactFamilyBinding returns the exact DomainBinding for (domain, factFamily),
// or CodeContextInvalid when the sealed/run-bound snapshot binding is missing.
func requireFactFamilyBinding(c factualapi.FactualReadContext, domain, factFamily string) (factualapi.DomainBinding, error) {
	b, ok := c.Binding(domain, factFamily)
	if !ok {
		return factualapi.DomainBinding{}, factualapi.NewError(factualapi.CodeContextInvalid, "fact-family binding not found: "+domain+"/"+factFamily)
	}
	return b, nil
}
func timePtr(v time.Time) *time.Time { return &v }
func factHash(v any) string          { b, _ := json.Marshal(v); return factualapi.StableHash(string(b)) }

func (s *Service) GetAccount(ctx context.Context, c factualapi.FactualReadContext) (factualapi.Account, error) {
	if e := s.validate(c); e != nil {
		return factualapi.Account{}, e
	}
	var a factualapi.Account
	e := s.pool.QueryRow(ctx, `SELECT id FROM seller_accounts WHERE id=$1`, c.SellerAccountID()).Scan(&a.SellerAccountID)
	if errors.Is(e, pgx.ErrNoRows) {
		return a, notFound("account")
	}
	if e != nil {
		return a, dbError(e)
	}
	a.Meta, e = s.meta(c, "CATALOG.SKU_UNIVERSE", "ACCOUNT", a.SellerAccountID, nil, factHash(a))
	return a, e
}
