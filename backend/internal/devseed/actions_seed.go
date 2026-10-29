package devseed

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ActionsSeedScenario string

const (
	ActionsSeedScenarioAvailable ActionsSeedScenario = "available"
	ActionsSeedScenarioChecked   ActionsSeedScenario = "checked"
	ActionsSeedScenarioEmpty     ActionsSeedScenario = "empty"
	ActionsSeedScenarioMixed     ActionsSeedScenario = "mixed"
)

type SeedActionsOptions struct {
	SellerAccountID int64
	MetricDate      *time.Time
	Scenario        ActionsSeedScenario
	DryRun          bool
	Force           bool
	AIEnabled       *bool
	// SetAsCurrent promotes the seeded action generation run as the seller's
	// legacy current AI snapshot (ai_pipeline_runs + seller_current_ai_snapshots).
	// Required for E2E after Unified fixtures: published unified actions are
	// protected from DeleteStale, so empty/available seeds must repoint the
	// legacy read path onto a fresh pipeline run.
	SetAsCurrent bool
}

type SeedActionsResult struct {
	SellerID              int64  `json:"seller_id"`
	MetricDate            string `json:"metric_date"`
	Scenario              string `json:"scenario"`
	ActionsUpserted       int    `json:"actions_upserted"`
	GenerationRunID       int64  `json:"generation_run_id"`
	CategoryRunsUpserted  int    `json:"category_runs_upserted"`
	EntityGroupsUpserted  int    `json:"entity_groups_upserted"`
	CheckedStatesUpserted int    `json:"checked_states_upserted"`
}

