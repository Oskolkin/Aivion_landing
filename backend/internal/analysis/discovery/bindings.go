package discovery

import (
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func ValidateDiscoveryBuildBindings(in BuildInput) error {
	detailSeller := in.Detail.Context.SellerAccountID
	if detailSeller == 0 {
		detailSeller = in.Detail.Account.Identity.SellerAccountID
	}
	if in.SellerAccountID != 0 && detailSeller != 0 && in.SellerAccountID != detailSeller {
		return packErr(CodePackInvalid, "seller mismatch between run and landscape detail")
	}
	if in.Sealed.SellerAccountID() != 0 && in.SellerAccountID != 0 && in.Sealed.SellerAccountID() != in.SellerAccountID {
		return packErr(CodePackInvalid, "seller mismatch between run and sealed read context")
	}
	if in.Sealed.SellerAccountID() != 0 && detailSeller != 0 && in.Sealed.SellerAccountID() != detailSeller {
		return packErr(CodePackInvalid, "seller mismatch between landscape detail and sealed read context")
	}

	detailDate := time.Time(in.Detail.Context.AnalysisDate)
	if !detailDate.IsZero() && !in.AnalysisDate.IsZero() && !sameDayUTC(detailDate, in.AnalysisDate) {
		return packErr(CodePackInvalid, "analysis_date mismatch between run and landscape detail")
	}
	if !in.Sealed.AnalysisDate().IsZero() && !in.AnalysisDate.IsZero() && !sameDayUTC(in.Sealed.AnalysisDate(), in.AnalysisDate) {
		return packErr(CodePackInvalid, "analysis_date mismatch between run and sealed read context")
	}
	if !in.Detail.Context.AnalysisCutoffAt.IsZero() && !in.CutoffAt.IsZero() && !in.Detail.Context.AnalysisCutoffAt.UTC().Equal(in.CutoffAt.UTC()) {
		return packErr(CodePackInvalid, "analysis_cutoff_at mismatch between run and landscape detail")
	}
	if !in.Sealed.CutoffAt().IsZero() && !in.CutoffAt.IsZero() && !in.Sealed.CutoffAt().UTC().Equal(in.CutoffAt.UTC()) {
		return packErr(CodePackInvalid, "analysis_cutoff_at mismatch between run and sealed read context")
	}

	if in.Sealed.Mode() != "" && in.Sealed.Mode() != factualapi.ModeLatestStable && in.Sealed.Mode() != factualapi.ModeSealedUniverse {
		return packErr(CodePackInvalid, fmt.Sprintf("unsupported sealed FactualReadContext mode %q", in.Sealed.Mode()))
	}

	if in.UniverseID != "" && in.Detail.Context.FactualUniverseID != "" && in.UniverseID != in.Detail.Context.FactualUniverseID {
		return packErr(CodePackInvalid, "universe id mismatch between run and landscape detail")
	}
	if in.UniverseID != "" && in.Sealed.UniverseID() != "" && in.UniverseID != in.Sealed.UniverseID() {
		return packErr(CodePackInvalid, "universe id mismatch between run and sealed read context")
	}

	if in.ContractBundleHash != "" && in.Detail.Context.ContractBundleHash != "" && in.ContractBundleHash != in.Detail.Context.ContractBundleHash {
		return packErr(CodePackInvalid, "contract bundle hash mismatch between run and landscape detail")
	}

	semanticSel := in.Selection.SelectionHash
	if in.Detail.Context.MaterialitySelectionHash != "" && semanticSel != "" && in.Detail.Context.MaterialitySelectionHash != semanticSel {
		return packErr(CodePackInvalid, "materiality selection semantic hash mismatch")
	}
	if in.Selection.PolicyVersion != "" {
		re := materiality.RecomputeSelectionHash(in.Selection)
		if semanticSel != "" && re != semanticSel {
			return packErr(CodePackInvalid, "materiality selection hash does not recompute")
		}
	}
	return nil
}

func sameDayUTC(a, b time.Time) bool {
	return a.UTC().Format("2006-01-02") == b.UTC().Format("2006-01-02")
}

func sealedUniverseDomains(sealed factualapi.FactualReadContext) map[string]bool {
	domains := map[string]bool{}
	for _, name := range sealed.DomainNames() {
		if name != "" {
			domains[strings.ToLower(name)] = true
		}
	}
	return domains
}
