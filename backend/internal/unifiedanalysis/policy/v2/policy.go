// Package policyv2 is the machine-readable business-rule registry for
// Aivion Unified Analysis "Part 1" (analysis_policy_v2), paired with the
// findings-first aivion_unified_analysis_v11 output contract and the
// unified_analyst_v1_32 prompt.
//
// This package is intentionally NOT wired into the production Unified
// Analysis pipeline: production packs still ship analysis_policy.json
// (contract.AnalysisPolicyV1, see backend/internal/unifiedanalysis/pack)
// and production thresholds still come from
// backend/internal/unifiedanalysis/thresholdpolicy. analysis_policy_v2
// exists for prompt/schema-v11 preparation, tests, and evals only.
package policyv2

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/lineendings"
)

// Version is the policy_version identifier stored on this registry and on
// any AI output that cites it.
const Version = "analysis_policy_v2"

// CanonicalPolicyHashV2 is the immutable LF-normalized sha256 digest of the
// embedded analysis_policy_v2.json bytes, in the same "sha256:<hex>" form as
// contract.CanonicalSchemaHashV10. Computed once via TestPolicyHash_Pinned
// and pinned here; changing the JSON file requires deliberately updating
// this constant (never edit the JSON without recomputing the hash).
const CanonicalPolicyHashV2 = "sha256:35f92ae0ba05cd4bdc08423472cd4a89b6d71d1dbd6b8bdaf9487456fa6d9063"

//go:embed analysis_policy_v2.json
var policyJSON []byte

var (
	loadOnce   sync.Once
	loaded     *Policy
	loadErr    error
	hashCached string
)

// FindingTypeEntry is one entry in finding_type_registry.
type FindingTypeEntry struct {
	FindingType         string   `json:"finding_type"`
	Domain              string   `json:"domain"`
	AllowedDirections   []string `json:"allowed_directions"`
	AllowedSeverities   []string `json:"allowed_severities"`
	AllowedScopes       []string `json:"allowed_scopes"`
	RequiredMetricCodes []string `json:"required_metric_codes"`
	Actionable          bool     `json:"actionable"`
}

// ActionTypeEntry is one entry in action_type_registry.
type ActionTypeEntry struct {
	ActionType           string   `json:"action_type"`
	Category             string   `json:"category"`
	Domain               string   `json:"domain"`
	AllowedScopeTypes    []string `json:"allowed_scope_types"`
	RequiredFindingTypes []string `json:"required_finding_types"`
}

// ActionLimits mirrors contract.MaxActionsV1 / MaxTopActionsV1 /
// MinActionsMaterialV1 / MinRequiredActionsMaterialV1 in policy form.
type ActionLimits struct {
	MaxActions          int `json:"max_actions"`
	MaxTopActions       int `json:"max_top_actions"`
	MinActionsMaterial  int `json:"min_actions_material"`
	MinRequiredMaterial int `json:"min_required_material"`
}

// BlockStatusRules describes which Dashboard blocks exist and how their
// status (available/limited/unavailable) is derived from data coverage.
type BlockStatusRules struct {
	Blocks                 []string `json:"blocks"`
	UnavailableWhenNoData  bool     `json:"unavailable_when_no_data"`
	LimitedWhenPartialData bool     `json:"limited_when_partial_data"`
}

// SectionDomainRule restricts which finding scope_types may ground a given
// Dashboard block (section_fact_policy analogue for v11).
type SectionDomainRule struct {
	AllowedScopeTypes   []string `json:"allowed_scope_types"`
	ForbiddenScopeTypes []string `json:"forbidden_scope_types,omitempty"`
}

// MetricRenderingRule maps a metric_code to its UnifiedMetricClaimV11
// render_format / delta_mode.
type MetricRenderingRule struct {
	RenderFormat string `json:"render_format"`
	DeltaMode    string `json:"delta_mode"`
}