func SeedActions(ctx context.Context, pool *pgxpool.Pool, opts SeedActionsOptions) (*SeedActionsResult, error) {
	if err := AssertDevSeedAllowed(); err != nil {
		return nil, err
	}
	if opts.SellerAccountID <= 0 {
		return nil, fmt.Errorf("seller-id must be > 0")
	}
	metricDate := time.Now().UTC()
	if opts.MetricDate != nil {
		metricDate = opts.MetricDate.UTC()
	} else if d, ok := lookupSellerFacingMetricDate(ctx, pool, opts.SellerAccountID); ok {
		// Align seed with the metric_date Actions API will resolve for this
		// seller (legacy snapshot or unified current publication). Seeding
		// "today" while the pointer sits on a fixture date leaves the UI empty
		// or stuck on prior unified actions after mode=legacy.
		metricDate = d
	}
	metricDate = time.Date(metricDate.Year(), metricDate.Month(), metricDate.Day(), 0, 0, 0, 0, time.UTC)
	if opts.Scenario == "" {
		opts.Scenario = ActionsSeedScenarioAvailable
	}
	generateOpts := actions.GenerateOptionsFromEnv()
	if opts.AIEnabled != nil {
		generateOpts.AIEnabled = *opts.AIEnabled
	}
	q := dbgen.New(pool)
	run, err := q.CreateActionGenerationRun(ctx, dbgen.CreateActionGenerationRunParams{
		SellerAccountID: opts.SellerAccountID,
		MetricDate:      pgtype.Date{Time: metricDate, Valid: true},
		Status:          "completed",
	})
	if err != nil {
		return nil, err
	}
	notes, _ := json.Marshal([]string{
		"Seeded from devtools",
		fmt.Sprintf("ai_enabled=%t", generateOpts.AIEnabled),
		fmt.Sprintf("ai_repair_enabled=%t", generateOpts.EnableAIRepair),
		fmt.Sprintf("legacy_global_check_enabled=%t", generateOpts.EnableLegacyGlobalCheck),
	})
	_, _ = q.UpdateActionGenerationRun(ctx, dbgen.UpdateActionGenerationRunParams{
		ID:                   run.ID,
		Status:               "completed",
		ContextHash:          pgtype.Text{String: "dev-seed-actions", Valid: true},
		FinalStatus:          pgtype.Text{String: finalStatusForScenario(opts.Scenario), Valid: true},
		AcceptedCount:        0,
		RepairedCount:        0,
		DroppedCount:         0,
		FallbackCount:        0,
		AiFocusJson:          []byte(`{"status":"seeded","source":"dev_seed"}`),
		TopActionKeysJson:    []byte(`[]`),
		DataQualityNotesJson: notes,
	})
	seededActions := scenarioActions(metricDate, opts.Scenario)
	if opts.DryRun {
		return &SeedActionsResult{
			SellerID:   opts.SellerAccountID,
			MetricDate: metricDate.Format("2006-01-02"),
			Scenario:   string(opts.Scenario),
		}, nil
	}
	if opts.Scenario == ActionsSeedScenarioEmpty && !opts.SetAsCurrent {
		// When SetAsCurrent is true we retarget the published pipeline onto a
		// fresh empty generation run; published unified action_items are FK-
		// protected by unified_publication_action_map and must not be deleted.
		if err := q.DeleteStaleActionItemsForDate(ctx, dbgen.DeleteStaleActionItemsForDateParams{
			SellerAccountID: opts.SellerAccountID,
			MetricDate:      pgtype.Date{Time: metricDate, Valid: true},
			Column3:         []string{},
		}); err != nil {
			return nil, err
		}
	}
	entityGroupsUpserted := 0
	actionsUpserted := 0
	checkedUpserted := 0
	for _, item := range seededActions {
		if item.EntityGroup != nil {
			if err := upsertSeedEntityGroup(ctx, q, opts.SellerAccountID, run.ID, metricDate, item.EntityGroup); err != nil {
				return nil, err
			}
			entityGroupsUpserted++
		}
		if err := upsertSeedAction(ctx, q, opts.SellerAccountID, run.ID, metricDate, item); err != nil {
			return nil, err
		}
		actionsUpserted++
		if item.Checked {
			checkedUpserted++
			_ = q.UpsertActionUserState(ctx, dbgen.UpsertActionUserStateParams{
				SellerAccountID: opts.SellerAccountID,
				ActionKey:       item.ActionKey,
				Status:          string(actions.ActionStatusChecked),
				CheckedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			})
		}
	}
	if opts.SetAsCurrent {
		if err := promoteSeedAsCurrentLegacySnapshot(ctx, pool, opts.SellerAccountID, run.ID, metricDate); err != nil {
			return nil, err
		}
	}
	return &SeedActionsResult{
		SellerID:              opts.SellerAccountID,
		MetricDate:            metricDate.Format("2006-01-02"),
		Scenario:              string(opts.Scenario),
		ActionsUpserted:       actionsUpserted,
		GenerationRunID:       run.ID,
		CategoryRunsUpserted:  0,
		EntityGroupsUpserted:  entityGroupsUpserted,
		CheckedStatesUpserted: checkedUpserted,
	}, nil
}

