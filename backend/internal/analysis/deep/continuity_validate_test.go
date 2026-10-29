package deep

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
)

func TestContinuityProposalAllowedOppositePrice(t *testing.T) {
	snap := actioncatalog.Snapshot{
		MainIDs: map[string]bool{"SKU|417": true},
		SKU: map[string]actioncatalog.SKUState{
			"417": {
				HasPrice:     true,
				CurrentPrice: &actioncatalog.Money{Amount: 1000, Currency: "RUB"},
			},
		},
	}
	cont := ContinuityView{
		PreviousActions: []PreviousAction{{
			ActionType: actioncatalog.TypeSKUAdjustPrice,
			Target:     EntityRef{Kind: discovery.KindSKU, ID: "417"},
			Parameters: actioncatalog.Parameters{
				RecommendedPrice: &actioncatalog.Money{Amount: 950, Currency: "RUB"},
			},
		}},
	}
	prop := actioncatalog.Proposal{
		ActionType: actioncatalog.TypeSKUAdjustPrice,
		Target:     EntityRef{Kind: discovery.KindSKU, ID: "417"},
		Parameters: actioncatalog.Parameters{
			RecommendedPrice: &actioncatalog.Money{Amount: 1050, Currency: "RUB"},
		},
	}
	if !continuityProposalAllowed(prop, cont, snap, PayloadV1{}) {
		t.Fatal("expected continuity price revalidation allowed")
	}
	menu := actioncatalog.Menu{Version: actioncatalog.MenuPolicyV1, Catalog: actioncatalog.CatalogV1, Items: nil}
	if err := actioncatalog.ValidateProposal(prop, menu, snap); err == nil {
		t.Fatal("expected NOT_IN_MENU without continuity menu")
	}
}
