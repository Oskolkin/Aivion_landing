package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

type DiscoveryDeps struct {
	Repo         *analysispg.Repository
	Store        artifacts.ArtifactStore
	API          factualapi.API
	Provider     discovery.Provider
	DeepProvider deep.Provider
	LoadBody     func(ctx context.Context, runID int64, stageKey string) ([]byte, string, error)
	Sealed       func(ctx context.Context, run analysispg.Run) (factualapi.FactualReadContext, error)
}

func persistKind(ctx context.Context, repo *analysispg.Repository, store artifacts.ArtifactStore, w Work, seller int64, kind artifacts.Kind, schema string, body []byte) error {
	_, err := artifacts.PutBeforeState(ctx, store, artifacts.Location{SellerID: seller, RunID: w.RunID, Stage: string(w.Stage), Kind: kind}, body, func(a artifacts.Artifact) error {
		attemptID := w.StageAttemptID
		stageID := w.StageID
		_, e := repo.AddArtifactRecord(ctx, analysispg.ArtifactRecordInput{
			RunID: w.RunID, StageID: &stageID, StageAttemptID: &attemptID, StageKey: string(w.Stage),
			Kind: string(kind), SchemaVersion: schema, StorageKey: a.Key, SHA256: a.Hash, Bytes: a.Size,
		})
		return e
	})
	return err
}

func DiscoveryPackHandler(d DiscoveryDeps) HandlerFunc {
	return func(ctx context.Context, w Work) (Output, error) {
		run, err := d.Repo.GetRun(ctx, w.RunID)
		if err != nil {
			return Output{}, err
		}
		sealed, err := d.Sealed(ctx, run)
		if err != nil {
			return Output{}, err
		}
		detailBody, detHash, err := d.LoadBody(ctx, w.RunID, string(stages.BuildLandscapeDetail))
		if err != nil {
			return Output{}, err
		}
		var detail landscape.LandscapeSet
		if err = json.Unmarshal(detailBody, &detail); err != nil {
			return Output{}, err
		}
		selBody, selArtSHA, err := d.LoadBody(ctx, w.RunID, string(stages.RunMateriality))
		if err != nil {
			return Output{}, err
		}
		var sel materiality.SelectionArtifact
		if err = json.Unmarshal(selBody, &sel); err != nil {
			return Output{}, err
		}
		sealStage, err := d.Repo.GetStage(ctx, w.RunID, string(stages.SealUniverse))
		if err != nil {
			return Output{}, err
		}
		model := discoveryModelFromEnv()
		maxOut := discoveryMaxOutputFromEnv()
		built, err := discovery.BuildPack(ctx, discovery.BuildInput{
			SellerAccountID: run.SellerID, RunID: run.ID, UniverseID: run.UniverseID,
			UniverseHash: sealStage.OutputHash, LandscapeDetailHash: detHash,
			MaterialitySelectionHash: sel.SelectionHash, SelectionArtifactSHA: selArtSHA,
			ContractBundleHash: run.ContractHash, AnalysisDate: run.AnalysisDate, CutoffAt: run.CutoffAt,
			Detail: detail, Selection: sel, Sealed: sealed, API: d.API, Model: model, MaxOutputTokens: maxOut,
		})
		if err != nil {
			discovery.ObservePack("fail", 0, 0, 0, 0)
			return Output{}, err
		}
		if err = persistKind(ctx, d.Repo, d.Store, w, run.SellerID, artifacts.DiscoveryFactHandleMap, discovery.FactHandleMapSchemaV2, built.HandleMapBytes); err != nil {
			return Output{}, err
		}
		if err = persistKind(ctx, d.Repo, d.Store, w, run.SellerID, artifacts.DiscoveryRequestManifest, discovery.RequestContractV2, built.ManifestBytes); err != nil {
			return Output{}, err
		}
		discovery.ObservePack("ok", len(built.PackBytes), built.Pack.EstimatedInputTokens, len(built.HandleMap.Entries), len(built.Pack.Payload.EvidenceMenu))
		return Output{Kind: artifacts.DiscoveryPack, Body: built.PackBytes, SchemaVersion: discovery.PackArtifactSchemaV2}, nil
	}
}