func promoteSeedAsCurrentLegacySnapshot(ctx context.Context, pool *pgxpool.Pool, sellerAccountID, actionsRunID int64, metricDate time.Time) error {
	var pipelineID int64
	err := pool.QueryRow(ctx, `
SELECT pipeline_run_id
FROM seller_current_ai_snapshots
WHERE seller_account_id = $1`, sellerAccountID).Scan(&pipelineID)
	if err != nil {
		return fmt.Errorf("no seller_current_ai_snapshots row to retarget (run fake AI pipeline / unified fixture first): %w", err)
	}

	var pipelineMetric pgtype.Date
	var publishedAt pgtype.Timestamptz
	if err := pool.QueryRow(ctx, `
SELECT metric_date, published_at
FROM ai_pipeline_runs
WHERE id = $1 AND seller_account_id = $2`, pipelineID, sellerAccountID).Scan(&pipelineMetric, &publishedAt); err != nil {
		return fmt.Errorf("load current pipeline run %d: %w", pipelineID, err)
	}
	if !publishedAt.Valid {
		return fmt.Errorf("current pipeline run %d is not published; cannot retarget actions", pipelineID)
	}
	if pipelineMetric.Valid {
		metricDate = time.Date(pipelineMetric.Time.UTC().Year(), pipelineMetric.Time.UTC().Month(), pipelineMetric.Time.UTC().Day(), 0, 0, 0, 0, time.UTC)
	}

	// Align generation-run metric date with the published snapshot contract
	// (pipeline/dashboard/actions metric dates must match). Clear the previous
	// actions run's pipeline link first (unique ai_pipeline_run_id).
	if _, err := pool.Exec(ctx, `
UPDATE action_generation_runs
SET ai_pipeline_run_id = NULL,
    updated_at = now()
WHERE ai_pipeline_run_id = $1
  AND seller_account_id = $2
  AND id <> $3`, pipelineID, sellerAccountID, actionsRunID); err != nil {
		return fmt.Errorf("unlink previous action generation run: %w", err)
	}

	if _, err := pool.Exec(ctx, `
UPDATE action_generation_runs agr
SET ai_pipeline_run_id = p.id,
    metric_date = p.metric_date,
    data_pack_hash = p.data_pack_hash,
    global_context_hash = p.global_context_hash,
    updated_at = now()
FROM ai_pipeline_runs p
WHERE agr.id = $1
  AND agr.seller_account_id = $2
  AND p.id = $3
  AND p.seller_account_id = $2`, actionsRunID, sellerAccountID, pipelineID); err != nil {
		return fmt.Errorf("link action generation run to pipeline: %w", err)
	}

	if _, err := pool.Exec(ctx, `
UPDATE ai_pipeline_runs
SET actions_run_id = $1,
    actions_status = 'completed',
    updated_at = now()
WHERE id = $2
  AND seller_account_id = $3`, actionsRunID, pipelineID, sellerAccountID); err != nil {
		return fmt.Errorf("retarget pipeline actions_run_id: %w", err)
	}

	// Keep snapshot pointer on the same published pipeline; only the actions
	// generation run behind it changed.
	return nil
}

// lookupSellerFacingMetricDate returns the metric date the Actions API would
// use for this seller: prefer the legacy current pipeline run date, else the
// current unified publication date.
func lookupSellerFacingMetricDate(ctx context.Context, pool *pgxpool.Pool, sellerAccountID int64) (time.Time, bool) {
	var d pgtype.Date
	err := pool.QueryRow(ctx, `
SELECT r.metric_date
FROM seller_current_ai_snapshots s
JOIN ai_pipeline_runs r ON r.id = s.pipeline_run_id
WHERE s.seller_account_id = $1`, sellerAccountID).Scan(&d)
	if err == nil && d.Valid {
		return d.Time.UTC(), true
	}
	err = pool.QueryRow(ctx, `
SELECT p.metric_date
FROM seller_current_unified_publications c
JOIN unified_analysis_publications p ON p.id = c.publication_id
WHERE c.seller_account_id = $1`, sellerAccountID).Scan(&d)
	if err == nil && d.Valid {
		return d.Time.UTC(), true
	}
	return time.Time{}, false
}

type seedEntityGroup struct {
	GroupKey   string
	EntityType string
	EntityIDs  []string
	Summary    map[string]any
}

type seedAction struct {
	ActionKey         string
	Type              actions.ActionType
	Category          actions.ActionCategory
	Priority          actions.ActionPriority
	Title             string
	Reason            string
	NextStep          string
	Checked           bool
	SourceCandidateID string
	EvidenceFactIDs   []string
	CTAKeys           []string
	AffectedEntities  actions.ActionAffectedEntities
	EntityGroupRef    string
	EntityGroup       *seedEntityGroup
	Evidence          []actions.ActionEvidence
	Limitations       []string
	Payload           map[string]any
}