// ThresholdEntry is one policy-backed comparative threshold. Metric code /
// operator / value must match the production thresholdpolicy.Registry
// entry with the same PolicyID (see policy_test.go).
type ThresholdEntry struct {
	PolicyID          string   `json:"policy_id"`
	MetricCode        string   `json:"metric_code"`
	Operator          string   `json:"operator"`
	Value             float64  `json:"value"`
	Unit              string   `json:"unit"`
	AllowedScopeTypes []string `json:"allowed_scope_types"`
}

// CompletionRules describes how completion_conditions targets must relate
// to their baseline (must be unmet at analysis time).
type CompletionRules struct {
	TargetMustBeUnmetAtAnalysisTime bool              `json:"target_must_be_unmet_at_analysis_time"`
	OperatorDirection               map[string]string `json:"operator_direction"`
}

// PriorityConstraints maps Finding severity to Action priority and sets the
// minimum count of execution_tier=required Actions for material outcomes.
type PriorityConstraints struct {
	MinRequiredExecutionTierCount int               `json:"min_required_execution_tier_count"`
	SeverityToPriority            map[string]string `json:"severity_to_priority"`
}

// ScopeRules governs entity_ids ownership/emptiness per scope_type.
type ScopeRules struct {
	SKUSingleActionOwnership               bool `json:"sku_single_action_ownership"`
	CampaignSingleActionOwnership          bool `json:"campaign_single_action_ownership"`
	AccountScopeAllowsEmptyEntityIDs       bool `json:"account_scope_allows_empty_entity_ids"`
	SKUScopeRequiresNonEmptyEntityIDs      bool `json:"sku_scope_requires_nonempty_entity_ids"`
	CampaignScopeRequiresNonEmptyEntityIDs bool `json:"campaign_scope_requires_nonempty_entity_ids"`
}

// OverlapRules governs Action/Finding deduplication and ownership.
type OverlapRules struct {
	SKUMaxActions          int      `json:"sku_max_actions"`
	FindingMaxPrimaryBlock int      `json:"finding_max_primary_block"`
	DedupeKey              []string `json:"dedupe_key"`
}

// LimitationEntry is one entry in limitation_registry (superset of the Pack
// v1 limitation codes plus new v2 codes: future_supply_visibility_limited,
// history_missing, source_incomplete, data_stale).
type LimitationEntry struct {
	Code           string   `json:"code"`
	Scope          string   `json:"scope"`
	Severity       string   `json:"severity"`
	Description    string   `json:"description"`
	AffectedTables []string `json:"affected_tables,omitempty"`
	AffectedBlocks []string `json:"affected_blocks,omitempty"`
}

// ChangeGroupRules governs the four fixed what_changed groups.
type ChangeGroupRules struct {
	Groups                     []string `json:"groups"`
	ChangeIDSingleGroup        bool     `json:"change_id_single_group"`
	EmptyGroupTextRequiredWhen []string `json:"empty_group_text_required_when"`
}

// CategoryLimits bounds dashboard.category_insights.items.
type CategoryLimits struct {
	MaxItems int `json:"max_items"`
}

// CausalityRules bans unconfirmed causal claims and requires every Action /
// numeric assertion to be grounded.
type CausalityRules struct {
	ForbiddenCausalClaims                      []string `json:"forbidden_causal_claims"`
	RequireFindingForEveryAction               bool     `json:"require_finding_for_every_action"`
	RequireMetricClaimForEveryNumericAssertion bool     `json:"require_metric_claim_for_every_numeric_assertion"`
}