func DiscoveryAIHandler(d DiscoveryDeps) HandlerFunc {
	return func(ctx context.Context, w Work) (Output, error) {
		run, err := d.Repo.GetRun(ctx, w.RunID)
		if err != nil {
			return Output{}, err
		}
		packBody, packHash, err := d.LoadBody(ctx, w.RunID, string(stages.BuildDiscoveryPack))
		if err != nil {
			return Output{}, err
		}
		envelope, payload, err := discovery.UnmarshalPackWire(packBody)
		if err != nil {
			return Output{}, err
		}
		packStage, err := d.Repo.GetStage(ctx, w.RunID, string(stages.BuildDiscoveryPack))
		if err != nil {
			return Output{}, err
		}
		if packStage.AcceptedAttemptID == nil {
			return Output{}, fmt.Errorf("runtime: BUILD_DISCOVERY_PACK accepted attempt missing")
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
		_, manKey, _, err := d.Repo.ArtifactByStageAttemptAndKind(ctx, w.RunID, *packStage.AcceptedAttemptID, string(artifacts.DiscoveryRequestManifest))
		if err != nil {
			return Output{}, err
		}
		manBody, err := d.Store.Get(ctx, manKey)
		if err != nil {
			return Output{}, err
		}
		var manifest discovery.RequestManifestV1
		if err = json.Unmarshal(manBody, &manifest); err != nil {
			return Output{}, err
		}
		idx := discovery.IndexFromPayload(payload)
		payloadBytes, err := canonicaljson.Marshal(payload)
		if err != nil {
			return Output{}, err
		}

		if id, key, hash, findErr := d.Repo.ArtifactByStageAttemptAndKind(ctx, w.RunID, w.StageAttemptID, string(artifacts.DiscoveryOutput)); findErr == nil && hash != "" {
			if prev, getErr := d.Store.Get(ctx, key); getErr == nil {
				var existing discovery.OutputV1
				if json.Unmarshal(prev, &existing) == nil {
					rep := discovery.Validate(discovery.ValidateInput{Output: existing, Index: idx, Handles: handles, Menu: payload.EvidenceMenu, Budget: discovery.CurrentOutputBudget()})
					if rep.Valid {
						_ = id
						_ = packHash
						return Output{Kind: artifacts.DiscoveryOutput, Body: prev, SchemaVersion: discovery.OutputSchemaV1}, nil
					}
				}
			}
		}

		req := discovery.Request{
			Model: manifest.Model, Instructions: discovery.StableInstructionsV1,
			PayloadJSON: payloadBytes, Schema: discovery.OutputJSONSchema(),
			SchemaName: firstNonEmpty(manifest.SchemaName, discovery.OutputSchemaNameV1), MaxOutputTokens: manifest.MaxOutputTokens,
		}
		vin := discovery.ValidateInput{Index: idx, Handles: handles, Menu: payload.EvidenceMenu, Budget: discovery.CurrentOutputBudget()}
		attempts, out, err := discovery.RunAttempts(ctx, d.Provider, req, vin, discovery.CurrentAttemptPolicy().MaxModelAttempts, func(ctx context.Context, at discovery.ModelAttempt) error {
			return recordAIAttempt(ctx, d.Repo, d.Store, w, run.SellerID, at)
		})
		_ = attempts
		if err != nil {
			return Output{}, err
		}
		body, err := canonicaljson.Marshal(out)
		if err != nil {
			return Output{}, err
		}
		return Output{Kind: artifacts.DiscoveryOutput, Body: body, SchemaVersion: discovery.OutputSchemaV1}, nil
	}
}

func recordAIAttempt(ctx context.Context, repo *analysispg.Repository, store artifacts.ArtifactStore, w Work, seller int64, at discovery.ModelAttempt) error {
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
	if !at.Report.Valid && at.Class != discovery.AttemptValid {
		code = string(at.Class)
		if len(at.Report.Errors) > 0 {
			msg = at.Response.Refusal
			if msg == "" {
				msg = strings.Join(at.Report.Errors, "; ")
			}
		}
	}
	if _, err := repo.AddAIAttempt(ctx, analysispg.AIAttemptInput{
		StageAttemptID: w.StageAttemptID, Provider: "OPENAI", Model: firstNonEmpty(at.Model, discoveryModelFromEnv()),
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
		if err := persistKind(ctx, repo, store, w, seller, artifacts.DiscoveryOutputAttempt, "discovery_output_attempt_v1", body); err != nil {
			return fmt.Errorf("persist DISCOVERY_OUTPUT_ATTEMPT: %w", err)
		}
	}
	discovery.ObserveAI(string(at.Class), at.Response.Latency.Seconds(), inTok, outTok, cached)
	return nil
}

func ptr64(v int64) *int64 {
	x := v
	return &x
}

func ValidateDiscoveryHandler(d DiscoveryDeps) HandlerFunc {
	return func(ctx context.Context, w Work) (Output, error) {
		run, err := d.Repo.GetRun(ctx, w.RunID)
		if err != nil {
			return Output{}, err
		}
		outBody, _, err := d.LoadBody(ctx, w.RunID, string(stages.DiscoveryAI))
		if err != nil {
			return Output{}, err
		}
		var raw discovery.OutputV1
		if err = json.Unmarshal(outBody, &raw); err != nil {
			return Output{}, err
		}
		packBody, _, err := d.LoadBody(ctx, w.RunID, string(stages.BuildDiscoveryPack))
		if err != nil {
			return Output{}, err
		}
		envelope, payload, err := discovery.UnmarshalPackWire(packBody)
		if err != nil {
			return Output{}, err
		}
		packStage, err := d.Repo.GetStage(ctx, w.RunID, string(stages.BuildDiscoveryPack))
		if err != nil {
			return Output{}, err
		}
		if packStage.AcceptedAttemptID == nil {
			return Output{}, fmt.Errorf("runtime: BUILD_DISCOVERY_PACK accepted attempt missing")
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
		idx := discovery.IndexFromPayload(payload)
		vin := discovery.ValidateInput{Output: raw, Index: idx, Handles: handles, Menu: payload.EvidenceMenu, Budget: discovery.CurrentOutputBudget()}
		validated, rep := discovery.ValidateAuthoritative(vin)
		if !rep.Valid {
			discovery.ObserveValidation(rep, raw.DiscoveryState)
			return Output{}, discovery.Failed(discovery.CodeValidationFailed, strings.Join(rep.Errors, "; "))
		}
		repBody, err := canonicaljson.Marshal(rep)
		if err != nil {
			return Output{}, err
		}
		if err = persistKind(ctx, d.Repo, d.Store, w, run.SellerID, artifacts.DiscoveryValidationReport, discovery.ValidationReportSchemaV1, repBody); err != nil {
			return Output{}, err
		}
		body, err := canonicaljson.Marshal(validated)
		if err != nil {
			return Output{}, err
		}
		discovery.ObserveValidation(rep, raw.DiscoveryState)
		return Output{Kind: artifacts.DiscoveryValidatedOutput, Body: body, SchemaVersion: discovery.ValidatedOutputSchemaV1}, nil
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func discoveryModelFromEnv() string {
	if v := strings.TrimSpace(os.Getenv("OPENAI_DISCOVERY_MODEL")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("OPENAI_MODEL")); v != "" {
		return v
	}
	return "gpt-4.1-mini"
}

func discoveryMaxOutputFromEnv() int {
	if v := strings.TrimSpace(os.Getenv("OPENAI_DISCOVERY_MAX_OUTPUT_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return discovery.DefaultMaxOutputTokens()
}