func scenarioActions(metricDate time.Time, scenario ActionsSeedScenario) []seedAction {
	if scenario == ActionsSeedScenarioEmpty {
		return nil
	}
	date := metricDate.Format("2006-01-02")
	base := []seedAction{
		buildSeedStockReplenishment(date),
		buildSeedNoSalesWithStock(date),
		buildSeedSKUOrdersDrop(date),
		buildSeedKeySKURisk(date),
		buildSeedAdHighDRR(date),
		buildSeedAdSpendNoOrders(date),
		buildSeedFunnelWeakTransition(date),
	}
	switch scenario {
	case ActionsSeedScenarioChecked:
		base[0].Checked = true
		base[2].Checked = true
	case ActionsSeedScenarioMixed:
		base[1].Checked = true
		base[4].Checked = true
	}
	return base
}

func buildSeedStockReplenishment(date string) seedAction {
	candidateID := fmt.Sprintf("seed_stock_replenishment_%s", date)
	groupKey := candidateID
	fact1 := candidateID + "_fact_1"
	fact2 := candidateID + "_fact_2"
	return seedAction{
		ActionKey:         "seed-stock-replenishment",
		Type:              actions.ActionTypeStockReplenishment,
		Category:          actions.ActionCategoryStock,
		Priority:          actions.ActionPriorityCritical,
		Title:             "Пополнить 4 ключевых SKU с покрытием меньше 7 дней",
		Reason:            "Эти SKU формируют 18% выручки за 7 дней, но покрытия осталось на 0.8-6.7 дней.",
		NextStep:          "Добавьте SKU в ближайшую поставку, начиная с покрытия < 3 дней.",
		SourceCandidateID: candidateID,
		EvidenceFactIDs:   []string{fact1, fact2},
		CTAKeys: []string{
			actions.ActionCTAOpenSKUList,
			actions.ActionCTAExportStockReplenishment,
			actions.ActionCTAAskAI,
			actions.ActionCTAMarkChecked,
		},
		EntityGroupRef: groupKey,
		EntityGroup: &seedEntityGroup{
			GroupKey:   groupKey,
			EntityType: "sku",
			EntityIDs:  []string{"100001", "100002", "100003", "100004"},
			Summary: map[string]any{
				"entities": []map[string]any{
					{"id": "100001", "offer_id": "lamp-white-01", "name": "Светильник кухонный белый", "metric": "покрытие 0.8 дн."},
					{"id": "100002", "offer_id": "lamp-black-02", "name": "Светильник кухонный чёрный", "metric": "покрытие 2.1 дн."},
					{"id": "100003", "offer_id": "bulb-warm-03", "name": "Лампа тёплый свет", "metric": "покрытие 4.5 дн."},
					{"id": "100004", "offer_id": "bulb-cool-04", "name": "Лампа холодный свет", "metric": "покрытие 6.7 дн."},
				},
			},
		},
		AffectedEntities: actions.ActionAffectedEntities{
			SKUs: []actions.ActionSKURef{{
				SKU:     "100001",
				OfferID: "lamp-white-01",
				Name:    "Светильник кухонный белый",
			}},
		},
		Evidence: []actions.ActionEvidence{
			{Kind: "metric", Label: "SKU в группе", Value: "4", Unit: "шт.", Description: fact1},
			{Kind: "metric", Label: "Мин. покрытие", Value: "0.8", Unit: "дней", Description: fact2},
		},
		Limitations: []string{"Не учитываются будущие поставки в пути."},
		Payload:     map[string]any{"source": "dev_seed", "candidate_id": candidateID},
	}
}

