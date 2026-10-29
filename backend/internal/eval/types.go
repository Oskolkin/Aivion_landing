package eval

import (
	"encoding/json"
	"time"
)

const (
	SplitDEV         = "DEV"
	SplitHoldout     = "HOLDOUT"
	SplitCalibration = "CALIBRATION"
	SplitReplay      = "REPLAY"

	KindFactualGolden     = "FACTUAL_GOLDEN"
	KindCapabilityMatrix  = "CAPABILITY_MATRIX"
	KindAIGolden          = "AI_GOLDEN"
	KindLongitudinal      = "LONGITUDINAL"
	KindChatGolden        = "CHAT_GOLDEN"
	KindAttachmentGolden  = "ATTACHMENT_GOLDEN"
	KindRealSellerReplay  = "REAL_SELLER_REPLAY"
	KindGraderCalibration = "GRADER_CALIBRATION"

	VerdictPass                 = "PASS"
	VerdictPassKnownLimitations = "PASS_WITH_KNOWN_LIMITATIONS"
	VerdictFail                 = "FAIL"
	VerdictPending              = "PENDING"

	StatusHumanReviewPending = "HUMAN_REVIEW_PENDING"
	StatusDatasetPending     = "DATASET_PENDING"
	StatusBudgetExceeded     = "BUDGET_EXCEEDED"

	LabelMustInclude      = "MUST_INCLUDE"
	LabelShouldInclude    = "SHOULD_INCLUDE"
	LabelOptional         = "OPTIONAL"
	LabelShouldNotInclude = "SHOULD_NOT_INCLUDE"

	ThreadMandatory  = "MANDATORY_THREAD"
	ThreadAcceptable = "ACCEPTABLE_THREAD"
	ThreadForbidden  = "FORBIDDEN_THREAD"

	ModelIdentityMismatch = "MODEL_IDENTITY_MISMATCH"
	DeterminismViolation  = "DETERMINISM_VIOLATION"
	EvalBudgetExceeded    = "EVAL_BUDGET_EXCEEDED"

	ArtifactNS = "eval/"

	HoldoutSentinel = "SECRET_HOLDOUT_LABEL_7F83"

	PurposeAcceptance  = "ACCEPTANCE"
	PurposeHoldout     = "HOLDOUT"
	PurposeLive        = "LIVE"
	PurposeBaseline    = "BASELINE"
	PurposeUnitFixture = "UNIT_FIXTURE"
)

type Suite struct {
	ID                   int64
	SuiteKey             string
	Version              string
	DatasetKind          string
	Description          string
	RubricVersion        string
	ManifestArtifactKey  string
	ManifestArtifactHash string
	CreatedAt            time.Time
}

type Case struct {
	ID                  int64
	CaseKey             string
	Version             string
	FixtureArtifactKey  string
	FixtureArtifactHash string
	AnalysisDate        *time.Time
	AnalysisCutoffAt    *time.Time
	Expected            Expected
	Tags                []string
}

type Expected struct {
	Invariants           []string              `json:"invariants,omitempty"`
	MandatoryThemes      []string              `json:"mandatory_themes,omitempty"`
	ForbiddenConclusions []string              `json:"forbidden_conclusions,omitempty"`
	ExpectedEntities     []string              `json:"expected_entities,omitempty"`
	ActionConstraints    []string              `json:"action_constraints,omitempty"`
	ChatScript           *ChatScript           `json:"chat_script,omitempty"`
	Sales                *SalesExpectation     `json:"sales,omitempty"`
	DRR                  *DRRExpectation       `json:"drr,omitempty"`
	Stock                *StockExpectation     `json:"stock,omitempty"`
	Funnel               *FunnelExpectation    `json:"funnel,omitempty"`
	Materiality          []MaterialLabel       `json:"materiality,omitempty"`
	Discovery            *DiscoveryExpectation `json:"discovery,omitempty"`
	DeepTraps            []string              `json:"deep_traps,omitempty"`
	Trajectory           *TrajectorySpec       `json:"trajectory,omitempty"`
	DatasetPending       bool                  `json:"dataset_pending,omitempty"`
	HoldoutSentinel      string                `json:"holdout_sentinel,omitempty"`
}

type ChatScript struct {
	Turns                []string `json:"turns,omitempty"`
	RequiredTools        []string `json:"required_tools,omitempty"`
	ForbiddenTools       []string `json:"forbidden_tools,omitempty"`
	RequireClarification bool     `json:"require_clarification,omitempty"`
}

type TrajectorySpec struct {
	Required   []string   `json:"required,omitempty"`
	Forbidden  []string   `json:"forbidden,omitempty"`
	Equivalent [][]string `json:"equivalent,omitempty"`
}

type SalesExpectation struct {
	OrderedAmount string `json:"ordered_amount"`
	OrderedUnits  string `json:"ordered_units"`
}

