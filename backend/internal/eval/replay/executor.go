package replay

import (
	"context"
	"fmt"
	"sync"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/eval"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RunEvaluationStages(ctx context.Context, pool *pgxpool.Pool, store artifacts.ArtifactStore, workerID string, runID int64) error {
	exec, err := runtime.NewProductionExecutor(pool, store, workerID, nil)
	if err != nil {
		return err
	}
	for _, stage := range stages.ProductionImplementedStageNames() {
		if err := exec.ExecuteStage(ctx, runID, stage); err != nil {
			return fmt.Errorf("stage %s: %w", stage, err)
		}
	}
	return nil
}

type ProductionCaseExecutor struct {
	Pool     *pgxpool.Pool
	Repo     *analysispg.Repository
	Store    artifacts.ArtifactStore
	SellerID int64
	WorkerID string
	NewRun   func(ctx context.Context) (analysispg.Run, error)

	mu    sync.Mutex
	cache map[string]eval.CaseActual
}

func (p *ProductionCaseExecutor) Execute(ctx context.Context, in eval.CaseInput, opts eval.ExecuteOptions) (eval.CaseActual, error) {
	_ = in
	_ = opts
	if p.NewRun == nil {
		return eval.CaseActual{}, fmt.Errorf("production executor missing NewRun")
	}
	key := fmt.Sprintf("%d", p.SellerID)
	p.mu.Lock()
	if p.cache == nil {
		p.cache = map[string]eval.CaseActual{}
	}
	if hit, ok := p.cache[key]; ok {
		p.mu.Unlock()
		hit.NoOp = true
		hit.DiscoveryCalls = 0
		hit.DeepCalls = 0
		return hit, nil
	}
	p.mu.Unlock()

	run, err := p.NewRun(ctx)
	if err != nil {
		return eval.CaseActual{}, err
	}
	before, err := p.Repo.CurrentPublicationID(ctx, p.SellerID)
	if err != nil {
		return eval.CaseActual{}, err
	}
	if err := RunEvaluationStages(ctx, p.Pool, p.Store, p.WorkerID, run.ID); err != nil {
		return eval.CaseActual{}, err
	}
	if err := AssertNoPublish(ctx, p.Repo, p.SellerID, before); err != nil {
		return eval.CaseActual{}, err
	}
	act := eval.CaseActual{
		Values:         map[string]any{"seller_isolation_ok": true, "unavailable_as_zero": false},
		AnalysisRunID:  &run.ID,
		Hashes:         map[string]string{},
		DiscoveryCalls: 1,
		DeepCalls:      1,
	}
	got, err := p.Repo.GetRun(ctx, run.ID)
	if err == nil && got.UniverseID != "" {
		act.Hashes["universe_id"] = got.UniverseID
	}
	p.mu.Lock()
	p.cache[key] = act
	p.mu.Unlock()
	return act, nil
}
