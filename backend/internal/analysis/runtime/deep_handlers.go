package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidence"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

func DeepPackHandler(d DiscoveryDeps) HandlerFunc {
	return func(ctx context.Context, w Work) (Output, error) {
		run, err := d.Repo.GetRun(ctx, w.RunID)
		if err != nil {
			return Output{}, err
		}
		sealStage, err := d.Repo.GetStage(ctx, w.RunID, string(stages.SealUniverse))
		if err != nil {
			return Output{}, err
		}
		packBody, packSHA, err := d.LoadBody(ctx, w.RunID, string(stages.BuildDiscoveryPack))
		if err != nil {
			return Output{}, err
		}
		penv, ppayload, err := discovery.UnmarshalPackWire(packBody)
		if err != nil {
			return Output{}, err
		}
		valBody, valSHA, err := d.LoadBody(ctx, w.RunID, string(stages.ValidateDiscovery))
		if err != nil {
			return Output{}, err
		}
		var validated discovery.ValidatedOutputV1
		if err = json.Unmarshal(valBody, &validated); err != nil {
			return Output{}, err
		}
		bundleBody, bundleSHA, err := d.LoadBody(ctx, w.RunID, string(stages.ExpandEvidence))
		if err != nil {
			return Output{}, err
		}
		var bundle evidence.BundleV1
		if err = json.Unmarshal(bundleBody, &bundle); err != nil {
			return Output{}, err
		}
		evStage, err := d.Repo.GetStage(ctx, w.RunID, string(stages.ExpandEvidence))
		if err != nil {
			return Output{}, err
		}
		if evStage.AcceptedAttemptID == nil {
			return Output{}, fmt.Errorf("runtime: EXPAND_EVIDENCE accepted attempt missing")
		}
		_, regKey, regSHA, err := d.Repo.ArtifactByStageAttemptAndKind(ctx, w.RunID, *evStage.AcceptedAttemptID, string(artifacts.AnalysisFactHandleRegistry))
		if err != nil {
			return Output{}, err
		}
		regBody, err := d.Store.Get(ctx, regKey)
		if err != nil {
			return Output{}, err
		}
		var handles evidence.UnifiedHandleRegistryV1
		if err = json.Unmarshal(regBody, &handles); err != nil {
			return Output{}, err
		}
		contBody, contSHA, err := d.LoadBody(ctx, w.RunID, string(stages.PrepareContinuity))
		if err != nil {
			return Output{}, err
		}
		var cont continuity.ContinuityContextV1
		if err = json.Unmarshal(contBody, &cont); err != nil {
			return Output{}, err
		}
		built, err := deep.BuildPack(deep.BuildInput{
			SellerAccountID: run.SellerID, RunID: run.ID, UniverseHash: sealStage.OutputHash,
			AnalysisDate: run.AnalysisDate, CutoffAt: run.CutoffAt, ContractHash: run.ContractHash,
			DiscoveryPackSHA: packSHA, ValidatedDiscoverySHA: valSHA, EvidenceBundleSHA: bundleSHA,
			FactRegistrySHA: regSHA, ContinuitySHA: contSHA,
			DiscoveryEnvelope: penv, DiscoveryPayload: ppayload, Validated: validated,
			Bundle: bundle, Handles: handles, Continuity: cont,
		})
		if err != nil {
			return Output{}, err
		}
		if err = persistKind(ctx, d.Repo, d.Store, w, run.SellerID, artifacts.DeepActionMenu, deep.ActionMenuArtifactV1, built.MenuBytes); err != nil {
			return Output{}, err
		}
		if err = persistKind(ctx, d.Repo, d.Store, w, run.SellerID, artifacts.DeepRelationshipPolicy, deep.RelationshipPolicyArtV1, built.PolicyBytes); err != nil {
			return Output{}, err
		}
		if err = persistKind(ctx, d.Repo, d.Store, w, run.SellerID, artifacts.DeepRequestManifest, deep.RequestManifestSchemaV1, built.ManifestBytes); err != nil {
			return Output{}, err
		}
		return Output{Kind: artifacts.DeepPack, Body: built.PackBytes, SchemaVersion: deep.PackArtifactSchemaV1}, nil
	}
}