// Policy is the fully typed decode of analysis_policy_v2.json.
type Policy struct {
	PolicyVersion         string                         `json:"policy_version"`
	FindingTypeRegistry   []FindingTypeEntry             `json:"finding_type_registry"`
	ActionTypeRegistry    []ActionTypeEntry              `json:"action_type_registry"`
	ActionTypeToCategory  map[string]string              `json:"action_type_to_category"`
	ActionLimits          ActionLimits                   `json:"action_limits"`
	TextLimits            map[string]int                 `json:"text_limits"`
	BlockStatusRules      BlockStatusRules               `json:"block_status_rules"`
	SectionDomainRules    map[string]SectionDomainRule   `json:"section_domain_rules"`
	MetricRenderingRules  map[string]MetricRenderingRule `json:"metric_rendering_rules"`
	Thresholds            []ThresholdEntry               `json:"thresholds"`
	CompletionRules       CompletionRules                `json:"completion_rules"`
	PriorityConstraints   PriorityConstraints            `json:"priority_constraints"`
	ScopeRules            ScopeRules                     `json:"scope_rules"`
	OverlapRules          OverlapRules                   `json:"overlap_rules"`
	LimitationRegistry    []LimitationEntry              `json:"limitation_registry"`
	ChangeGroupRules      ChangeGroupRules               `json:"change_group_rules"`
	SKUFocusLimits        map[string]int                 `json:"sku_focus_limits"`
	CategoryLimits        CategoryLimits                 `json:"category_limits"`
	RecommendedCheckTypes map[string][]string            `json:"recommended_check_types"`
	CausalityRules        CausalityRules                 `json:"causality_rules"`
	UserFacingTextFields  []string                       `json:"user_facing_text_fields"`
}

// validOperators are the comparative operators accepted in thresholds[].operator.
var validOperators = map[string]bool{
	">": true, ">=": true, "<": true, "<=": true, "==": true,
}

// Load parses, validates, and caches the embedded analysis_policy_v2.json.
// Safe for concurrent use; the embedded bytes never change at runtime, so
// the result is memoized after the first successful call.
func Load() (*Policy, error) {
	loadOnce.Do(func() {
		var p Policy
		if err := json.Unmarshal(policyJSON, &p); err != nil {
			loadErr = fmt.Errorf("policyv2: parse analysis_policy_v2.json: %w", err)
			return
		}
		if err := p.Validate(); err != nil {
			loadErr = fmt.Errorf("policyv2: validate analysis_policy_v2.json: %w", err)
			return
		}
		loaded = &p
		normalized := lineendings.NormalizeLineEndingsBytes(policyJSON)
		sum := sha256.Sum256(normalized)
		hashCached = "sha256:" + hex.EncodeToString(sum[:])
	})
	return loaded, loadErr
}

// MustLoad panics if the embedded policy fails to load/validate. Intended
// for tests and package-init-time wiring only.
func MustLoad() *Policy {
	p, err := Load()
	if err != nil {
		panic(err)
	}
	return p
}

// Hash returns the stable "sha256:<hex>" digest of the embedded
// analysis_policy_v2.json bytes after LF normalization. Load() must
// succeed at least once (directly or transitively) before calling Hash.
func Hash() (string, error) {
	if _, err := Load(); err != nil {
		return "", err
	}
	return hashCached, nil
}

