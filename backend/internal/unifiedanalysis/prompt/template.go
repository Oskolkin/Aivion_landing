package prompt

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/lineendings"
)

// Version publication status for the prompt registry.
const (
	PromptStatusPublished      = "published"
	PromptStatusNeverPublished = "never_published"
)

//go:embed analyst_v1_22.txt
var analystV1_22Instructions string

//go:embed analyst_v1_22_input.txt
var analystV1_22Input string

//go:embed analyst_v1_24.txt
var analystV1_24Instructions string

//go:embed analyst_v1_24_input.txt
var analystV1_24Input string

//go:embed analyst_v1_25.txt
var analystV1_25Instructions string

//go:embed analyst_v1_25_input.txt
var analystV1_25Input string

//go:embed analyst_v1_26.txt
var analystV1_26Instructions string

//go:embed analyst_v1_26_input.txt
var analystV1_26Input string

//go:embed analyst_v1_27.txt
var analystV1_27Instructions string

//go:embed analyst_v1_27_input.txt
var analystV1_27Input string

//go:embed analyst_v1_28.txt
var analystV1_28Instructions string

//go:embed analyst_v1_28_input.txt
var analystV1_28Input string

//go:embed analyst_v1_29.txt
var analystV1_29Instructions string

//go:embed analyst_v1_29_input.txt
var analystV1_29Input string

//go:embed analyst_v1_30.txt
var analystV1_30Instructions string

//go:embed analyst_v1_30_input.txt
var analystV1_30Input string

//go:embed analyst_v1_31.txt
var analystV1_31Instructions string

//go:embed analyst_v1_31_input.txt
var analystV1_31Input string

//go:embed analyst_v1_32.txt
var analystV1_32Instructions string

//go:embed analyst_v1_32_input.txt
var analystV1_32Input string

// CurrentAnalystVersion is the production analysis prompt version.
const CurrentAnalystVersion = VersionAnalystV1_31

// VersionMeta describes one registered analyst prompt version.
type VersionMeta struct {
	Version string
	Status  string
	// Notes is human-readable registry provenance (not part of the hash).
	Notes string
}

// RegistryMeta returns immutable registry metadata for known versions.
func RegistryMeta(version string) (VersionMeta, error) {
	switch strings.TrimSpace(version) {
	case VersionAnalystV1_22:
		return VersionMeta{
			Version: VersionAnalystV1_22,
			Status:  PromptStatusPublished,
			Notes:   "Frozen from baseline commit e0dfc322 (schema v8 body). Restored via git show of analyst_v1.txt at that SHA.",
		}, nil
	case VersionAnalystV1_23:
		return VersionMeta{
			Version: VersionAnalystV1_23,
			Status:  PromptStatusNeverPublished,
			Notes:   "Intermediate Wave 2 constant only; no provider run and no recoverable distinct body. TemplateByVersion must not fall back to current.",
		}, nil
	case VersionAnalystV1_24:
		return VersionMeta{
			Version: VersionAnalystV1_24,
			Status:  PromptStatusPublished,
			Notes:   "Schema v9 + ad_efficiency.narrative.metric_claims. Superseded by v1_25 after acceptance runs 190/191.",
		}, nil
	case VersionAnalystV1_25:
		return VersionMeta{
			Version: VersionAnalystV1_25,
			Status:  PromptStatusPublished,
			Notes:   "Schema v9: store-share vs attributed DRR labeling, Pack fact_id shape, no min-5 Actions quota. Superseded by v1_26 after run 194.",
		}, nil
	case VersionAnalystV1_26:
		return VersionMeta{
			Version: VersionAnalystV1_26,
			Status:  PromptStatusPublished,
			Notes:   "Schema v10 + threshold_claims for policy-backed comparative thresholds (run 194). Superseded by v1_27 after run 201.",
		}, nil
	case VersionAnalystV1_27:
		return VersionMeta{
			Version: VersionAnalystV1_27,
			Status:  PromptStatusPublished,
			Notes:   "Schema v10 + Top Actions must be execution_tier=required only (run 201). Superseded by v1_28 after run 202.",
		}, nil
	case VersionAnalystV1_28:
		return VersionMeta{
			Version: VersionAnalystV1_28,
			Status:  PromptStatusPublished,
			Notes:   "Schema v10 + canonical health_policy_v1 risk_drivers[].id (run 202). Superseded by v1_29 after run 204.",
		}, nil
	case VersionAnalystV1_29:
		return VersionMeta{
			Version: VersionAnalystV1_29,
			Status:  PromptStatusPublished,
			Notes:   "Schema v10 + section_fact_policy_v1 Seller Pulse account-only facts (run 204). Superseded by v1_30.",
		}, nil
	case VersionAnalystV1_30:
		return VersionMeta{
			Version: VersionAnalystV1_30,
			Status:  PromptStatusPublished,
			Notes:   "Schema v10 + materiality Top Actions + content evidence ban. Superseded by v1_31.",
		}, nil
	case VersionAnalystV1_31:
		return VersionMeta{
			Version: VersionAnalystV1_31,
			Status:  PromptStatusPublished,
			Notes:   "Current production, demo-readiness pass: schema v10 + 8-12 Actions (stock-by-urgency-band, ads high-DRR-vs-spend-without-orders split) when multiple material problems exist; structured 3-paragraph Seller Pulse narrative (sales/funnel/advertising) with priority-action deep links; mandatory funnel wording; expanded banned-phrase list.",
		}, nil
	case VersionAnalystV1_32:
		return VersionMeta{
			Version: VersionAnalystV1_32,
			Status:  PromptStatusPublished,
			Notes: "Registered for the v11 findings-first contract (schema aivion_unified_analysis_v11) " +
				"paired with analysis_policy_v2 (backend/internal/unifiedanalysis/policy/v2). " +
				"Status=published only so TemplateByVersion/HashForVersion resolve for tests and evals " +
				"— published DOES NOT mean current: CurrentAnalystVersion remains unified_analyst_v1_31 " +
				"until the v11 + policy v2 pipeline is fully wired into production.",
		}, nil
	default:
		return VersionMeta{}, fmt.Errorf("prompt: unknown version %q", version)
	}
}