func DeepAIHandler(d DiscoveryDeps) HandlerFunc {
	return func(ctx context.Context, w Work) (Output, error) {
		run, err := d.Repo.GetRun(ctx, w.RunID)
		if err != nil {
			return Output{}, err
		}
		packBody, packHash, err := d.LoadBody(ctx, w.RunID, string(stages.BuildDeepPack))
		if err != nil {
			return Output{}, err
		}
		envelope, payload, err := deep.UnmarshalPackWire(packBody)
		if err != nil {
			return Output{}, err
		}
		packStage, err := d.Repo.GetStage(ctx, w.RunID, string(stages.BuildDeepPack))
		if err != nil {
			return Output{}, err
		}
		if packStage.AcceptedAttemptID == nil {
			return Output{}, fmt.Errorf("runtime: BUILD_DEEP_PACK accepted attempt missing")
		}
		_, menuKey, menuSHA, err := d.Repo.ArtifactByStageAttemptAndKind(ctx, w.RunID, *packStage.AcceptedAttemptID, string(artifacts.DeepActionMenu))
		if err != nil {
			return Output{}, err
		}
		if menuSHA != envelope.ActionMenuHash {
			return Output{}, fmt.Errorf("%w: action menu", ErrArtifactBindingMismatch)
		}
		_ = menuKey
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return Output{}, err
		}
		if id, key, hash, findErr := d.Repo.ArtifactByStageAttemptAndKind(ctx, w.RunID, w.StageAttemptID, string(artifacts.DeepOutput)); findErr == nil && hash != "" {
			if prev, getErr := d.Store.Get(ctx, key); getErr == nil {
				var existing deep.OutputV1
				if json.Unmarshal(prev, &existing) == nil {
					rep := deep.Validate(deep.ValidateInput{Output: existing, Payload: payload, Snapshot: deep.RebuildSnapshot(payload), Budget: deep.CurrentOutputBudget()})
					if rep.Valid {
						_ = id
						_ = packHash
						return Output{Kind: artifacts.DeepOutput, Body: prev, SchemaVersion: deep.OutputSchemaV4}, nil
					}
				}
			}
		}
		mp := deep.CurrentModelPolicy()
		req := deep.Request{
			Model: mp.Model, Instructions: deep.StableInstructionsV2, PayloadJSON: payloadBytes,
			Schema: deep.OutputJSONSchema(), SchemaName: deep.OutputSchemaNameV4,
			MaxOutputTokens: mp.MaxOutputTokens, ReasoningEffort: mp.ReasoningEffort, Verbosity: mp.Verbosity,
		}
		vin := deep.ValidateInput{Payload: payload, Snapshot: deep.RebuildSnapshot(payload), Budget: deep.CurrentOutputBudget()}
		_, out, err := deep.RunAttempts(ctx, d.DeepProvider, req, vin, deep.CurrentAttemptPolicy().MaxModelAttempts, func(ctx context.Context, at deep.ModelAttempt) error {
			return recordDeepAIAttempt(ctx, d.Repo, d.Store, w, run.SellerID, at)
		})
		if err != nil {
			return Output{}, err
		}
		body, err := deep.MarshalOutputCanonical(out)
		if err != nil {
			return Output{}, err
		}
		return Output{Kind: artifacts.DeepOutput, Body: body, SchemaVersion: deep.OutputSchemaV4}, nil
	}
}