// Validate checks internal structural consistency of the policy document.
// It does NOT compare against production registries (thresholdpolicy,
// contract.AllowedActionTypesV1, actions.ActionCategoryForType) — that
// cross-check belongs to policy_test.go so this package has no hard
// dependency on the actions package.
func (p *Policy) Validate() error {
	if p.PolicyVersion != Version {
		return fmt.Errorf("policy_version=%q want=%q", p.PolicyVersion, Version)
	}

	if len(p.FindingTypeRegistry) == 0 {
		return fmt.Errorf("finding_type_registry must not be empty")
	}
	seenFindingTypes := make(map[string]bool, len(p.FindingTypeRegistry))
	for _, ft := range p.FindingTypeRegistry {
		if ft.FindingType == "" {
			return fmt.Errorf("finding_type_registry: empty finding_type")
		}
		if seenFindingTypes[ft.FindingType] {
			return fmt.Errorf("finding_type_registry: duplicate finding_type %q", ft.FindingType)
		}
		seenFindingTypes[ft.FindingType] = true
		if ft.Domain == "" {
			return fmt.Errorf("finding_type_registry[%s]: empty domain", ft.FindingType)
		}
		if len(ft.AllowedDirections) == 0 {
			return fmt.Errorf("finding_type_registry[%s]: allowed_directions must not be empty", ft.FindingType)
		}
		if len(ft.AllowedSeverities) == 0 {
			return fmt.Errorf("finding_type_registry[%s]: allowed_severities must not be empty", ft.FindingType)
		}
		if len(ft.AllowedScopes) == 0 {
			return fmt.Errorf("finding_type_registry[%s]: allowed_scopes must not be empty", ft.FindingType)
		}
	}

	if len(p.ActionTypeRegistry) == 0 {
		return fmt.Errorf("action_type_registry must not be empty")
	}
	seenActionTypes := make(map[string]bool, len(p.ActionTypeRegistry))
	for _, at := range p.ActionTypeRegistry {
		if at.ActionType == "" {
			return fmt.Errorf("action_type_registry: empty action_type")
		}
		if seenActionTypes[at.ActionType] {
			return fmt.Errorf("action_type_registry: duplicate action_type %q", at.ActionType)
		}
		seenActionTypes[at.ActionType] = true
		if at.Category == "" {
			return fmt.Errorf("action_type_registry[%s]: empty category", at.ActionType)
		}
		if len(at.RequiredFindingTypes) == 0 {
			return fmt.Errorf("action_type_registry[%s]: required_finding_types must not be empty", at.ActionType)
		}
		for _, ftype := range at.RequiredFindingTypes {
			if !seenFindingTypes[ftype] {
				return fmt.Errorf("action_type_registry[%s]: required_finding_type %q not in finding_type_registry", at.ActionType, ftype)
			}
		}
		category, ok := p.ActionTypeToCategory[at.ActionType]
		if !ok {
			return fmt.Errorf("action_type_registry[%s]: missing from action_type_to_category", at.ActionType)
		}
		if category != at.Category {
			return fmt.Errorf("action_type_registry[%s]: category=%q action_type_to_category=%q mismatch", at.ActionType, at.Category, category)
		}
	}
	// Every action_type_to_category key must be a registered action type
	// (no orphan mappings) and vice versa (checked above).
	for actionType := range p.ActionTypeToCategory {
		if !seenActionTypes[actionType] {
			return fmt.Errorf("action_type_to_category[%s]: not present in action_type_registry", actionType)
		}
	}

	if p.ActionLimits.MaxActions <= 0 {
		return fmt.Errorf("action_limits.max_actions must be positive")
	}
	if p.ActionLimits.MaxTopActions <= 0 || p.ActionLimits.MaxTopActions > p.ActionLimits.MaxActions {
		return fmt.Errorf("action_limits.max_top_actions must be positive and <= max_actions")
	}
	if p.ActionLimits.MinActionsMaterial < 0 || p.ActionLimits.MinRequiredMaterial < 0 {
		return fmt.Errorf("action_limits: material floors must be >= 0")
	}

	if len(p.Thresholds) == 0 {
		return fmt.Errorf("thresholds must not be empty")
	}
	seenPolicyIDs := make(map[string]bool, len(p.Thresholds))
	for _, th := range p.Thresholds {
		if th.PolicyID == "" {
			return fmt.Errorf("thresholds: empty policy_id")
		}
		if seenPolicyIDs[th.PolicyID] {
			return fmt.Errorf("thresholds: duplicate policy_id %q", th.PolicyID)
		}
		seenPolicyIDs[th.PolicyID] = true
		if th.MetricCode == "" {
			return fmt.Errorf("thresholds[%s]: empty metric_code", th.PolicyID)
		}
		if !validOperators[th.Operator] {
			return fmt.Errorf("thresholds[%s]: invalid operator %q", th.PolicyID, th.Operator)
		}
		if len(th.AllowedScopeTypes) == 0 {
			return fmt.Errorf("thresholds[%s]: allowed_scope_types must not be empty", th.PolicyID)
		}
	}

	if len(p.LimitationRegistry) == 0 {
		return fmt.Errorf("limitation_registry must not be empty")
	}
	seenLimitationCodes := make(map[string]bool, len(p.LimitationRegistry))
	for _, lim := range p.LimitationRegistry {
		if lim.Code == "" {
			return fmt.Errorf("limitation_registry: empty code")
		}
		if seenLimitationCodes[lim.Code] {
			return fmt.Errorf("limitation_registry: duplicate code %q", lim.Code)
		}
		seenLimitationCodes[lim.Code] = true
		if lim.Scope == "" {
			return fmt.Errorf("limitation_registry[%s]: empty scope", lim.Code)
		}
		if lim.Description == "" {
			return fmt.Errorf("limitation_registry[%s]: empty description", lim.Code)
		}
	}

	if len(p.UserFacingTextFields) == 0 {
		return fmt.Errorf("user_facing_text_fields must not be empty")
	}

	return nil
}

