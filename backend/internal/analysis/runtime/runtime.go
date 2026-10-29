package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actionreconcile"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidence"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/policy"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/projection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/publication"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/observability"
	"go.opentelemetry.io/otel/attribute"
)

type Work struct {
	RunID, StageID, SellerID, StageAttemptID int64
	Stage                                    stages.Name
	Attempt                                  int
}
type Output struct {
	Kind            artifacts.Kind
	Body            []byte
	Hash            string
	SchemaVersion   string
	CompletionClass CompletionClass
	SkipStore       bool
	PublishID       int64
}

type CompletionClass string

const (
	CompletionSucceeded                CompletionClass = "SUCCEEDED"
	CompletionSucceededWithLimitations CompletionClass = "SUCCEEDED_WITH_LIMITATIONS"
)

type Handler interface {
	Execute(context.Context, Work) (Output, error)
}
type HandlerFunc func(context.Context, Work) (Output, error)

func (f HandlerFunc) Execute(ctx context.Context, w Work) (Output, error) { return f(ctx, w) }

// ProductionHandlers registers production stage handlers (1–10).
func ProductionHandlers(handlers map[stages.Name]Handler) map[stages.Name]Handler {
	out := make(map[stages.Name]Handler, len(handlers))
	allowed := map[stages.Name]bool{}
	for _, name := range stages.ProductionImplementedStageNames() {
		allowed[stages.Name(name)] = true
	}
	for name, h := range handlers {
		if h == nil || !allowed[name] {
			continue
		}
		out[name] = h
	}
	return out
}

type Executor struct {
	Repo       *analysispg.Repository
	Store      artifacts.ArtifactStore
	Policy     policy.Policy
	Handlers   map[stages.Name]Handler
	Registry   stages.Registry
	WorkerID   string
	Enqueue    EnqueueFunc
	NextStages []string // implemented stage keys allowed to become READY
}

type EnqueueFunc func(ctx context.Context, runID int64, stageKey string) error

func (e *Executor) implementedNext() []string {
	if len(e.NextStages) > 0 {
		return e.NextStages
	}
	return stages.ProductionImplementedStageNames()
}

func (e *Executor) ExecuteOne(ctx context.Context) (bool, error) {
	now := e.Policy.Clock.Now().UTC()
	_, _ = e.Repo.PromoteRetryableReady(ctx, now)
	// Best-effort: hash any READY stages we can identify via recovery scan.
	if items, err := e.Repo.ListRecoveryCandidates(ctx, now); err == nil {
		for _, it := range items {
			_ = e.EnsureExecutableInputHashes(ctx, it.RunID)
		}
	}
	lease, err := e.Repo.Claim(ctx, e.WorkerID, e.Policy.LeaseDuration, now)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return false, err
		}
		return false, nil
	}
	return true, e.runClaimed(ctx, lease)
}

func (e *Executor) ExecuteStage(ctx context.Context, runID int64, stageKey string) error {
	stage, err := e.Repo.GetStage(ctx, runID, stageKey)
	if err != nil {
		return err
	}
	if stage.Status == "SUCCEEDED" || stage.Status == "SUCCEEDED_WITH_LIMITATIONS" {
		return nil
	}
	run, err := e.Repo.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	if run.CancelRequestedAt != nil {
		return e.Repo.ApplyCancelAtBoundary(ctx, runID)
	}
	if err = e.EnsureExecutableInputHashes(ctx, runID); err != nil {
		return err
	}
	now := e.Policy.Clock.Now().UTC()
	_, _ = e.Repo.PromoteRetryableReady(ctx, now)
	lease, err := e.Repo.ClaimByRunStage(ctx, runID, stageKey, e.WorkerID, e.Policy.LeaseDuration, now)
	if err != nil {
		stage, getErr := e.Repo.GetStage(ctx, runID, stageKey)
		if getErr == nil && (stage.Status == "SUCCEEDED" || stage.Status == "SUCCEEDED_WITH_LIMITATIONS") {
			return nil
		}
		return err
	}
	return e.runClaimed(ctx, lease)
}