func buildSeedNoSalesWithStock(date string) seedAction {
	candidateID := fmt.Sprintf("seed_no_sales_with_stock_%s", date)
	groupKey := candidateID
	fact1 := candidateID + "_fact_1"
	fact2 := candidateID + "_fact_2"
	return seedAction{
		ActionKey:         "seed-no-sales-with-stock",
		Type:              actions.ActionTypeNoSalesWithStock,
		Category:          actions.ActionCategorySales,
		Priority:          actions.ActionPriorityImportant,
		Title:             "Проверить SKU без продаж при наличии остатка",
		Reason:            "Есть остатки, но нет заказов за 14 дней.",
		NextStep:          "Проверьте карточку товара и цену, затем запустите тест в рекламе.",
		SourceCandidateID: candidateID,
		EvidenceFactIDs:   []string{fact1, fact2},
		CTAKeys: []string{
			actions.ActionCTAOpenSKUList,
			actions.ActionCTAExportNoSalesSKUs,
			actions.ActionCTAAskAI,
			actions.ActionCTAMarkChecked,
		},
		EntityGroupRef: groupKey,
		EntityGroup: &seedEntityGroup{
			GroupKey:   groupKey,
			EntityType: "sku",
			EntityIDs:  []string{"100011", "100012"},
			Summary: map[string]any{
				"entities": []map[string]any{
					{"id": "100011", "offer_id": "vase-glass-01", "name": "Ваза стеклянная", "metric": "0 заказов / 14 дн."},
					{"id": "100012", "offer_id": "vase-ceramic-02", "name": "Ваза керамическая", "metric": "0 заказов / 14 дн."},
				},
			},
		},
		AffectedEntities: actions.ActionAffectedEntities{
			SKUs: []actions.ActionSKURef{{SKU: "100011", OfferID: "vase-glass-01", Name: "Ваза стеклянная"}},
		},
		Evidence: []actions.ActionEvidence{
			{Kind: "metric", Label: "SKU без продаж", Value: "2", Unit: "шт.", Description: fact1},
			{Kind: "metric", Label: "Период", Value: "14", Unit: "дней", Description: fact2},
		},
		Payload: map[string]any{"source": "dev_seed", "candidate_id": candidateID},
	}
}

func buildSeedSKUOrdersDrop(date string) seedAction {
	candidateID := fmt.Sprintf("seed_sku_orders_drop_%s", date)
	groupKey := candidateID
	fact1 := candidateID + "_fact_1"
	fact2 := candidateID + "_fact_2"
	return seedAction{
		ActionKey:         "seed-sku-orders-drop",
		Type:              actions.ActionTypeSKUOrdersDrop,
		Category:          actions.ActionCategorySKU,
		Priority:          actions.ActionPriorityImportant,
		Title:             "Разобрать падение заказов по SKU",
		Reason:            "Падение заказов >30% неделя к неделе.",
		NextStep:          "Сравните цену и доступность, проверьте конкурентную выдачу.",
		SourceCandidateID: candidateID,
		EvidenceFactIDs:   []string{fact1, fact2},
		CTAKeys: []string{
			actions.ActionCTAOpenSKUList,
			actions.ActionCTAAskAI,
			actions.ActionCTAMarkChecked,
		},
		EntityGroupRef: groupKey,
		EntityGroup: &seedEntityGroup{
			GroupKey:   groupKey,
			EntityType: "sku",
			EntityIDs:  []string{"100021"},
			Summary: map[string]any{
				"entities": []map[string]any{
					{"id": "100021", "offer_id": "chair-wood-01", "name": "Стул деревянный", "metric": "-34% заказов"},
				},
			},
		},
		AffectedEntities: actions.ActionAffectedEntities{
			SKUs: []actions.ActionSKURef{{SKU: "100021", OfferID: "chair-wood-01", Name: "Стул деревянный"}},
		},
		Evidence: []actions.ActionEvidence{
			{Kind: "metric", Label: "Падение заказов", Value: "34", Unit: "%", Description: fact1},
			{Kind: "metric", Label: "Период сравнения", Value: "7", Unit: "дней", Description: fact2},
		},
		Payload: map[string]any{"source": "dev_seed", "candidate_id": candidateID},
	}
}

