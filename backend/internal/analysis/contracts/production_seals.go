package contracts

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actionreconcile"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidence"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/projection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/publication"
)

func productionLandscapeCoreSeal() string {
	return landscape.LandscapeSchemaV5 + "|" + landscape.CoreProfileVersionSealV5()
}

func productionLandscapeDetailSeal() string {
	return landscape.LandscapeSchemaV5 + "|" + landscape.DetailProfileVersionSealV5()
}

func productionMaterialitySeal() string {
	p := materiality.CurrentPolicy()
	reg := materiality.LaneRegistryV2Current()
	budget := materiality.DiscoveryStandardBudget()
	return fmt.Sprintf("%s#%s|%s#%s|%s#%s|RRF_K%d|ANCHOR_COV70|DIVERSITY_REL_V1",
		p.Version, p.Hash(),
		reg.Version, reg.Hash(),
		budget.Version, budget.Hash(),
		p.RRFK,
	)
}

func productionDiscoveryPackSeal() string       { return discovery.PackContractSeal() }
func productionDiscoveryPromptSeal() string     { return discovery.PromptContractSeal() }
func productionDiscoveryOutputSeal() string     { return discovery.OutputContractSeal() }
func productionDiscoveryValidationSeal() string { return discovery.ValidationContractSeal() }
func productionEvidenceExpansionSeal() string   { return evidence.ExpansionContractSeal() }
func productionDeepPackSeal() string            { return deep.PackContractSeal() }
func productionDeepPromptSeal() string          { return deep.PromptContractSeal() }
func productionDeepOutputSeal() string          { return deep.OutputContractSeal() }
func productionDeepValidationSeal() string      { return deep.ValidationContractSeal() }
func productionActionReconciliationSeal() string {
	return "ACTION_RECONCILIATION#" + actionreconcile.IdentityPolicyV1 + "|" + actionreconcile.ReconciliationPolicyV1 + "|" + actionreconcile.RecurrencePolicyV1 + ":" + actionreconcile.RecurrenceDisabledUntilCalibrated
}
func productionPublicationSeal() string {
	return "PUBLICATION#" + publication.SchemaV1 + "|" + publication.ContentHashPolicyV1
}
func productionProjectionSeal() string {
	return "PROJECTION#" + projection.PolicyV1 + "|" + projection.OverviewV1 + "|" + projection.FunnelV1
}
func productionPublicationValidationSeal() string {
	return "PUBLICATION_VALIDATION#" + publication.ValidationV1
}
func productionPublishSeal() string     { return "PUBLISH#" + publication.PublishPolicyV1 }
func productionPostPublishSeal() string { return "POST_PUBLISH#" + publication.PostPublishPolicyV1 }
