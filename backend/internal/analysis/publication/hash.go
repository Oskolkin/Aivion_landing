package publication

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"

func ContentHash(c CandidateV1) string {
	cp := c
	cp.PublicationContentHash = ""
	return canonicaljson.Hash(map[string]any{
		"policy": ContentHashPolicyV1,
		"candidate": map[string]any{
			"schema_version":                  cp.SchemaVersion,
			"seller_account_id":               cp.SellerAccountID,
			"analysis_run_id":                 cp.AnalysisRunID,
			"expected_base_publication_id":    cp.ExpectedBasePublicationID,
			"expected_publication_generation": cp.ExpectedPublicationGeneration,
			"analysis_date":                   cp.AnalysisDate,
			"analysis_cutoff_at":              cp.AnalysisCutoffAt,
			"factual_universe_id":             cp.FactualUniverseID,
			"factual_universe_hash":           cp.FactualUniverseHash,
			"contract_bundle_hash":            cp.ContractBundleHash,
			"components":                      cp.Components,
			"action_reconciliation_plan_hash": cp.ActionReconciliationHash,
		},
	})
}

func BuildCandidate(c CandidateV1) CandidateV1 {
	c.SchemaVersion = SchemaV1
	c.ContentHashPolicy = ContentHashPolicyV1
	c.PublicationContentHash = ContentHash(c)
	return c
}