// EnsureExecutableInputHashes sets real input hashes for implemented stages before claim.
func (e *Executor) EnsureExecutableInputHashes(ctx context.Context, runID int64) error {
	run, err := e.Repo.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	list, err := e.Repo.ListStages(ctx, runID)
	if err != nil {
		return err
	}
	outByKey := map[string]string{}
	for _, s := range list {
		if s.Status == "SUCCEEDED" || s.Status == "SUCCEEDED_WITH_LIMITATIONS" {
			outByKey[s.Key] = s.OutputHash
		}
	}
	policyHash := materialityPolicyHash()
	budgetHash := materialityBudgetHash()
	laneRegistryHash := materialityLaneRegistryHash()
	for _, s := range list {
		if s.Status != "READY" && s.Status != "PENDING" && s.Status != "FAILED_RETRYABLE" {
			continue
		}
		var h string
		switch s.Key {
		case string(stages.ResolveContext):
			h = HashResolveContext(ResolveInput{
				SellerID: run.SellerID, AnalysisDate: run.AnalysisDate, CutoffAt: run.CutoffAt,
				RunKind: run.Kind, IntentHash: run.IntentHash, ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.SealUniverse):
			resolveOut := outByKey[string(stages.ResolveContext)]
			if resolveOut == "" {
				continue
			}
			h = HashSealUniverse(SealInput{
				ResolvedContextHash: resolveOut, AnalysisDate: run.AnalysisDate, CutoffAt: run.CutoffAt,
				ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.PrepareContinuity):
			h = HashPrepareContinuity(ContinuityInput{
				SellerID: run.SellerID, AnalysisDate: run.AnalysisDate, CutoffAt: run.CutoffAt,
				RunKind: run.Kind, IntentHash: run.IntentHash, ContractBundleHash: run.ContractHash,
				BasePublicationID: run.BasePublicationID, StageVersion: s.Version,
			})
		case string(stages.BuildLandscapeCore):
			sealOut := outByKey[string(stages.SealUniverse)]
			if sealOut == "" {
				continue
			}
			h = HashLandscapeCore(LandscapeCoreInput{
				UniverseManifestHash: sealOut, ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.RunMateriality):
			coreOut := outByKey[string(stages.BuildLandscapeCore)]
			if coreOut == "" {
				continue
			}
			h = HashMateriality(MaterialityInput{
				LandscapeCoreHash: coreOut, MaterialityPolicyHash: policyHash, LaneRegistryHash: laneRegistryHash,
				BudgetProfileHash: budgetHash, ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.BuildLandscapeDetail):
			sealOut := outByKey[string(stages.SealUniverse)]
			matOut := outByKey[string(stages.RunMateriality)]
			if sealOut == "" || matOut == "" {
				continue
			}
			h = HashLandscapeDetail(LandscapeDetailInput{
				UniverseManifestHash: sealOut, MaterialitySelectionHash: matOut,
				ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.BuildDiscoveryPack):
			sealOut := outByKey[string(stages.SealUniverse)]
			matOut := outByKey[string(stages.RunMateriality)]
			detOut := outByKey[string(stages.BuildLandscapeDetail)]
			if sealOut == "" || matOut == "" || detOut == "" {
				continue
			}
			h = HashBuildDiscoveryPack(BuildDiscoveryPackInput{
				UniverseManifestHash: sealOut, MaterialitySelectionHash: matOut, LandscapeDetailHash: detOut,
				DiscoveryPackSeal: discovery.PackContractSeal(), FactHandleRegistry: discovery.FactHandleRegistryV2,
				EvidenceCatalogSeal: discovery.EvidenceCatalogV1 + "|" + discovery.EvidenceMenuPolicyV3,
				PackBudgetVersion:   discovery.PackBudgetV2, ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.DiscoveryAI):
			packOut := outByKey[string(stages.BuildDiscoveryPack)]
			if packOut == "" {
				continue
			}
			packEnv, mapHash, manHash, err := e.loadAcceptedDiscoveryBindings(ctx, run.ID, packOut)
			if err != nil {
				return err
			}
			h = HashDiscoveryAI(DiscoveryAIInput{
				DiscoveryPackHash: packOut, PayloadHash: packEnv.PayloadHash, FactHandleMapHash: mapHash, RequestManifestHash: manHash,
				PromptPolicyVersion: discovery.PromptPolicyV1,
				OutputSchemaVersion: discovery.OutputSchemaV1, ModelPolicyVersion: discovery.ModelPolicyV1,
				OutputBudgetVersion: discovery.OutputBudgetV1, ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.ValidateDiscovery):
			packOut := outByKey[string(stages.BuildDiscoveryPack)]
			aiOut := outByKey[string(stages.DiscoveryAI)]
			if packOut == "" || aiOut == "" {
				continue
			}
			packEnv, mapHash, _, err := e.loadAcceptedDiscoveryBindings(ctx, run.ID, packOut)
			if err != nil {
				return err
			}
			h = HashValidateDiscovery(ValidateDiscoveryInput{
				AcceptedDiscoveryOutputHash: aiOut, DiscoveryPackHash: packOut, FactHandleMapHash: mapHash, EvidenceMenuHash: packEnv.EvidenceMenuHash,
				ValidationPolicyVersion: discovery.ValidationV3, OutputBudgetVersion: discovery.OutputBudgetV1,
				ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.ExpandEvidence):
			packOut := outByKey[string(stages.BuildDiscoveryPack)]
			valOut := outByKey[string(stages.ValidateDiscovery)]
			sealOut := outByKey[string(stages.SealUniverse)]
			contOut := outByKey[string(stages.PrepareContinuity)]
			if packOut == "" || valOut == "" || sealOut == "" || contOut == "" {
				continue
			}
			packEnv, mapHash, _, err := e.loadAcceptedDiscoveryBindings(ctx, run.ID, packOut)
			if err != nil {
				return err
			}
			h = HashExpandEvidence(ExpandEvidenceInput{
				AcceptedDiscoveryOutputHash: valOut, DiscoveryPackHash: packOut, EvidenceMenuHash: packEnv.EvidenceMenuHash,
				FactHandleMapHash: mapHash, FactualUniverseHash: sealOut, ContinuityContextHash: contOut,
				EvidenceCatalogVersion: evidence.CatalogV1, ResolverRegistryVersion: evidence.ResolverRegistryV1,
				RequestNormalizationVersion: evidence.RequestNormalizationV2, EvidenceEnvelopeVersion: evidence.EnvelopeSchemaV1,
				EvidenceBudgetVersion: evidence.BudgetProfileV1, FactHandleRegistryVersion: evidence.FactHandleRegistryV1,
				EvidenceValidationVersion: evidence.ExpansionValidationV1,
				ContractBundleHash:        run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.BuildDeepPack):
			valOut := outByKey[string(stages.ValidateDiscovery)]
			packOut := outByKey[string(stages.BuildDiscoveryPack)]
			evOut := outByKey[string(stages.ExpandEvidence)]
			contOut := outByKey[string(stages.PrepareContinuity)]
			if valOut == "" || packOut == "" || evOut == "" || contOut == "" {
				continue
			}
			regSHA, err := e.loadAcceptedEvidenceRegistrySHA(ctx, run.ID)
			if err != nil {
				return err
			}
			h = HashBuildDeepPack(BuildDeepPackInput{
				ValidatedDiscoveryHash: valOut, DiscoveryPackHash: packOut, EvidenceBundleHash: evOut,
				AnalysisFactHandleRegistryHash: regSHA, ContinuityContextHash: contOut,
				ActionCatalogVersion: actioncatalog.CatalogV1, ActionMenuPolicyVersion: actioncatalog.MenuPolicyV1,
				RelationshipClaimPolicyVersion: deep.RelationshipClaimPolicyV1, DeepPackSchemaVersion: deep.PackSchemaV2,
				DeepPackBudgetVersion: deep.PackBudgetV1, ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.DeepAI):
			deepOut := outByKey[string(stages.BuildDeepPack)]
			if deepOut == "" {
				continue
			}
			env, menuSHA, polSHA, err := e.loadAcceptedDeepBindings(ctx, run.ID, deepOut)
			if err != nil {
				return err
			}
			h = HashDeepAI(DeepAIInput{
				DeepPackArtifactSHA: deepOut, PayloadHash: env.PayloadHash, ActionMenuHash: menuSHA,
				RelationshipPolicyHash: polSHA, PromptHash: canonicaljson.Hash(deep.StableInstructionsV1),
				OutputSchemaHash: canonicaljson.Hash(deep.OutputJSONSchema()), ModelPolicyVersion: deep.ModelPolicyV1,
				AttemptPolicyVersion: deep.AIAttemptPolicyV1, OutputBudgetVersion: deep.OutputBudgetV1,
				ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.ValidateDeep):
			deepPack := outByKey[string(stages.BuildDeepPack)]
			aiOut := outByKey[string(stages.DeepAI)]
			if deepPack == "" || aiOut == "" {
				continue
			}
			_, menuSHA, polSHA, err := e.loadAcceptedDeepBindings(ctx, run.ID, deepPack)
			if err != nil {
				return err
			}
			h = HashValidateDeep(ValidateDeepInput{
				AcceptedDeepOutputHash: aiOut, DeepPackHash: deepPack, ActionMenuHash: menuSHA,
				RelationshipPolicyHash: polSHA, ValidationPolicyVersion: deep.ValidationV3,
				OutputBudgetVersion: deep.OutputBudgetV1, ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.PlanActionReconciliation):
			deepOut := outByKey[string(stages.ValidateDeep)]
			contOut := outByKey[string(stages.PrepareContinuity)]
			if deepOut == "" || contOut == "" {
				continue
			}
			h = HashPlanActionReconciliation(PlanActionInput{
				ValidatedDeepHash: deepOut, ContinuityContextHash: contOut, BasePublicationID: run.BasePublicationID,
				IdentityPolicy: actionreconcile.IdentityPolicyV1, VisibilityPolicy: actionreconcile.VisibilityPolicyV1,
				UserSuppressionPolicy: actionreconcile.UserSuppressionPolicyV1, RecurrencePolicy: actionreconcile.RecurrencePolicyV1,
				GroupingPolicy: actionreconcile.GroupingPolicyV1, ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.BuildPublication):
			deepOut := outByKey[string(stages.ValidateDeep)]
			planOut := outByKey[string(stages.PlanActionReconciliation)]
			univ := outByKey[string(stages.SealUniverse)]
			if deepOut == "" || planOut == "" || univ == "" {
				continue
			}
			h = HashBuildPublication(BuildPublicationInput{
				ValidatedDeepHash: deepOut, ActionPlanHash: planOut, FactualUniverseHash: univ,
				BasePublicationID: run.BasePublicationID, ContentHashPolicy: publication.ContentHashPolicyV1,
				ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.BuildProjections):
			cand := outByKey[string(stages.BuildPublication)]
			deepOut := outByKey[string(stages.ValidateDeep)]
			planOut := outByKey[string(stages.PlanActionReconciliation)]
			univ := outByKey[string(stages.SealUniverse)]
			if cand == "" || deepOut == "" || planOut == "" || univ == "" {
				continue
			}
			h = HashBuildProjections(BuildProjectionsInput{
				PublicationCandidateHash: cand, ValidatedDeepHash: deepOut, ActionPlanHash: planOut,
				FactualUniverseHash: univ, ProjectionPolicy: projection.PolicyV1,
				ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.ValidatePublication):
			cand := outByKey[string(stages.BuildPublication)]
			proj := outByKey[string(stages.BuildProjections)]
			planOut := outByKey[string(stages.PlanActionReconciliation)]
			if cand == "" || proj == "" || planOut == "" {
				continue
			}
			h = HashValidatePublication(ValidatePublicationInput{
				PublicationCandidateHash: cand, ProjectionManifestHash: proj, ActionPlanHash: planOut,
				ValidationPolicy: publication.ValidationV1, ContractBundleHash: run.ContractHash, StageVersion: s.Version,
			})
		case string(stages.Publish):
			val := outByKey[string(stages.ValidatePublication)]
			if val == "" {
				continue
			}
			h = HashPublish(PublishInputHash{ValidatedCandidateHash: val, SellerID: run.SellerID, RunID: run.ID, ContractBundleHash: run.ContractHash, StageVersion: s.Version})
		case string(stages.PostPublish):
			pub := outByKey[string(stages.Publish)]
			if pub == "" {
				continue
			}
			h = HashPostPublish(PostPublishInput{PublishOutputHash: pub, OutboxPolicy: publication.OutboxPolicyV1, ContractBundleHash: run.ContractHash, StageVersion: s.Version})
		default:
			continue
		}
		if err = RequireInputHash(h); err != nil {
			return err
		}
		if err = e.Repo.SetStageInputHash(ctx, s.ID, h); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) runClaimed(ctx context.Context, lease analysispg.Lease) error {
	run, err := e.Repo.GetRun(ctx, lease.RunID)
	if err == nil && run.CancelRequestedAt != nil {
		return e.Repo.ApplyCancelAtBoundary(ctx, lease.RunID)
	}
	h := e.Handlers[stages.Name(lease.Stage)]
	if h == nil {
		err = fmt.Errorf("runtime: STAGE_HANDLER_NOT_IMPLEMENTED:%s", lease.Stage)
		return e.Repo.FinalizeTerminal(ctx, lease, string(policy.InternalBug), err.Error())
	}
	rule := e.Policy.Retry[policy.TransientDependency]
	timeout := e.attemptTimeout(stages.Name(lease.Stage))
	if timeout <= 0 {
		timeout = rule.AttemptTimeout
	}
	if timeout <= 0 {
		timeout = rule.StageTimeout
	}
	if timeout <= 0 {
		timeout = e.Policy.LeaseDuration
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	handlerCtx, cancelHandler := context.WithCancel(runCtx)
	defer cancelHandler()
	nowFn := func() time.Time { return e.Policy.Clock.Now().UTC() }
	stageStart := time.Now()
	execClass := ""
	if err == nil {
		execClass = run.ExecutionClass
	}
	handlerCtx, span := observability.Tracer(observability.TracerAnalysis).Start(handlerCtx, "analysis.stage")
	span.SetAttributes(
		attribute.String("analysis.stage", lease.Stage),
		attribute.Int64("run_id", lease.RunID),
		attribute.String("execution_class", execClass),
	)
	defer func() {
		observability.ObserveAnalysisStage(lease.Stage, time.Since(stageStart).Seconds())
		span.End()
	}()
	hb := startLeaseHeartbeat(ctx, e.Repo, lease, e.Policy.LeaseDuration, nowFn, cancelHandler)
	out, execErr := h.Execute(handlerCtx, Work{lease.RunID, lease.StageID, lease.SellerID, lease.AttemptID, stages.Name(lease.Stage), lease.AttemptCount})
	hb.Stop()
	if hb.Lost() {
		return analysispg.ErrLeaseLost
	}
	if execErr != nil {
		if errors.Is(execErr, analysispg.ErrStalePublish) {
			return nil
		}
		return e.finalizeFailure(ctx, lease, execErr)
	}
	if out.SkipStore {
		if out.PublishID == 0 {
			status := string(CompletionSucceeded)
			if err = e.Repo.FinalizeCompleted(ctx, lease, status, nil, out.Hash, extraSuccessEvents(lease.Stage, status)...); err != nil && !errors.Is(err, analysispg.ErrLeaseLost) {
				return err
			}
			if err = e.Repo.MarkNextStageReady(ctx, lease.RunID, lease.Stage, e.implementedNext()); err != nil {
				return err
			}
		}
		_ = e.EnsureExecutableInputHashes(ctx, lease.RunID)
		if e.Enqueue != nil {
			next, nextErr := e.nextReadyStage(ctx, lease.RunID, lease.Stage)
			if nextErr == nil && next != "" {
				_ = e.Enqueue(ctx, lease.RunID, next)
			}
		}
		return nil
	}
	var artifactID *int64
	outputHash := out.Hash
	if len(out.Body) > 0 {
		a, putErr := artifacts.PutBeforeState(ctx, e.Store, artifacts.Location{SellerID: lease.SellerID, RunID: lease.RunID, Stage: lease.Stage, Kind: out.Kind}, out.Body, func(a artifacts.Artifact) error {
			attemptID := lease.AttemptID
			schema := out.SchemaVersion
			if schema == "" {
				schema = "v1"
			}
			id, saveErr := e.Repo.AddArtifactRecord(ctx, analysispg.ArtifactRecordInput{
				RunID: lease.RunID, StageID: &lease.StageID, StageAttemptID: &attemptID, StageKey: lease.Stage,
				Kind: string(out.Kind), SchemaVersion: schema, StorageKey: a.Key, SHA256: a.Hash, Bytes: a.Size,
			})
			if saveErr == nil {
				artifactID = &id
			}
			return saveErr
		})
		if putErr != nil {
			return e.finalizeFailure(ctx, lease, putErr)
		}
		outputHash = a.Hash
	} else if outputHash != "" {
		if id, ok, findErr := e.Repo.FindArtifactByHash(ctx, lease.RunID, outputHash); findErr != nil {
			return findErr
		} else if ok {
			artifactID = &id
		}
	}
	status := string(CompletionSucceeded)
	if out.CompletionClass == CompletionSucceededWithLimitations {
		status = string(CompletionSucceededWithLimitations)
	}
	if err = e.Repo.FinalizeCompleted(ctx, lease, status, artifactID, outputHash, extraSuccessEvents(lease.Stage, status)...); err != nil {
		if errors.Is(err, analysispg.ErrLeaseLost) {
			return err
		}
		// Crash-after-artifact: tolerate existing verified artifact by hash on retry finalize.
		if id, ok, findErr := e.Repo.FindArtifactByHash(ctx, lease.RunID, outputHash); findErr == nil && ok {
			artifactID = &id
			err = e.Repo.FinalizeCompleted(ctx, lease, status, artifactID, outputHash, extraSuccessEvents(lease.Stage, status)...)
		}
		if err != nil {
			return err
		}
	}
	if err = e.Repo.MarkNextStageReady(ctx, lease.RunID, lease.Stage, e.implementedNext()); err != nil {
		return err
	}
	_ = e.EnsureExecutableInputHashes(ctx, lease.RunID)
	if e.Enqueue != nil {
		next, nextErr := e.nextReadyStage(ctx, lease.RunID, lease.Stage)
		if nextErr == nil && next != "" {
			_ = e.Enqueue(ctx, lease.RunID, next) // enqueue failure must not undo success
		}
	}
	return nil
}

func (e *Executor) nextReadyStage(ctx context.Context, runID int64, completed string) (string, error) {
	stagesList, err := e.Repo.ListStages(ctx, runID)
	if err != nil {
		return "", err
	}
	var after bool
	for _, s := range stagesList {
		if s.Key == completed {
			after = true
			continue
		}
		if after && s.Status == "READY" {
			return s.Key, nil
		}
	}
	return "", nil
}

func (e *Executor) finalizeFailure(ctx context.Context, lease analysispg.Lease, execErr error) error {
	class := classify(execErr)
	code := failureCode(execErr)
	attempts, first := lease.AttemptCount, e.Policy.Clock.Now().UTC()
	if stage, err := e.Repo.GetStage(ctx, lease.RunID, lease.Stage); err == nil && stage.StartedAt != nil {
		first = *stage.StartedAt
	}
	if e.Policy.CanRetry(class, attempts, first) {
		delay := e.Policy.Delay(class, attempts)
		return e.Repo.FinalizeRetryable(ctx, lease, code, execErr.Error(), e.Policy.Clock.Now().UTC().Add(delay))
	}
	return e.Repo.FinalizeTerminal(ctx, lease, code, execErr.Error())
}

func failureCode(err error) string {
	var de *deep.Error
	if errors.As(err, &de) && de.Code != "" {
		return de.Code
	}
	return string(classify(err))
}

type RecoveryDispatcher struct {
	Enabled  bool
	Executor *Executor
	Enqueue  EnqueueFunc
}

func NewDispatcherFromEnv(executor *Executor) RecoveryDispatcher {
	enabled, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv("ANALYSIS_ENABLED")))
	return RecoveryDispatcher{Enabled: enabled, Executor: executor, Enqueue: executor.Enqueue}
}

type WorkItem struct {
	RunID, StageID, SellerID int64
	StageKey, Reason         string
}

func (d RecoveryDispatcher) Scan(ctx context.Context) ([]WorkItem, error) {
	if d.Executor == nil || d.Executor.Repo == nil {
		return nil, nil
	}
	now := d.Executor.Policy.Clock.Now().UTC()
	_, _ = d.Executor.Repo.PromoteRetryableReady(ctx, now)
	cands, err := d.Executor.Repo.ListRecoveryCandidates(ctx, now)
	if err != nil {
		return nil, err
	}
	out := make([]WorkItem, 0, len(cands))
	for _, c := range cands {
		out = append(out, WorkItem{RunID: c.RunID, StageID: c.StageID, SellerID: c.SellerID, StageKey: c.StageKey, Reason: c.Reason})
	}
	return out, nil
}

func (d RecoveryDispatcher) DispatchRecovery(ctx context.Context) (bool, error) {
	if !d.Enabled {
		return false, nil
	}
	items, err := d.Scan(ctx)
	if err != nil {
		return false, err
	}
	if len(items) == 0 {
		return false, nil
	}
	item := items[0]
	enqueue := d.Enqueue
	if enqueue == nil {
		enqueue = d.Executor.Enqueue
	}
	if enqueue != nil {
		if err = enqueue(ctx, item.RunID, item.StageKey); err != nil {
			return true, d.Executor.ExecuteStage(ctx, item.RunID, item.StageKey)
		}
		return true, nil
	}
	return true, d.Executor.ExecuteStage(ctx, item.RunID, item.StageKey)
}

type NoOpDetector struct{ Repo *analysispg.Repository }

func (n NoOpDetector) Prior(ctx context.Context, seller int64, universeHash, contractHash, intentHash string) (int64, bool, error) {
	return n.Repo.PriorSuccessfulProduction(ctx, seller, universeHash, contractHash, intentHash)
}

type ClassifiedError interface{ FailureClass() policy.FailureClass }

func classify(err error) policy.FailureClass {
	var classified ClassifiedError
	if errors.As(err, &classified) {
		return classified.FailureClass()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return policy.Timeout
	}
	if errors.Is(err, evidence.ErrIntegrity) {
		return policy.ContractViolation
	}
	if errors.Is(err, analysispg.ErrLeaseLost) {
		return policy.LeaseExpired
	}
	return policy.TransientDependency
}

// FenceMatches is shared by in-memory tests and mirrors every fenced UPDATE.
func FenceMatches(currentToken string, currentVersion int64, lease analysispg.Lease) bool {
	return currentToken == lease.Token && currentVersion == lease.Version
}

// CanTransition exposes stage transition policy for tests and callers.
func CanTransition(from, to string) error { return analysispg.ValidateStageTransition(from, to) }

// StandardIntentHash is deterministic for STANDARD_DAILY (no request timestamps).
func StandardIntentHash() string {
	return canonicaljson.Hash(map[string]any{"intent": "STANDARD_DAILY", "version": "v1"})
}

func extraSuccessEvents(stage, status string) []analysispg.RunEventInput {
	if stage == string(stages.PostPublish) && (status == string(CompletionSucceeded) || status == string(CompletionSucceededWithLimitations)) {
		return []analysispg.RunEventInput{{
			Kind:    "IMPLEMENTED_FRONTIER_REACHED",
			Payload: map[string]any{"last_stage": string(stages.PostPublish)},
		}}
	}
	return nil
}

func (e *Executor) attemptTimeout(stage stages.Name) time.Duration {
	class := "stage_default"
	if d, ok := e.Registry.Lookup(stage); ok && d.TimeoutPolicy != "" {
		class = d.TimeoutPolicy
	}
	return e.Policy.TimeoutFor(class)
}
