package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actionreconcile"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/projection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/publication"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

func PlanActionReconciliationHandler(d DiscoveryDeps) HandlerFunc {
	return func(ctx context.Context, w Work) (Output, error) {
		run, err := d.Repo.GetRun(ctx, w.RunID)
		if err != nil {
			return Output{}, err
		}
		body, hash, err := d.LoadBody(ctx, w.RunID, string(stages.ValidateDeep))
		if err != nil {
			return Output{}, err
		}
		var validated deep.ValidatedOutputV1
		if err = json.Unmarshal(body, &validated); err != nil {
			return Output{}, err
		}
		contBody, contHash, err := d.LoadBody(ctx, w.RunID, string(stages.PrepareContinuity))
		if err != nil {
			return Output{}, err
		}
		var cont continuity.ContinuityContextV1
		if err = json.Unmarshal(contBody, &cont); err != nil {
			return Output{}, err
		}
		packStage, err := d.Repo.GetStage(ctx, w.RunID, string(stages.BuildDeepPack))
		if err != nil || packStage.AcceptedAttemptID == nil {
			return Output{}, fmt.Errorf("runtime: deep pack missing")
		}
		_, menuKey, _, err := d.Repo.ArtifactByStageAttemptAndKind(ctx, w.RunID, *packStage.AcceptedAttemptID, string(artifacts.DeepActionMenu))
		if err != nil {
			return Output{}, err
		}
		menuBody, err := d.Store.Get(ctx, menuKey)
		if err != nil {
			return Output{}, err
		}
		var menu actioncatalog.Menu
		_ = json.Unmarshal(menuBody, &menu)
		plan, rep, err := actionreconcile.Plan(actionreconcile.PlanInput{
			SellerID: run.SellerID, RunID: run.ID, Validated: validated, ValidatedHash: hash,
			Menu: menu, Continuity: cont, ContinuityHash: contHash, BasePublicationID: run.BasePublicationID,
			Now: time.Now().UTC(), Catalog: actioncatalog.Catalog(),
		})
		repBody, _ := canonicaljson.Marshal(rep)
		_ = persistKind(ctx, d.Repo, d.Store, w, run.SellerID, artifacts.ActionReconciliationReport, actionreconcile.ReportSchemaV1, repBody)
		if err != nil {
			return Output{}, err
		}
		planBody, err := actionreconcile.MarshalPlanCanonical(plan)
		if err != nil {
			return Output{}, err
		}
		return Output{Kind: artifacts.ActionReconciliationPlan, Body: planBody, SchemaVersion: actionreconcile.PlanSchemaV1}, nil
	}
}

func BuildPublicationHandler(d DiscoveryDeps) HandlerFunc {
	return func(ctx context.Context, w Work) (Output, error) {
		run, err := d.Repo.GetRun(ctx, w.RunID)
		if err != nil {
			return Output{}, err
		}
		_, planHash, err := d.LoadBody(ctx, w.RunID, string(stages.PlanActionReconciliation))
		if err != nil {
			return Output{}, err
		}
		_, deepHash, err := d.LoadBody(ctx, w.RunID, string(stages.ValidateDeep))
		if err != nil {
			return Output{}, err
		}
		seal, err := d.Repo.GetStage(ctx, w.RunID, string(stages.SealUniverse))
		if err != nil {
			return Output{}, err
		}
		gen := int64(1)
		if run.BasePublicationID != nil {
			gen = 0
		}
		c := publication.BuildCandidate(publication.CandidateV1{
			SellerAccountID: run.SellerID, AnalysisRunID: run.ID, ExpectedBasePublicationID: run.BasePublicationID,
			ExpectedPublicationGeneration: gen, AnalysisDate: run.AnalysisDate.UTC().Format("2006-01-02"),
			AnalysisCutoffAt: run.CutoffAt.UTC().Format(time.RFC3339Nano), FactualUniverseID: run.UniverseID,
			FactualUniverseHash: seal.OutputHash, ContractBundleHash: run.ContractHash,
			ActionReconciliationHash: planHash,
			Components: []publication.ComponentRef{
				{ComponentType: "DEEP_VALIDATED_OUTPUT", SchemaVersion: deep.ValidatedOutputSchemaV1, SHA256: deepHash},
				{ComponentType: "ACTION_RECONCILIATION_PLAN", SchemaVersion: actionreconcile.PlanSchemaV1, SHA256: planHash},
				{ComponentType: "FACTUAL_UNIVERSE", SchemaVersion: "v1", SHA256: seal.OutputHash},
			},
		})
		body, err := canonicaljson.Marshal(c)
		if err != nil {
			return Output{}, err
		}
		man, _ := canonicaljson.Marshal(publication.ManifestV1{SchemaVersion: publication.ComponentManifestV1, Components: c.Components})
		_ = persistKind(ctx, d.Repo, d.Store, w, run.SellerID, artifacts.PublicationComponentManifest, publication.ComponentManifestV1, man)
		return Output{Kind: artifacts.PublicationCandidate, Body: body, SchemaVersion: publication.CandidateSchemaV1}, nil
	}
}