// Template holds the frozen instructions + static input for one version.
type Template struct {
	Version      string
	Instructions string
	Input        string
}

// TemplateByVersion returns the exact immutable template for a published
// version. never_published and unknown versions return an explicit error —
// there is no fallback onto the current template.
func TemplateByVersion(version string) (Template, error) {
	meta, err := RegistryMeta(version)
	if err != nil {
		return Template{}, err
	}
	if meta.Status == PromptStatusNeverPublished {
		return Template{}, fmt.Errorf("prompt: version %q has status %s and has no recoverable template body", version, PromptStatusNeverPublished)
	}
	switch meta.Version {
	case VersionAnalystV1_22:
		return Template{
			Version:      VersionAnalystV1_22,
			Instructions: canonicalizePromptText(analystV1_22Instructions),
			Input:        canonicalizePromptText(analystV1_22Input),
		}, nil
	case VersionAnalystV1_24:
		return Template{
			Version:      VersionAnalystV1_24,
			Instructions: canonicalizePromptText(analystV1_24Instructions),
			Input:        canonicalizePromptText(analystV1_24Input),
		}, nil
	case VersionAnalystV1_25:
		return Template{
			Version:      VersionAnalystV1_25,
			Instructions: canonicalizePromptText(analystV1_25Instructions),
			Input:        canonicalizePromptText(analystV1_25Input),
		}, nil
	case VersionAnalystV1_26:
		return Template{
			Version:      VersionAnalystV1_26,
			Instructions: canonicalizePromptText(analystV1_26Instructions),
			Input:        canonicalizePromptText(analystV1_26Input),
		}, nil
	case VersionAnalystV1_27:
		return Template{
			Version:      VersionAnalystV1_27,
			Instructions: canonicalizePromptText(analystV1_27Instructions),
			Input:        canonicalizePromptText(analystV1_27Input),
		}, nil
	case VersionAnalystV1_28:
		return Template{
			Version:      VersionAnalystV1_28,
			Instructions: canonicalizePromptText(analystV1_28Instructions),
			Input:        canonicalizePromptText(analystV1_28Input),
		}, nil
	case VersionAnalystV1_29:
		return Template{
			Version:      VersionAnalystV1_29,
			Instructions: canonicalizePromptText(analystV1_29Instructions),
			Input:        canonicalizePromptText(analystV1_29Input),
		}, nil
	case VersionAnalystV1_30:
		return Template{
			Version:      VersionAnalystV1_30,
			Instructions: canonicalizePromptText(analystV1_30Instructions),
			Input:        canonicalizePromptText(analystV1_30Input),
		}, nil
	case VersionAnalystV1_31:
		return Template{
			Version:      VersionAnalystV1_31,
			Instructions: canonicalizePromptText(analystV1_31Instructions),
			Input:        canonicalizePromptText(analystV1_31Input),
		}, nil
	case VersionAnalystV1_32:
		return Template{
			Version:      VersionAnalystV1_32,
			Instructions: canonicalizePromptText(analystV1_32Instructions),
			Input:        canonicalizePromptText(analystV1_32Input),
		}, nil
	default:
		return Template{}, fmt.Errorf("prompt: version %q is registered but has no embedded body", version)
	}
}

// canonicalizePromptText normalizes line endings to LF, then TrimSpace.
// Hash and provider request bytes both use this canonical form.
func canonicalizePromptText(raw string) string {
	return strings.TrimSpace(lineendings.NormalizeLineEndings(raw))
}

// HashForVersion returns Hash(instructions, input) for a published version.
func HashForVersion(version string) (string, error) {
	tpl, err := TemplateByVersion(version)
	if err != nil {
		return "", err
	}
	return Hash(tpl.Instructions, tpl.Input), nil
}

// RenderByVersion builds a RenderedPrompt from a specific frozen version.
// Used for historical replay; unknown/never_published versions fail closed.
func RenderByVersion(version string) (RenderedPrompt, error) {
	tpl, err := TemplateByVersion(version)
	if err != nil {
		return RenderedPrompt{}, err
	}
	return RenderedPrompt{
		Profile:      PromptProfileAnalysis,
		Version:      tpl.Version,
		Instructions: tpl.Instructions,
		Input:        tpl.Input,
		Hash:         Hash(tpl.Instructions, tpl.Input),
	}, nil
}

// AnalystV1Instructions returns the current production instructions (v1_31).
func AnalystV1Instructions() string {
	tpl, err := TemplateByVersion(CurrentAnalystVersion)
	if err != nil {
		panic(err)
	}
	return tpl.Instructions
}

// AnalystV1Input returns the current production static input (v1_31).
func AnalystV1Input() string {
	tpl, err := TemplateByVersion(CurrentAnalystVersion)
	if err != nil {
		panic(err)
	}
	return tpl.Input
}
