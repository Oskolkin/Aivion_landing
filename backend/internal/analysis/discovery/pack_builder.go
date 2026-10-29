package discovery

import (
	"context"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type BuildInput struct {
	SellerAccountID          int64
	RunID                    int64
	UniverseID               string
	UniverseHash             string
	LandscapeDetailHash      string
	MaterialitySelectionHash string
	SelectionArtifactSHA     string
	ContractBundleHash       string
	AnalysisDate             time.Time
	CutoffAt                 time.Time
	Detail                   landscape.LandscapeSet
	Selection                materiality.SelectionArtifact
	Sealed                   factualapi.FactualReadContext
	API                      factualapi.API
	Model                    string
	MaxOutputTokens          int
	Cache                    *GroundingCache
}

type BuildResult struct {
	Pack           PackArtifactV1
	PackBytes      []byte
	PayloadBytes   []byte
	HandleMap      FactHandleMapV1
	HandleMapBytes []byte
	Index          PackEntityIndex
	Manifest       RequestManifestV1
	ManifestBytes  []byte
}

func BuildPack(ctx context.Context, in BuildInput) (BuildResult, error) {
	if err := ValidateDiscoveryBuildBindings(in); err != nil {
		return BuildResult{}, err
	}
	sorted := landscape.SemanticHashInput(in.Detail)
	in.Detail = sorted

	idx, err := BuildEntityIndex(in.Detail, in.Selection)
	if err != nil {
		return BuildResult{}, err
	}
	for _, c := range in.Detail.Categories {
		if !c.Identity.IsLeaf || c.Identity.Unmapped {
			return BuildResult{}, packErr(CodePackInvalid, "pack categories must be leaf and not UNMAPPED")
		}
	}

	caps := map[string]string{}
	for _, c := range in.Detail.GlobalCapabilities {
		caps[c.Key] = c.FinalState
	}
	kinds := map[EntityKind]bool{KindAccount: true}
	if len(in.Detail.Categories) > 0 {
		kinds[KindCategory] = true
	}
	if len(in.Detail.SKUs) > 0 {
		kinds[KindSKU] = true
	}
	if len(in.Detail.Campaigns) > 0 {
		kinds[KindCampaign] = true
	}
	menu := BuildEvidenceMenu(caps, kinds, sealedUniverseDomains(in.Sealed))

	var cache GroundingCache
	if in.Cache != nil {
		cache = *in.Cache
	} else {
		cache, err = LoadGrounding(ctx, in.API, in.Sealed, in.Detail)
		if err != nil {
			return BuildResult{}, err
		}
	}
	handles := BuildFactHandleMap(in.Detail, cache)
	handleBytes, err := canonicaljson.Marshal(handles)
	if err != nil {
		return BuildResult{}, err
	}
	handleHash := artifacts.HashBytes(handleBytes)
	payload := ProjectPayload(in.Detail, menu, handles)
	payloadBytes, err := canonicaljson.Marshal(payload)
	if err != nil {
		return BuildResult{}, err
	}
	payloadHash := HashPayload(payload)

	model := in.Model
	if model == "" {
		model = "gpt-4.1-mini"
	}
	maxOut := in.MaxOutputTokens
	if maxOut <= 0 {
		maxOut = DefaultMaxOutputTokens()
	}
	req := Request{
		Model: model, Instructions: StableInstructionsV1,
		PayloadJSON: payloadBytes, Schema: OutputJSONSchema(),
		SchemaName: OutputSchemaNameV1, MaxOutputTokens: maxOut,
	}
	est, err := EstimateDiscoveryRequestTokens(req)
	if err != nil {
		return BuildResult{}, err
	}
	budget := CurrentPackBudget()
	if est > budget.MaxInputTokens {
		return BuildResult{}, packErr(CodePackBudgetExceeded, "estimated_input_tokens exceeds pack budget")
	}
	fp := FingerprintFromRequest(req)
	reqHash := HashFingerprint(fp)

	semanticSel := in.Selection.SelectionHash
	if semanticSel == "" {
		semanticSel = in.MaterialitySelectionHash
	}
	selArt := in.SelectionArtifactSHA
	if selArt == "" {
		selArt = in.MaterialitySelectionHash
	}

	pack := PackArtifactV1{
		SchemaVersion:   PackArtifactSchemaV2,
		SellerAccountID: in.SellerAccountID, RunID: in.RunID, FactualUniverseID: in.UniverseID,
		FactualUniverseHash: in.UniverseHash, MaterialitySelectionHash: semanticSel,
		SelectionArtifactSHA256: selArt, LandscapeDetailHash: in.LandscapeDetailHash,
		LandscapeProfileVersion:    in.Detail.Context.LandscapeProfileVersion,
		ContractBundleHash:         in.ContractBundleHash,
		DiscoveryPackSchemaVersion: PackSchemaV2, FactHandleRegistryVersion: FactHandleRegistryV2,
		DiscoveryPromptPolicyVersion: PromptPolicyV1, DiscoveryOutputSchemaVersion: OutputSchemaV1,
		DiscoveryValidationVersion: ValidationV3, EvidenceCatalogVersion: EvidenceCatalogV1,
		EvidenceMenuPolicyVersion: EvidenceMenuPolicyV3, PackBudgetProfileVersion: PackBudgetV2,
		OutputBudgetProfileVersion: OutputBudgetV1, ModelPolicyVersion: ModelPolicyV1,
		AIAttemptPolicyVersion: AIAttemptPolicyV1, RequestContractVersion: RequestContractV2,
		PayloadHash: payloadHash, FactHandleMapHash: handleHash, EvidenceMenuHash: MenuHash(menu),
		RequestHash: reqHash, EstimatedInputTokens: est, Payload: payload,
	}
	if err = ValidatePack(pack, handles, idx, payloadBytes); err != nil {
		return BuildResult{}, err
	}

	schemaHash := canonicaljson.Hash(OutputJSONSchema())
	full := struct {
		AuditEnvelope
		Payload PayloadV1 `json:"payload"`
	}{AuditEnvelope: pack.Envelope(), Payload: payload}
	packBytes, err := canonicaljson.Marshal(full)
	if err != nil {
		return BuildResult{}, err
	}
	manifest := RequestManifestV1{
		RequestContractVersion: RequestContractV2, Model: model, ModelPolicyVersion: ModelPolicyV1,
		PromptPolicyVersion: PromptPolicyV1, PromptHash: PromptHash(),
		OutputSchemaVersion: OutputSchemaV1, OutputSchemaHash: schemaHash,
		SchemaName: OutputSchemaNameV1, Strict: true,
		PackHash: canonicaljson.Hash(full), PayloadHash: payloadHash, FactHandleMapHash: handleHash,
		EvidenceMenuHash: MenuHash(menu), RequestHash: reqHash, MaxOutputTokens: maxOut, TruncationMode: "disabled",
		Store: false, ToolsEnabled: false, PreviousResponseEnabled: false, EstimatedInputTokens: est,
	}
	manifestBytes, err := canonicaljson.Marshal(manifest)
	if err != nil {
		return BuildResult{}, err
	}
	return BuildResult{
		Pack: pack, PackBytes: packBytes, PayloadBytes: payloadBytes,
		HandleMap: handles, HandleMapBytes: handleBytes, Index: idx,
		Manifest: manifest, ManifestBytes: manifestBytes,
	}, nil
}