func BuildProjectionsHandler(d DiscoveryDeps) HandlerFunc {
	return func(ctx context.Context, w Work) (Output, error) {
		run, err := d.Repo.GetRun(ctx, w.RunID)
		if err != nil {
			return Output{}, err
		}
		valBody, _, err := d.LoadBody(ctx, w.RunID, string(stages.ValidateDeep))
		if err != nil {
			return Output{}, err
		}
		var validated deep.ValidatedOutputV1
		if err = json.Unmarshal(valBody, &validated); err != nil {
			return Output{}, err
		}
		planBody, _, err := d.LoadBody(ctx, w.RunID, string(stages.PlanActionReconciliation))
		if err != nil {
			return Output{}, err
		}
		var plan actionreconcile.PlanV1
		if err = json.Unmarshal(planBody, &plan); err != nil {
			return Output{}, err
		}
		candBody, _, err := d.LoadBody(ctx, w.RunID, string(stages.BuildPublication))
		if err != nil {
			return Output{}, err
		}
		var cand publication.CandidateV1
		_ = json.Unmarshal(candBody, &cand)
		sealed, err := d.Sealed(ctx, run)
		if err != nil {
			return Output{}, err
		}
		built, err := projection.Build(ctx, projection.Input{
			SellerID: run.SellerID, AnalysisDate: run.AnalysisDate, CutoffAt: run.CutoffAt,
			UniverseHash: cand.FactualUniverseHash, Sealed: sealed, API: d.API, Validated: validated, Plan: plan,
		})
		if err != nil {
			return Output{}, err
		}
		kindBy := map[string]artifacts.Kind{
			projection.KeyOverview: artifacts.OverviewProjection, projection.KeyFunnel: artifacts.FunnelProjection,
			projection.KeyStock: artifacts.StockProjection, projection.KeyAdvertising: artifacts.AdvertisingProjection,
			projection.KeyActionsSnapshot: artifacts.ActionsSnapshotProjection, projection.KeyChatContext: artifacts.ChatContextProjection,
			projection.KeyStockSKURows: artifacts.StockSKURows, projection.KeyAdCampaignRows: artifacts.AdCampaignRows,
			projection.KeySKUFocusRows: artifacts.SKUFocusRows,
		}
		for k, raw := range built.Bytes {
			if kind, ok := kindBy[k]; ok {
				_ = persistKind(ctx, d.Repo, d.Store, w, run.SellerID, kind, "v1", raw)
			}
		}
		return Output{Kind: artifacts.PublicationProjectionManifest, Body: built.Bytes["MANIFEST"], SchemaVersion: projection.ManifestV1}, nil
	}
}

func ValidatePublicationHandler(d DiscoveryDeps) HandlerFunc {
	return func(ctx context.Context, w Work) (Output, error) {
		run, err := d.Repo.GetRun(ctx, w.RunID)
		if err != nil {
			return Output{}, err
		}
		candBody, _, err := d.LoadBody(ctx, w.RunID, string(stages.BuildPublication))
		if err != nil {
			return Output{}, err
		}
		var cand publication.CandidateV1
		if err = json.Unmarshal(candBody, &cand); err != nil {
			return Output{}, err
		}
		manBody, _, err := d.LoadBody(ctx, w.RunID, string(stages.BuildProjections))
		if err != nil {
			return Output{}, err
		}
		var man projection.Manifest
		_ = json.Unmarshal(manBody, &man)
		planBody, _, err := d.LoadBody(ctx, w.RunID, string(stages.PlanActionReconciliation))
		if err != nil {
			return Output{}, err
		}
		var plan actionreconcile.PlanV1
		_ = json.Unmarshal(planBody, &plan)
		valBody, _, err := d.LoadBody(ctx, w.RunID, string(stages.ValidateDeep))
		if err != nil {
			return Output{}, err
		}
		var validated deep.ValidatedOutputV1
		_ = json.Unmarshal(valBody, &validated)
		rep := publication.Validate(cand, man, plan, validated, run.SellerID, run.ID, run.ContractHash)
		rb, _ := canonicaljson.Marshal(rep)
		_ = persistKind(ctx, d.Repo, d.Store, w, run.SellerID, artifacts.PublicationValidationReport, publication.ValidationReportV1, rb)
		if !rep.Valid {
			return Output{}, fmt.Errorf("publication validation failed: %v", rep.Errors)
		}
		vc := publication.ValidatedCandidateV1{SchemaVersion: publication.ValidatedCandidateSchemaV1, Candidate: cand, Valid: true}
		body, err := canonicaljson.Marshal(vc)
		if err != nil {
			return Output{}, err
		}
		return Output{Kind: artifacts.PublicationValidatedCandidate, Body: body, SchemaVersion: publication.ValidatedCandidateSchemaV1}, nil
	}
}

