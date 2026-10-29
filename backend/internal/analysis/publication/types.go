package publication

type CandidateV1 struct {
	SchemaVersion                 string         `json:"schema_version"`
	ContentHashPolicy             string         `json:"content_hash_policy"`
	SellerAccountID               int64          `json:"seller_account_id"`
	AnalysisRunID                 int64          `json:"analysis_run_id"`
	ExpectedBasePublicationID     *int64         `json:"expected_base_publication_id"`
	ExpectedPublicationGeneration int64          `json:"expected_publication_generation"`
	AnalysisDate                  string         `json:"analysis_date"`
	AnalysisCutoffAt              string         `json:"analysis_cutoff_at"`
	FactualUniverseID             string         `json:"factual_universe_id"`
	FactualUniverseHash           string         `json:"factual_universe_hash"`
	ContractBundleHash            string         `json:"contract_bundle_hash"`
	Components                    []ComponentRef `json:"components"`
	ActionReconciliationHash      string         `json:"action_reconciliation_plan_hash"`
	PublicationContentHash        string         `json:"publication_content_hash"`
}

type ComponentRef struct {
	ComponentType string `json:"component_type"`
	SchemaVersion string `json:"schema_version"`
	SHA256        string `json:"sha256"`
}

type ManifestV1 struct {
	SchemaVersion string         `json:"schema_version"`
	Components    []ComponentRef `json:"components"`
}

type ValidationReport struct {
	SchemaVersion string   `json:"schema_version"`
	Valid         bool     `json:"valid"`
	Errors        []string `json:"errors"`
}

type ValidatedCandidateV1 struct {
	SchemaVersion string      `json:"schema_version"`
	Candidate     CandidateV1 `json:"candidate"`
	Valid         bool        `json:"valid"`
}