type DRRExpectation struct {
	AccountDRRPct         string `json:"account_drr_pct"`
	ForbidAvgDaily        bool   `json:"forbid_avg_daily"`
	ForbidAttributedDenom bool   `json:"forbid_attributed_denom"`
}

type StockExpectation struct {
	StockZeroDemandPositiveCoverageZero bool `json:"stock_zero_demand_positive_coverage_zero"`
	NoDemandCoverageNull                bool `json:"no_demand_coverage_null"`
	StockUnavailableCoverageNull        bool `json:"stock_unavailable_coverage_null"`
	InboundDoesNotIncreaseStock         bool `json:"inbound_does_not_increase_stock"`
}

type FunnelExpectation struct {
	MissingStageNotInvented bool   `json:"missing_stage_not_invented"`
	IncompatibleChain       string `json:"incompatible_chain,omitempty"`
	PremiumUnavailable      string `json:"premium_unavailable,omitempty"`
	PartialChain            string `json:"partial_chain,omitempty"`
	ValidZeroActivity       string `json:"valid_zero_activity,omitempty"`
	ConversionDeltaPP       string `json:"conversion_delta_pp,omitempty"`
}

type MaterialLabel struct {
	Kind  string `json:"kind"`
	Key   string `json:"key"`
	Label string `json:"label"`
}

type DiscoveryExpectation struct {
	Threads           []ThreadLabel `json:"threads,omitempty"`
	AllowNoMeaningful bool          `json:"allow_no_meaningful,omitempty"`
}

type ThreadLabel struct {
	ID    string `json:"id"`
	Class string `json:"class"`
}

type CaseInput struct {
	CaseKey          string     `json:"case_key"`
	Tags             []string   `json:"tags,omitempty"`
	Fixture          Fixture    `json:"fixture"`
	Question         string     `json:"question,omitempty"`
	AnalysisDate     *time.Time `json:"analysis_date,omitempty"`
	AnalysisCutoffAt *time.Time `json:"analysis_cutoff_at,omitempty"`
}

type Fixture struct {
	SalesDays               []SalesDay `json:"sales_days,omitempty"`
	PaidSpend               string     `json:"paid_spend,omitempty"`
	OrderedAmountForDRR     string     `json:"ordered_amount_for_drr,omitempty"`
	DailyDRR                []string   `json:"daily_drr,omitempty"`
	AttributedOrderedAmount string     `json:"attributed_ordered_amount,omitempty"`
	StockOOSDemand          bool       `json:"stock_oos_demand,omitempty"`
	StockNoDemand           bool       `json:"stock_no_demand,omitempty"`
	StockUnavailable        bool       `json:"stock_unavailable,omitempty"`
	InboundUnits            string     `json:"inbound_units,omitempty"`
	OnHandUnits             string     `json:"on_hand_units,omitempty"`
	FunnelCurrentConv       string     `json:"funnel_current_conv,omitempty"`
	FunnelPrevConv          string     `json:"funnel_prev_conv,omitempty"`
	FunnelPremiumAvailable  bool       `json:"funnel_premium_available,omitempty"`
	FunnelPartial           bool       `json:"funnel_partial,omitempty"`
	FunnelZeroActivity      bool       `json:"funnel_zero_activity,omitempty"`
	FunnelIncompatible      bool       `json:"funnel_incompatible,omitempty"`
	CapabilityState         string     `json:"capability_state,omitempty"`
	MaterialSelected        []string   `json:"material_selected,omitempty"`
	DiscoveryThreads        []string   `json:"discovery_threads,omitempty"`
	DeepFacts               string     `json:"deep_facts,omitempty"`
	ChatToolsFromQuestion   bool       `json:"chat_tools_from_question,omitempty"`
}

type SalesDay struct {
	OrderedAmount string `json:"ordered_amount"`
	OrderedUnits  string `json:"ordered_units"`
}

type ExecuteOptions struct {
	Live    bool
	Budget  *Budget
	Purpose string
	Split   string
}

type CaseActual struct {
	Values             map[string]any
	Output             string
	Tools              []string
	Threads            []string
	ProviderPrompt     string
	ProviderRequests   int
	InputTokens        int64
	OutputTokens       int64
	ProviderRequestIDs []string
	Model              string
	LatencyMS          int64
	AnalysisRunID      *int64
	Hashes             map[string]string
	NoOp               bool
	DiscoveryCalls     int
	DeepCalls          int
	ArtifactRefs       map[string]string
}

type SuiteCase struct {
	SuiteID  int64
	CaseID   int64
	Split    string
	Ordinal  int
	Critical bool
	Input    CaseInput
	Case     Case
}