func buildSeedKeySKURisk(date string) seedAction {
	candidateID := fmt.Sprintf("seed_key_sku_risk_%s", date)
	groupKey := candidateID
	fact1 := candidateID + "_fact_1"
	fact2 := candidateID + "_fact_2"
	return seedAction{
		ActionKey:         "seed-key-sku-risk",
		Type:              actions.ActionTypeKeySKURisk,
		Category:          actions.ActionCategorySKU,
		Priority:          actions.ActionPriorityCritical,
		Title:             "Снизить риск по ключевым SKU",
		Reason:            "Ключевые SKU просели по видимости и выручке.",
		NextStep:          "Обновите контент карточки и проверьте остатки/ставки.",
		SourceCandidateID: candidateID,
		EvidenceFactIDs:   []string{fact1, fact2},
		CTAKeys: []string{
			actions.ActionCTAOpenSKUList,
			actions.ActionCTAAskAI,
			actions.ActionCTAMarkChecked,
		},
		EntityGroupRef: groupKey,
		EntityGroup: &seedEntityGroup{
			GroupKey:   groupKey,
			EntityType: "sku",
			EntityIDs:  []string{"100031", "100032"},
			Summary: map[string]any{
				"entities": []map[string]any{
					{"id": "100031", "offer_id": "sofa-grey-01", "name": "Диван серый", "metric": "выручка -22%"},
					{"id": "100032", "offer_id": "sofa-beige-02", "name": "Диван бежевый", "metric": "выручка -18%"},
				},
			},
		},
		AffectedEntities: actions.ActionAffectedEntities{
			SKUs: []actions.ActionSKURef{{SKU: "100031", OfferID: "sofa-grey-01", Name: "Диван серый"}},
		},
		Evidence: []actions.ActionEvidence{
			{Kind: "metric", Label: "Ключевые SKU", Value: "2", Unit: "шт.", Description: fact1},
			{Kind: "metric", Label: "Просадка выручки", Value: "22", Unit: "%", Description: fact2},
		},
		Payload: map[string]any{"source": "dev_seed", "candidate_id": candidateID},
	}
}

func buildSeedAdHighDRR(date string) seedAction {
	candidateID := fmt.Sprintf("seed_ad_high_drr_%s", date)
	groupKey := candidateID
	fact1 := candidateID + "_fact_1"
	fact2 := candidateID + "_fact_2"
	return seedAction{
		ActionKey:         "seed-ad-high-drr",
		Type:              actions.ActionTypeAdHighDRR,
		Category:          actions.ActionCategoryAdvertising,
		Priority:          actions.ActionPriorityWatch,
		Title:             "Снизить DRR в неэффективных кампаниях",
		Reason:            "DRR выше целевого в 2x.",
		NextStep:          "Уменьшите ставки по слабым фразам и перераспределите бюджет.",
		SourceCandidateID: candidateID,
		EvidenceFactIDs:   []string{fact1, fact2},
		CTAKeys: []string{
			actions.ActionCTAOpenCampaignList,
			actions.ActionCTAExportHighDRRCampaigns,
			actions.ActionCTAAskAI,
			actions.ActionCTAMarkChecked,
		},
		EntityGroupRef: groupKey,
		EntityGroup: &seedEntityGroup{
			GroupKey:   groupKey,
			EntityType: "campaign",
			EntityIDs:  []string{"200001", "200002"},
			Summary: map[string]any{
				"entities": []map[string]any{
					{"id": "200001", "name": "Поиск — кухонные светильники", "metric": "DRR 48%"},
					{"id": "200002", "name": "Трафареты — лампы", "metric": "DRR 52%"},
				},
			},
		},
		AffectedEntities: actions.ActionAffectedEntities{
			Campaigns: []actions.ActionCampaignRef{{CampaignID: "200001", Name: "Поиск — кухонные светильники"}},
		},
		Evidence: []actions.ActionEvidence{
			{Kind: "metric", Label: "Кампании", Value: "2", Unit: "шт.", Description: fact1},
			{Kind: "metric", Label: "Порог DRR", Value: "25", Unit: "%", Description: fact2},
		},
		Limitations: []string{"Рекомендации только на уровне кампаний."},
		Payload:     map[string]any{"source": "dev_seed", "candidate_id": candidateID},
	}
}

