package publication

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actionreconcile"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/projection"
)

func Validate(c CandidateV1, man projection.Manifest, plan actionreconcile.PlanV1, validated deep.ValidatedOutputV1, seller, runID int64, contractHash string) ValidationReport {
	r := ValidationReport{SchemaVersion: ValidationV1, Errors: []string{}}
	add := func(s string) { r.Errors = append(r.Errors, s) }
	if c.SellerAccountID != seller || c.AnalysisRunID != runID {
		add("SELLER_OR_RUN_MISMATCH")
	}
	if c.ContractBundleHash != contractHash {
		add("CONTRACT_BUNDLE_MISMATCH")
	}
	if ContentHash(c) != c.PublicationContentHash {
		add("CONTENT_HASH_RECOMPUTE")
	}
	if len(plan.NewProposalIntents) != len(validated.Output.ActionProposals) {
		add("DISPOSITION_COUNT")
	}
	need := []string{projection.KeyOverview, projection.KeyFunnel, projection.KeyStock, projection.KeyAdvertising, projection.KeyActionsSnapshot, projection.KeyChatContext}
	for _, k := range need {
		if man.Hashes[k] == "" {
			add("MISSING_PROJECTION:" + k)
		}
	}
	if len(c.Components) == 0 {
		add("COMPONENTS_MISSING")
	}
	for _, np := range plan.NewProposalIntents {
		req := 0
		for _, s := range np.Supports {
			if s.Role == "REQUIRED_SUPPORT" {
				req++
			}
		}
		if req < 1 && len(validated.Output.ActionProposals) > 0 {
			add("REQUIRED_SUPPORT_MISSING:" + np.LocalRef)
		}
	}
	r.Valid = len(r.Errors) == 0
	return r
}