// FindingType returns the registry entry for a finding_type, or false when unknown.
func (p *Policy) FindingType(name string) (FindingTypeEntry, bool) {
	for _, ft := range p.FindingTypeRegistry {
		if ft.FindingType == name {
			return ft, true
		}
	}
	return FindingTypeEntry{}, false
}

// ActionType returns the registry entry for an action_type, or false when unknown.
func (p *Policy) ActionType(name string) (ActionTypeEntry, bool) {
	for _, at := range p.ActionTypeRegistry {
		if at.ActionType == name {
			return at, true
		}
	}
	return ActionTypeEntry{}, false
}

// CategoryForActionType returns the category for an action_type, mirroring
// actions.ActionCategoryForType for the v2 policy taxonomy.
func (p *Policy) CategoryForActionType(actionType string) (string, bool) {
	category, ok := p.ActionTypeToCategory[actionType]
	return category, ok
}

// Threshold returns the registry entry for a policy_id, or false when unknown.
func (p *Policy) Threshold(policyID string) (ThresholdEntry, bool) {
	for _, th := range p.Thresholds {
		if th.PolicyID == policyID {
			return th, true
		}
	}
	return ThresholdEntry{}, false
}

// Limitation returns the registry entry for a limitation code, or false when unknown.
func (p *Policy) Limitation(code string) (LimitationEntry, bool) {
	for _, lim := range p.LimitationRegistry {
		if lim.Code == code {
			return lim, true
		}
	}
	return LimitationEntry{}, false
}

// TextLimit returns the configured max length for a user-facing text field
// path (e.g. "actions[].title"), or false when the path is not configured.
func (p *Policy) TextLimit(fieldPath string) (int, bool) {
	limit, ok := p.TextLimits[fieldPath]
	return limit, ok
}

// IsUserFacingTextField reports whether fieldPath is registered as a
// user-facing text field that must never contain a raw business number.
func (p *Policy) IsUserFacingTextField(fieldPath string) bool {
	for _, f := range p.UserFacingTextFields {
		if f == fieldPath {
			return true
		}
	}
	return false
}

// ActionTypesSorted returns a stable, alphabetically sorted copy of every
// registered action_type (convenience for tests/tools).
func (p *Policy) ActionTypesSorted() []string {
	out := make([]string, 0, len(p.ActionTypeRegistry))
	for _, at := range p.ActionTypeRegistry {
		out = append(out, at.ActionType)
	}
	sort.Strings(out)
	return out
}

// FindingTypesSorted returns a stable, alphabetically sorted copy of every
// registered finding_type (convenience for tests/tools).
func (p *Policy) FindingTypesSorted() []string {
	out := make([]string, 0, len(p.FindingTypeRegistry))
	for _, ft := range p.FindingTypeRegistry {
		out = append(out, ft.FindingType)
	}
	sort.Strings(out)
	return out
}