func buildSeedAdSpendNoOrders(date string) seedAction {
	candidateID := fmt.Sprintf("seed_ad_spend_no_orders_%s", date)
	groupKey := candidateID
	fact1 := candidateID + "_fact_1"
	fact2 := candidateID + "_fact_2"
	return seedAction{
		ActionKey:         "seed-ad-spend-no-orders",
		Type:              actions.ActionTypeAdSpendNoOrders,
		Category:          actions.ActionCategoryAdvertising,
		Priority:          actions.ActionPriorityImportant,
		Title:             "Остановить расход без заказов",
		Reason:            "Расход есть, заказов нет.",
		NextStep:          "Поставьте паузу на группы без конверсии > 7 дней.",
		SourceCandidateID: candidateID,
		EvidenceFactIDs:   []string{fact1, fact2},
		CTAKeys: []string{
			actions.ActionCTAOpenCampaignList,
			actions.ActionCTAExportNoOrderCampaigns,
			actions.ActionCTAAskAI,
			actions.ActionCTAMarkChecked,
		},
		EntityGroupRef: groupKey,
		EntityGroup: &seedEntityGroup{
			GroupKey:   groupKey,
			EntityType: "campaign",
			EntityIDs:  []string{"200011"},
			Summary: map[string]any{
				"entities": []map[string]any{
					{"id": "200011", "name": "Поиск — вазы", "metric": "расход 4200 ₽ / 0 заказов"},
				},
			},
		},
		AffectedEntities: actions.ActionAffectedEntities{
			Campaigns: []actions.ActionCampaignRef{{CampaignID: "200011", Name: "Поиск — вазы"}},
		},
		Evidence: []actions.ActionEvidence{
			{Kind: "metric", Label: "Расход", Value: "4200", Unit: "₽", Description: fact1},
			{Kind: "metric", Label: "Заказы", Value: "0", Unit: "шт.", Description: fact2},
		},
		Limitations: []string{"Рекомендации только на уровне кампаний."},
		Payload:     map[string]any{"source": "dev_seed", "candidate_id": candidateID},
	}
}

func buildSeedFunnelWeakTransition(date string) seedAction {
	candidateID := fmt.Sprintf("seed_funnel_weak_transition_%s", date)
	fact1 := candidateID + "_fact_1"
	fact2 := candidateID + "_fact_2"
	return seedAction{
		ActionKey:         "seed-funnel-weak-transition",
		Type:              actions.ActionTypeFunnelWeakTransition,
		Category:          actions.ActionCategoryFunnel,
		Priority:          actions.ActionPriorityWatch,
		Title:             "Усилить переход в воронке",
		Reason:            "Слабый переход из просмотра в корзину.",
		NextStep:          "Перепроверьте цену, фото и сроки доставки.",
		SourceCandidateID: candidateID,
		EvidenceFactIDs:   []string{fact1, fact2},
		CTAKeys: []string{
			actions.ActionCTAAskAI,
			actions.ActionCTAMarkChecked,
		},
		AffectedEntities: actions.ActionAffectedEntities{
			Funnel: &actions.ActionFunnelRef{
				TransitionKey: "pdp_to_cart",
				FromStage:     "Просмотры карточки",
				ToStage:       "Добавления в корзину",
			},
		},
		Evidence: []actions.ActionEvidence{
			{Kind: "metric", Label: "Конверсия перехода", Value: "1.8", Unit: "%", Description: fact1},
			{Kind: "metric", Label: "Бенчмарк", Value: "3.2", Unit: "%", Description: fact2},
		},
		Payload: map[string]any{"source": "dev_seed", "candidate_id": candidateID},
	}
}

func upsertSeedEntityGroup(ctx context.Context, q *dbgen.Queries, sellerID, runID int64, metricDate time.Time, group *seedEntityGroup) error {
	if group == nil {
		return nil
	}
	idsJSON, _ := json.Marshal(group.EntityIDs)
	summaryJSON, _ := json.Marshal(group.Summary)
	_, err := q.UpsertActionEntityGroup(ctx, dbgen.UpsertActionEntityGroupParams{
		SellerAccountID:       sellerID,
		MetricDate:            pgtype.Date{Time: metricDate, Valid: true},
		GroupKey:              group.GroupKey,
		EntityType:            group.EntityType,
		EntityIdsJson:         idsJSON,
		SummaryJson:           summaryJSON,
		ActionGenerationRunID: runID,
	})
	return err
}