func PublishHandler(d DiscoveryDeps) HandlerFunc {
	return func(ctx context.Context, w Work) (Output, error) {
		run, err := d.Repo.GetRun(ctx, w.RunID)
		if err != nil {
			return Output{}, err
		}
		if run.ExecutionClass == "EVALUATION" {
			return Output{Kind: artifacts.PublishReceipt, Hash: canonicaljson.Hash("EVALUATION_NO_PUBLISH"), SchemaVersion: publication.PublishPolicyV1, SkipStore: true}, nil
		}
		body, _, err := d.LoadBody(ctx, w.RunID, string(stages.ValidatePublication))
		if err != nil {
			return Output{}, err
		}
		var vc publication.ValidatedCandidateV1
		if err = json.Unmarshal(body, &vc); err != nil {
			return Output{}, err
		}
		planBody, _, err := d.LoadBody(ctx, w.RunID, string(stages.PlanActionReconciliation))
		if err != nil {
			return Output{}, err
		}
		var plan actionreconcile.PlanV1
		_ = json.Unmarshal(planBody, &plan)
		valBody, _, err := d.LoadBody(ctx, w.RunID, string(stages.ValidateDeep))
		if err != nil {
			return Output{}, err
		}
		var validated deep.ValidatedOutputV1
		_ = json.Unmarshal(valBody, &validated)
		projStage, err := d.Repo.GetStage(ctx, w.RunID, string(stages.BuildProjections))
		if err != nil || projStage.AcceptedAttemptID == nil {
			return Output{}, fmt.Errorf("runtime: projections missing")
		}
		projBytes := map[string][]byte{}
		projHashes := map[string]string{}
		kindBy := map[string]string{
			string(artifacts.OverviewProjection):        projection.KeyOverview,
			string(artifacts.FunnelProjection):          projection.KeyFunnel,
			string(artifacts.StockProjection):           projection.KeyStock,
			string(artifacts.AdvertisingProjection):     projection.KeyAdvertising,
			string(artifacts.ActionsSnapshotProjection): projection.KeyActionsSnapshot,
			string(artifacts.ChatContextProjection):     projection.KeyChatContext,
			string(artifacts.StockSKURows):              projection.KeyStockSKURows,
			string(artifacts.AdCampaignRows):            projection.KeyAdCampaignRows,
			string(artifacts.SKUFocusRows):              projection.KeySKUFocusRows,
		}
		for kind, key := range kindBy {
			_, loc, sha, e := d.Repo.ArtifactByStageAttemptAndKind(ctx, w.RunID, *projStage.AcceptedAttemptID, kind)
			if e != nil {
				continue
			}
			raw, _ := d.Store.Get(ctx, loc)
			projBytes[key] = raw
			projHashes[key] = sha
		}
		lease := analysispg.Lease{RunID: w.RunID, StageID: w.StageID, AttemptID: w.StageAttemptID, SellerID: w.SellerID, Stage: string(w.Stage)}
		st, err := d.Repo.GetStage(ctx, w.RunID, string(w.Stage))
		if err == nil {
			lease.Token = st.LeaseToken
			lease.Version = st.LeaseVersion
		}
		res, err := d.Repo.PublishAtomic(ctx, analysispg.PublishInput{
			Run: run, Lease: lease, Candidate: vc.Candidate, Plan: plan, Validated: validated,
			ProjectionBytes: projBytes, ProjectionHashes: projHashes, AllowedNext: stages.ProductionImplementedStageNames(),
		})
		if errors.Is(err, analysispg.ErrEvaluationCannotPublish) {
			return Output{Kind: artifacts.PublishReceipt, Hash: canonicaljson.Hash("EVALUATION_NO_PUBLISH"), SkipStore: true}, nil
		}
		if err != nil {
			return Output{}, err
		}
		return Output{Kind: artifacts.PublishReceipt, Hash: vc.Candidate.PublicationContentHash, SkipStore: true, SchemaVersion: publication.PublishPolicyV1, PublishID: res.PublicationID}, nil
	}
}

func PostPublishHandler(d DiscoveryDeps) HandlerFunc {
	return func(ctx context.Context, w Work) (Output, error) {
		n, err := d.Repo.ProcessOutbox(ctx, 50)
		if err != nil {
			return Output{}, err
		}
		rep, _ := canonicaljson.Marshal(map[string]any{"schema_version": "post_publish_report_v1", "processed": n})
		return Output{Kind: artifacts.PostPublishReport, Body: rep, SchemaVersion: "post_publish_report_v1"}, nil
	}
}
