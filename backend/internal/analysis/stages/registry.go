package stages

type Name string

const (
	ResolveContext           Name = "RESOLVE_CONTEXT"
	SealUniverse             Name = "SEAL_UNIVERSE"
	PrepareContinuity        Name = "PREPARE_CONTINUITY"
	BuildLandscapeCore       Name = "BUILD_LANDSCAPE_CORE"
	RunMateriality           Name = "RUN_MATERIALITY"
	BuildLandscapeDetail     Name = "BUILD_LANDSCAPE_DETAIL"
	BuildDiscoveryPack       Name = "BUILD_DISCOVERY_PACK"
	DiscoveryAI              Name = "DISCOVERY_AI"
	ValidateDiscovery        Name = "VALIDATE_DISCOVERY"
	ExpandEvidence           Name = "EXPAND_EVIDENCE"
	BuildDeepPack            Name = "BUILD_DEEP_PACK"
	DeepAI                   Name = "DEEP_AI"
	ValidateDeep             Name = "VALIDATE_DEEP"
	PlanActionReconciliation Name = "PLAN_ACTION_RECONCILIATION"
	BuildPublication         Name = "BUILD_PUBLICATION"
	BuildProjections         Name = "BUILD_PROJECTIONS"
	ValidatePublication      Name = "VALIDATE_PUBLICATION"
	Publish                  Name = "PUBLISH"
	PostPublish              Name = "POST_PUBLISH"
)

type ExecutionType string

const (
	Deterministic            ExecutionType = "DETERMINISTIC"
	ExternalNondeterministic ExecutionType = "EXTERNAL_NONDETERMINISTIC"
	TransactionalSideEffect  ExecutionType = "TRANSACTIONAL_SIDE_EFFECT"
	AsyncPostCommit          ExecutionType = "ASYNC_POST_COMMIT"
)

type Definition struct {
	Name                      Name
	Ordinal                   int
	StageVersion              string
	ExecutionType             ExecutionType
	ArtifactRequirement       string
	RetryPolicyClass          string
	TimeoutPolicy             string
	ImplementedInCurrentBuild bool
}

type Registry struct {
	ordered []Definition
	byName  map[Name]Definition
}

func New(defs []Definition) Registry {
	r := Registry{ordered: append([]Definition(nil), defs...), byName: make(map[Name]Definition, len(defs))}
	for _, d := range defs {
		r.byName[d.Name] = d
	}
	return r
}

func (r Registry) Ordered() []Definition { return append([]Definition(nil), r.ordered...) }
func (r Registry) Lookup(n Name) (Definition, bool) {
	d, ok := r.byName[n]
	return d, ok
}
func (r Registry) Count() int { return len(r.ordered) }

func def(name Name, ordinal int, exec ExecutionType, artifact, retry, timeout string, implemented bool) Definition {
	return Definition{
		Name: name, Ordinal: ordinal, StageVersion: "v1", ExecutionType: exec,
		ArtifactRequirement: artifact, RetryPolicyClass: retry, TimeoutPolicy: timeout,
		ImplementedInCurrentBuild: implemented,
	}
}

// ProductionRegistry enumerates all 19 normative stages. Prompt №12 implements
// stages 1–13; PLAN_ACTION_RECONCILIATION (14+) remains NOT_IMPLEMENTED.
var ProductionRegistry = New([]Definition{
	def(ResolveContext, 1, Deterministic, "RESOLVED_CONTEXT_MANIFEST", "deterministic", "stage_default", true),
	def(SealUniverse, 2, Deterministic, "FACTUAL_UNIVERSE_MANIFEST", "deterministic", "stage_default", true),
	def(PrepareContinuity, 3, Deterministic, "OPTIONAL", "deterministic", "stage_default", true),
	def(BuildLandscapeCore, 4, Deterministic, "LANDSCAPE_CORE", "deterministic", "stage_default", true),
	def(RunMateriality, 5, Deterministic, "MATERIALITY", "deterministic", "stage_default", true),
	def(BuildLandscapeDetail, 6, Deterministic, "LANDSCAPE_DETAIL", "deterministic", "stage_default", true),
	def(BuildDiscoveryPack, 7, Deterministic, "DISCOVERY_PACK", "deterministic", "stage_default", true),
	def(DiscoveryAI, 8, ExternalNondeterministic, "DISCOVERY_OUTPUT", "ai", "ai_default", true),
	def(ValidateDiscovery, 9, Deterministic, "DISCOVERY_VALIDATION", "deterministic", "stage_default", true),
	def(ExpandEvidence, 10, Deterministic, "EVIDENCE_BUNDLE", "deterministic", "stage_default", true),
	def(BuildDeepPack, 11, Deterministic, "DEEP_PACK", "deterministic", "stage_default", true),
	def(DeepAI, 12, ExternalNondeterministic, "DEEP_OUTPUT", "ai", "ai_deep", true),
	def(ValidateDeep, 13, Deterministic, "DEEP_VALIDATION", "deterministic", "stage_default", true),
	def(PlanActionReconciliation, 14, Deterministic, "ACTION_PLAN", "deterministic", "stage_default", true),
	def(BuildPublication, 15, Deterministic, "PUBLICATION_CANDIDATE", "deterministic", "stage_default", true),
	def(BuildProjections, 16, Deterministic, "PROJECTIONS", "deterministic", "stage_default", true),
	def(ValidatePublication, 17, Deterministic, "PUBLICATION_VALIDATION", "deterministic", "stage_default", true),
	def(Publish, 18, TransactionalSideEffect, "PUBLICATION", "transactional", "stage_default", true),
	def(PostPublish, 19, AsyncPostCommit, "POST_PUBLISH", "async", "async_default", true),
})

// ProductionImplementedStageNames is the only stage chain Prompt №10 may schedule.
func ProductionImplementedStageNames() []string {
	return []string{
		string(ResolveContext), string(SealUniverse), string(PrepareContinuity),
		string(BuildLandscapeCore), string(RunMateriality), string(BuildLandscapeDetail),
		string(BuildDiscoveryPack), string(DiscoveryAI), string(ValidateDiscovery),
		string(ExpandEvidence), string(BuildDeepPack), string(DeepAI), string(ValidateDeep),
		string(PlanActionReconciliation), string(BuildPublication), string(BuildProjections),
		string(ValidatePublication), string(Publish), string(PostPublish),
	}
}

// TestRegistry is intentionally separate and must never be used by production wiring.
var TestRegistry = New([]Definition{
	def("TEST_STAGE_A", 1, Deterministic, "TEST_STAGE_OUTPUT", "deterministic", "stage_default", true),
	def("TEST_STAGE_B", 2, Deterministic, "TEST_STAGE_OUTPUT", "deterministic", "stage_default", true),
})