func recordDeepAIAttempt(ctx context.Context, repo *analysispg.Repository, store artifacts.ArtifactStore, w Work, seller int64, at deep.ModelAttempt) error {
	inTok, outTok, reasonTok, cached, total := at.Response.Usage.InputTokens, at.Response.Usage.OutputTokens, at.Response.Usage.ReasoningTokens, at.Response.Usage.CachedInputTokens, at.Response.Usage.TotalTokens
	lat := at.Response.Latency.Milliseconds()
	started := time.Now().UTC().Add(-at.Response.Latency)
	finished := time.Now().UTC()
	var respHash string
	if len(at.Response.RawJSON) > 0 {
		sum := sha256.Sum256(at.Response.RawJSON)
		respHash = hex.EncodeToString(sum[:])
	} else if at.Response.OutputText != "" {
		sum := sha256.Sum256([]byte(at.Response.OutputText))
		respHash = hex.EncodeToString(sum[:])
	}
	code, msg := "", ""
	if !at.Report.Valid && at.Class != deep.AttemptValid {
		code = string(at.Class)
		if len(at.Report.Errors) > 0 {
			msg = at.Response.Refusal
			if msg == "" {
				msg = strings.Join(at.Report.Errors, "; ")
			}
		}
	}
	if _, err := repo.AddAIAttempt(ctx, analysispg.AIAttemptInput{
		StageAttemptID: w.StageAttemptID, Provider: "OPENAI", Model: firstNonEmpty(at.Model, deep.CurrentModelPolicy().Model),
		ProviderRequestID: at.Response.ProviderRequestID,
		InputTokens:       ptr64(inTok), OutputTokens: ptr64(outTok), ReasoningTokens: ptr64(reasonTok),
		CachedTokens: ptr64(cached), TotalTokens: ptr64(total), LatencyMS: ptr64(lat),
		FinishReason: at.Response.Status, IncompleteReason: at.Response.IncompleteReason,
		RequestHash: at.RequestHash, ResponseHash: respHash, StartedAt: &started, FinishedAt: &finished,
		ErrorCode: code, ErrorMessage: openaix.RedactSecrets(msg),
	}); err != nil {
		return fmt.Errorf("persist analysis_ai_attempts: %w", err)
	}
	if at.Response.OutputText != "" || len(at.Response.RawJSON) > 0 {
		body := []byte(at.Response.OutputText)
		if len(body) == 0 {
			body = at.Response.RawJSON
		}
		if err := persistKind(ctx, repo, store, w, seller, artifacts.DeepOutputAttempt, "deep_output_attempt_v1", body); err != nil {
			return fmt.Errorf("persist DEEP_OUTPUT_ATTEMPT: %w", err)
		}
	}
	return nil
}

func ValidateDeepHandler(d DiscoveryDeps) HandlerFunc {
	return func(ctx context.Context, w Work) (Output, error) {
		run, err := d.Repo.GetRun(ctx, w.RunID)
		if err != nil {
			return Output{}, err
		}
		outBody, _, err := d.LoadBody(ctx, w.RunID, string(stages.DeepAI))
		if err != nil {
			return Output{}, err
		}
		var raw deep.OutputV1
		if err = json.Unmarshal(outBody, &raw); err != nil {
			return Output{}, err
		}
		packBody, _, err := d.LoadBody(ctx, w.RunID, string(stages.BuildDeepPack))
		if err != nil {
			return Output{}, err
		}
		_, payload, err := deep.UnmarshalPackWire(packBody)
		if err != nil {
			return Output{}, err
		}
		vin := deep.ValidateInput{Output: raw, Payload: payload, Snapshot: deep.RebuildSnapshot(payload), Budget: deep.CurrentOutputBudget()}
		rep := deep.Validate(vin)
		if !rep.Valid {
			deep.ObserveValidationFailure("stage13")
			return Output{}, deep.Failed(deep.CodeValidationFailed, strings.Join(rep.Errors, "; "))
		}
		validated := deep.AssignBackendRefs(raw)
		repBody, err := canonicaljson.Marshal(rep)
		if err != nil {
			return Output{}, err
		}
		if err = persistKind(ctx, d.Repo, d.Store, w, run.SellerID, artifacts.DeepValidationReport, deep.ValidationReportSchemaV1, repBody); err != nil {
			return Output{}, err
		}
		body, err := deep.MarshalValidatedOutputCanonical(validated)
		if err != nil {
			return Output{}, err
		}
		deep.ObserveValidated(raw, payload.DiscoveryResult.DiscoveryState == string(discovery.StateNoMeaningfulThreads))
		return Output{Kind: artifacts.DeepValidatedOutput, Body: body, SchemaVersion: deep.ValidatedOutputSchemaV1}, nil
	}
}