type Run struct {
	ID                       int64
	SuiteID                  int64
	Split                    string
	Purpose                  string
	Status                   string
	Verdict                  string
	CodeSHA                  string
	UCDMVersion              string
	FactualAPIVersion        string
	TemporalPolicyVersion    string
	CapabilityPolicyVersion  string
	MaterialityPolicyVersion string
	DiscoveryPromptVersion   string
	DiscoverySchemaVersion   string
	DiscoveryModelPolicy     string
	EvidenceCatalogVersion   string
	DeepPromptVersion        string
	DeepSchemaVersion        string
	DeepModelPolicy          string
	ActionCatalogVersion     string
	ActionContinuityPolicy   string
	ProjectionVersion        string
	ChatPromptVersion        string
	ChatToolPolicyVersion    string
	ChatModelPolicyVersion   string
	ContractBundleHash       string
	ResolvedModels           ResolvedModels
	ReasoningConfig          json.RawMessage
	GraderVersions           json.RawMessage
	StartedAt                time.Time
	CompletedAt              *time.Time
	CriticalFailuresCount    int
	Aggregate                Aggregate
	ReportArtifactKey        string
	ReportArtifactHash       string
	Limitations              []KnownLimitation
}

type ResolvedModels struct {
	Discovery ConfiguredActualModel `json:"discovery"`
	Deep      ConfiguredActualModel `json:"deep"`
	Chat      ConfiguredActualModel `json:"chat"`
}

type ConfiguredActualModel struct {
	Configured      string `json:"configured"`
	Actual          string `json:"actual,omitempty"`
	PolicyVersion   string `json:"policy_version,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	Verbosity       string `json:"verbosity,omitempty"`
	MaxOutput       int    `json:"max_output,omitempty"`
	TimeoutSeconds  int    `json:"timeout_seconds,omitempty"`
	Mismatch        bool   `json:"mismatch,omitempty"`
	MismatchCode    string `json:"mismatch_code,omitempty"`
}

type Aggregate struct {
	FactualPassRate        *float64 `json:"factual_pass_rate,omitempty"`
	MaterialityCritical    *float64 `json:"materiality_must_include_recall,omitempty"`
	MaterialityOverall     *float64 `json:"materiality_overall_recall,omitempty"`
	DiscoveryCritical      *float64 `json:"discovery_critical_mandatory_recall,omitempty"`
	DiscoveryOverall       *float64 `json:"discovery_overall_mandatory_recall,omitempty"`
	DiscoveryUnsupported   *int     `json:"discovery_unsupported_critical,omitempty"`
	DiscoveryDuplicateRate *float64 `json:"discovery_duplicate_rate,omitempty"`
	ActionAcceptability    *float64 `json:"action_acceptability,omitempty"`
	HarmfulActions         *int     `json:"harmful_actions,omitempty"`
	InputTokens            int64    `json:"input_tokens"`
	OutputTokens           int64    `json:"output_tokens"`
	CachedTokens           int64    `json:"cached_tokens"`
	EstimatedCostUSD       *float64 `json:"estimated_cost_usd,omitempty"`
	PricingVersion         string   `json:"pricing_version,omitempty"`
}

type KnownLimitation struct {
	Limitation string `json:"limitation"`
	Suite      string `json:"suite"`
	Domain     string `json:"domain"`
	WhyNotCrit string `json:"why_not_critical"`
	Owner      string `json:"owner"`
	FollowUp   string `json:"follow_up"`
}

type CaseRun struct {
	ID               int64
	EvalRunID        int64
	CaseID           int64
	Split            string
	Status           string
	Verdict          string
	AnalysisRunID    *int64
	InputTokens      int64
	OutputTokens     int64
	CachedTokens     int64
	ReasoningTokens  int64
	EstimatedCostUSD *float64
	PricingVersion   string
	LatencyMS        int64
	ArtifactKey      string
	ArtifactHash     string
	ErrorCode        string
}

type GraderResult struct {
	CaseRunID        int64
	GraderKey        string
	GraderVersion    string
	GraderPromptHash string
	GraderModel      string
	RubricVersion    string
	Score            map[string]any
	CriticalFail     bool
	ArtifactKey      string
	ArtifactHash     string
}

type HumanReview struct {
	ID                 int64
	CaseRunID          int64
	Reviewer           string
	Status             string
	Ratings            map[string]any
	Notes              string
	PacketArtifactKey  string
	PacketArtifactHash string
}

type CompareResult struct {
	HardDeterministic  []DiffItem `json:"hard_deterministic"`
	Semantic           []DiffItem `json:"semantic"`
	Latency            []DiffItem `json:"latency"`
	Tokens             []DiffItem `json:"tokens"`
	Cost               []DiffItem `json:"cost"`
	CriticalRegression bool       `json:"critical_regression"`
}

type DiffItem struct {
	Name      string `json:"name"`
	Baseline  any    `json:"baseline"`
	Candidate any    `json:"candidate"`
	Severity  string `json:"severity"`
}