func upsertSeedAction(ctx context.Context, q *dbgen.Queries, sellerID, runID int64, metricDate time.Time, item seedAction) error {
	evidence, _ := json.Marshal(item.Evidence)
	limitations, _ := json.Marshal(item.Limitations)
	ctaJSON, _ := json.Marshal(buildSeedCTAs(item.CTAKeys))
	affected, _ := json.Marshal(item.AffectedEntities)
	sourceIDs, _ := json.Marshal([]string{item.SourceCandidateID})
	factIDs, _ := json.Marshal(item.EvidenceFactIDs)
	payload, _ := json.Marshal(item.Payload)
	_, err := q.UpsertActionItem(ctx, dbgen.UpsertActionItemParams{
		SellerAccountID:        sellerID,
		RunID:                  pgtype.Int8{Int64: runID, Valid: true},
		ActionKey:              item.ActionKey,
		Type:                   string(item.Type),
		Category:               string(item.Category),
		Priority:               string(item.Priority),
		Title:                  item.Title,
		Summary:                pgtype.Text{String: item.Reason, Valid: true},
		Reason:                 item.Reason,
		NextStep:               item.NextStep,
		ExpectedEffect:         pgtype.Text{String: "Рост эффективности", Valid: true},
		EvidenceJson:           evidence,
		LimitationsJson:        limitations,
		CtaJson:                ctaJSON,
		AffectedEntitiesJson:   affected,
		EntityGroupRef:         pgtype.Text{String: item.EntityGroupRef, Valid: item.EntityGroupRef != ""},
		SourceCandidateIdsJson: sourceIDs,
		EvidenceFactIdsJson:    factIDs,
		PayloadJson:            payload,
		MetricDate:             pgtype.Date{Time: metricDate, Valid: true},
	})
	return err
}

func buildSeedCTAs(keys []string) []actions.ActionCTA {
	out := make([]actions.ActionCTA, 0, len(keys))
	for _, key := range keys {
		if cta, ok := seedCTAForKey(key); ok {
			out = append(out, cta)
		}
	}
	return out
}

func seedCTAForKey(key string) (actions.ActionCTA, bool) {
	labels := map[string]actions.ActionCTA{
		actions.ActionCTAOpenSKUList:              {Type: "navigate", Label: "Открыть список SKU", Href: "/app/products"},
		actions.ActionCTAOpenCampaignList:         {Type: "navigate", Label: "Открыть кампании", Href: "/app/advertising"},
		actions.ActionCTAExportStockReplenishment: {Type: "export", Label: "Экспорт пополнения", ExportType: "stock_replenishment"},
		actions.ActionCTAExportNoSalesSKUs:        {Type: "export", Label: "Экспорт SKU без продаж", ExportType: "no_sales_skus"},
		actions.ActionCTAExportSKUOrdersDrop:      {Type: "export", Label: "Экспорт падения заказов", ExportType: "sku_orders_drop"},
		actions.ActionCTAExportHighDRRCampaigns:   {Type: "export", Label: "Экспорт кампаний с высоким ДРР", ExportType: "high_drr_campaigns"},
		actions.ActionCTAExportNoOrderCampaigns:   {Type: "export", Label: "Экспорт кампаний без заказов", ExportType: "no_order_campaigns"},
		actions.ActionCTAAskAI:                    {Type: "ask_ai", Label: "Спросить AI"},
		actions.ActionCTAMarkChecked:              {Type: "mark_checked", Label: "Отметить выполненным"},
	}
	cta, ok := labels[key]
	if !ok {
		return actions.ActionCTA{}, false
	}
	cta.Params = map[string]string{"cta_key": key}
	return cta, true
}

func finalStatusForScenario(s ActionsSeedScenario) string {
	switch s {
	case ActionsSeedScenarioEmpty:
		return "empty"
	case ActionsSeedScenarioChecked, ActionsSeedScenarioMixed:
		return "completed_partial"
	default:
		return "completed"
	}
}
