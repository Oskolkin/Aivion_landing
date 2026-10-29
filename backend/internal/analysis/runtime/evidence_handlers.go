package runtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidence"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

func ExpandEvidenceHandler(d DiscoveryDeps) HandlerFunc {
	return func(ctx context.Context, w Work) (Output, error) {
		run, err := d.Repo.GetRun(ctx, w.RunID)
		if err != nil {
			return Output{}, err
		}
		sealed, err := d.Sealed(ctx, run)
		if err != nil {
			return Output{}, err
		}
		valBody, valHash, err := d.LoadBody(ctx, w.RunID, string(stages.ValidateDiscovery))
		if err != nil {
			return Output{}, err
		}
		var validated discovery.ValidatedOutputV1
		if err = json.Unmarshal(valBody, &validated); err != nil {
			return Output{}, err
		}
		packBody, packHash, err := d.LoadBody(ctx, w.RunID, string(stages.BuildDiscoveryPack))
		if err != nil {
			return Output{}, err
		}
		envelope, _, err := discovery.UnmarshalPackWire(packBody)
		if err != nil {
			return Output{}, err
		}
		if envelope.FactualUniverseHash != "" && run.UniverseID != "" && envelope.SellerAccountID != run.SellerID {
			return Output{}, evidence.Integrity("pack seller mismatch")
		}
		packStage, err := d.Repo.GetStage(ctx, w.RunID, string(stages.BuildDiscoveryPack))
		if err != nil {
			return Output{}, err
		}
		if packStage.AcceptedAttemptID == nil {
			return Output{}, evidence.Integrity("BUILD_DISCOVERY_PACK accepted attempt missing")
		}
		_, mapKey, mapSHA, err := d.Repo.ArtifactByStageAttemptAndKind(ctx, w.RunID, *packStage.AcceptedAttemptID, string(artifacts.DiscoveryFactHandleMap))
		if err != nil {
			return Output{}, err
		}
		if mapSHA != envelope.FactHandleMapHash {
			return Output{}, fmt.Errorf("%w: fact handle map", ErrArtifactBindingMismatch)
		}
		mapBody, err := d.Store.Get(ctx, mapKey)
		if err != nil {
			return Output{}, err
		}
		var handles discovery.FactHandleMapV1
		if err = json.Unmarshal(mapBody, &handles); err != nil {
			return Output{}, err
		}
		sealStage, err := d.Repo.GetStage(ctx, w.RunID, string(stages.SealUniverse))
		if err != nil {
			return Output{}, err
		}
		if sealStage.OutputHash == "" {
			return Output{}, evidence.Integrity("sealed universe hash missing")
		}
		contBody, _, err := d.LoadBody(ctx, w.RunID, string(stages.PrepareContinuity))
		if err != nil {
			return Output{}, err
		}
		var contCtx continuity.ContinuityContextV1
		if err = json.Unmarshal(contBody, &contCtx); err != nil {
			return Output{}, err
		}
		reg := evidence.ProductionRegistry(evidence.APIReads{API: d.API})
		out, err := evidence.Expand(ctx, evidence.ExpandInput{
			SellerID: run.SellerID, UniverseHash: sealStage.OutputHash, AnalysisDate: run.AnalysisDate, CutoffAt: run.CutoffAt,
			Validated: validated, Continuity: contCtx, ValidatedHash: valHash, DiscoveryPackHash: packHash, FactHandleMap: handles,
			FactHandleMapHash: mapSHA, EvidenceMenuHash: envelope.EvidenceMenuHash, Sealed: sealed, Registry: reg,
			Budget: evidence.CurrentBudget(),
		})
		if err != nil {
			return Output{}, err
		}
		if err = persistKind(ctx, d.Repo, d.Store, w, run.SellerID, artifacts.EvidenceRequestSet, evidence.RequestSetSchemaV2, out.RequestBytes); err != nil {
			return Output{}, err
		}
		if err = persistKind(ctx, d.Repo, d.Store, w, run.SellerID, artifacts.AnalysisFactHandleRegistry, evidence.FactHandleRegistryV1, out.HandleBytes); err != nil {
			return Output{}, err
		}
		if err = persistKind(ctx, d.Repo, d.Store, w, run.SellerID, artifacts.EvidenceExpansionReport, evidence.ExpansionReportSchemaV1, out.ReportBytes); err != nil {
			return Output{}, err
		}
		cls := CompletionSucceeded
		if out.Limited {
			cls = CompletionSucceededWithLimitations
		}
		return Output{Kind: artifacts.EvidenceBundle, Body: out.BundleBytes, SchemaVersion: evidence.BundleSchemaV2, CompletionClass: cls}, nil
	}
}
