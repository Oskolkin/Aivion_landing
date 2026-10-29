package deep

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
)

func FixtureJSON(payloadJSON []byte) string {
	var p PayloadV1
	_ = json.Unmarshal(payloadJSON, &p)
	if p.DiscoveryResult.DiscoveryState == string(discovery.StateNoMeaningfulThreads) || len(p.DiscoveryResult.Threads) == 0 {
		return noMeaningfulFixture(p)
	}
	handle := firstHandle(p)
	th := p.DiscoveryResult.Threads[0]
	primary := th.PrimaryEntity
	if primary.ID == "" {
		primary = p.AccountContext.Entity
	}
	finding := Finding{
		Ordinal: 1, OriginType: "THREAD_DERIVED", PrimaryEntity: primary,
		RelatedEntities: []EntityRef{}, Title: "Подтверждённое наблюдение",
		FindingStatement:  "Факты пакета подтверждают исходную нить расследования.",
		BusinessDirection: "NEGATIVE", PrimaryDomain: DomainSales, OverviewBucket: BucketActiveRisk,
		RelationshipAssessments: []RelationshipAssessment{},
		SupportingFactHandles:   []string{handle}, CounterFactHandles: []string{},
		Limitations: []string{}, SourceThreadRefs: []string{th.ThreadRef},
	}
	rec := Recommendation{
		Ordinal: 1, TargetEntity: primary,
		RecommendationStatement: "Зафиксировать ограничение и действовать только через допустимое меню.",
		Rationale:               "Finding опирается на доступные fact handles пакета.",
		SupportFindingOrdinals:  []int{1}, SupportFactHandles: []string{handle},
		Limitations: []string{}, Actionability: "MONITOR",
	}
	actions := []ActionProposal{}
	menuItem, menuOK := firstMenuAction(p.ActionMenu)
	if strings.TrimSpace(os.Getenv("DEEP_FAKE_PRICE_INTENT")) != "" ||
		strings.TrimSpace(os.Getenv("DEEP_FAKE_PRICE_MENU")) == "1" ||
		strings.TrimSpace(os.Getenv("DEEP_FAKE_OPPOSITE_PRICE_PAIR")) == "1" {
		menuItem, menuOK = priceMenuItem(p)
	}
	if menuOK {
		if strings.TrimSpace(os.Getenv("DEEP_FAKE_OPPOSITE_PRICE_PAIR")) == "1" {
			if pair, ok := oppositePricePair(p, handle, menuItem); ok {
				actions = pair
				rec.Actionability = "ACTIONABLE_NOW"
				rec.RecommendationStatement = "Выполнить допустимое действие из Action Menu."
			}
		} else if params, target, okp := fixtureParams(menuItem, p); okp {
			rec.Actionability = "ACTIONABLE_NOW"
			rec.RecommendationStatement = "Выполнить допустимое действие из Action Menu."
			supports := priceFixtureSupports(target, handle)
			if menuItem.ActionType != actioncatalog.TypeSKUAdjustPrice &&
				strings.TrimSpace(os.Getenv("DEEP_FAKE_STOCK_SUPPORT")) != "1" {
				supports = supports[:2]
			}
			actions = []ActionProposal{{
				Ordinal: 1, ActionType: menuItem.ActionType, Target: target, Parameters: params,
				RecommendationOrdinal: 1, FindingOrdinals: []int{1}, FactHandles: []string{handle},
				Supports: supports,
			}}
		}
	}
	if len(actions) == 0 {
		if a, ok := fallbackAction(p, handle, finding); ok {
			actions = []ActionProposal{a}
			rec.Actionability = "ACTIONABLE_NOW"
		}
	}
	if n := fakeGroupSize(); n > 1 && len(actions) == 1 {
		actions = appendSameTypeClones(actions, n)
	} else if strings.TrimSpace(os.Getenv("DEEP_FAKE_TWIN_ACTIONS")) == "1" && len(actions) == 1 {
		actions = appendTwinAction(actions)
	}
	if len(p.Continuity.PreviousActions) > 0 {
		assessment := envAssessment()
		actions = []ActionProposal{}
		switch assessment {
		case "FULLY_SUPPORTED":
			if a, ok := continuityMirroredProposal(p, handle, 1); ok {
				actions = []ActionProposal{a}
				rec.Actionability = "ACTIONABLE_NOW"
				rec.RecommendationStatement = "Предыдущее действие подтверждено текущими фактами."
			}
		case "SUPERSEDED":
			if a, ok := continuitySupersededProposal(p, handle, 1); ok {
				actions = []ActionProposal{a}
				rec.Actionability = "ACTIONABLE_NOW"
				rec.RecommendationStatement = "Новое действие заменяет предыдущее основание."
			}
		}
	}
	ensureMenuActions(&p, actions)
	out := OutputV1{
		ExecutiveSummary: ExecutiveSummary{
			Headline: "Результаты углубленного разбора",
			Summary:  "Нить расследования сопоставлена с доступными фактами продавца.",
			KeyTakeaways: []Takeaway{{
				Statement: "Есть проверяемое наблюдение по основной сущности.",
				Direction: "NEGATIVE", PrimaryDomain: DomainSales, SupportingFactHandles: []string{handle}, FindingOrdinals: []int{1},
			}},
			BasisFactHandles: []string{handle}, ReferencedFindingOrdinals: []int{1}, Limitations: []string{},
		},
		ThreadResolutions:         fixtureResolutions(p.DiscoveryResult.Threads, handle),
		Findings:                  []Finding{finding},
		Recommendations:           []Recommendation{rec},
		ActionProposals:           actions,
		PreviousActionAssessments: fixturePreviousAssessments(p, handle, len(actions) > 0),
	}
	b, err := json.Marshal(out)
	if err != nil {
		return noMeaningfulFixture(p)
	}
	return string(b)
}

func oppositePricePair(p PayloadV1, handle string, item actioncatalog.MenuItem) ([]ActionProposal, bool) {
	if item.ActionType != actioncatalog.TypeSKUAdjustPrice || len(item.EligibleTargetIDs) == 0 {
		return nil, false
	}
	id := item.EligibleTargetIDs[0]
	target := EntityRef{Kind: discovery.EntityKind(item.TargetKind), ID: id}
	curAmt := 1000.0
	cur := "RUB"
	if st := skuPrice(p, id); st != nil {
		curAmt = st.Amount
		cur = st.Currency
	}
	dec := curAmt - 10
	inc := curAmt + 10
	if dec <= 0 {
		dec = 1
	}
	dec = float64(int64(dec))
	inc = float64(int64(inc))
	supports := priceFixtureSupports(target, handle)
	return []ActionProposal{
		{
			Ordinal: 1, ActionType: actioncatalog.TypeSKUAdjustPrice, Target: target,
			Parameters:            actioncatalog.Parameters{RecommendedPrice: &actioncatalog.Money{Amount: dec, Currency: cur}},
			RecommendationOrdinal: 1, FindingOrdinals: []int{1}, FactHandles: []string{handle}, Supports: supports,
		},
		{
			Ordinal: 2, ActionType: actioncatalog.TypeSKUAdjustPrice, Target: target,
			Parameters:            actioncatalog.Parameters{RecommendedPrice: &actioncatalog.Money{Amount: inc, Currency: cur}},
			RecommendationOrdinal: 1, FindingOrdinals: []int{1}, FactHandles: []string{handle}, Supports: supports,
		},
	}, true
}

func priceFixtureSupports(target EntityRef, handle string) []ActionSupportBasis {
	supports := []ActionSupportBasis{
		{
			SupportKey: "1", Ordinal: 1, Role: "REQUIRED_SUPPORT",
			Claim:           "Цена SKU подтверждает основание действия.",
			FindingOrdinals: []int{1}, FactHandles: []string{handle},
			Revalidation: SupportRevalidationSpec{EvidenceType: "SKU_PRICE_HISTORY", Subject: target},
		},
		{
			SupportKey: "2", Ordinal: 2, Role: "REQUIRED_SUPPORT",
			Claim:           "Продажи SKU подтверждают основание действия.",
			FindingOrdinals: []int{1}, FactHandles: []string{handle},
			Revalidation: SupportRevalidationSpec{EvidenceType: "SKU_SALES_HISTORY", Subject: target},
		},
	}
	if strings.TrimSpace(os.Getenv("DEEP_FAKE_STOCK_SUPPORT")) == "1" {
		supports = append(supports, ActionSupportBasis{
			SupportKey: "3", Ordinal: 3, Role: "REQUIRED_SUPPORT",
			Claim:           "Остаток SKU подтверждает основание действия.",
			FindingOrdinals: []int{1}, FactHandles: []string{handle},
			Revalidation: SupportRevalidationSpec{EvidenceType: "SKU_STOCK_HISTORY", Subject: target},
		})
	}
	return supports
}

func fakeGroupSize() int {
	n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("DEEP_FAKE_GROUP_SIZE")))
	if err != nil || n < 2 {
		return 0
	}
	if n > 8 {
		n = 8
	}
	return n
}

func appendSameTypeClones(actions []ActionProposal, n int) []ActionProposal {
	if len(actions) != 1 || n < 2 {
		return actions
	}
	base := actions[0]
	out := []ActionProposal{base}
	for i := 1; i < n; i++ {
		clone := base
		clone.Ordinal = i + 1
		clone.Target.ID = fmt.Sprintf("%s-%d", base.Target.ID, i)
		out = append(out, clone)
	}
	return out
}

func appendTwinAction(actions []ActionProposal) []ActionProposal {
	if len(actions) != 1 {
		return actions
	}
	twin := actions[0]
	twin.Ordinal = 2
	if twin.Parameters.RecommendedPrice != nil {
		amt := twin.Parameters.RecommendedPrice.Amount
		cur := twin.Parameters.RecommendedPrice.Currency
		if strings.EqualFold(strings.TrimSpace(os.Getenv("DEEP_FAKE_PRICE_INTENT")), "DECREASE") {
			amt -= 20
			if amt <= 0 {
				amt = 1
			}
		} else {
			amt += 20
		}
		amt = float64(int64(amt))
		twin.Parameters.RecommendedPrice = &actioncatalog.Money{Amount: amt, Currency: cur}
	} else if twin.Parameters.RecommendedUnits != nil {
		u := *twin.Parameters.RecommendedUnits + 1
		twin.Parameters.RecommendedUnits = &u
	} else if twin.Parameters.RecommendedUnitsReduction != nil {
		u := *twin.Parameters.RecommendedUnitsReduction + 1
		twin.Parameters.RecommendedUnitsReduction = &u
	} else {
		u := 1
		twin.Parameters.RecommendedUnits = &u
	}
	return append(actions, twin)
}

func envAssessment() string {
	a := strings.TrimSpace(os.Getenv("DEEP_FAKE_ASSESSMENT"))
	if a == "" {
		return "FULLY_SUPPORTED"
	}
	return a
}

func continuitySupersededProposal(p PayloadV1, handle string, recOrdinal int) (ActionProposal, bool) {
	if len(p.Continuity.PreviousActions) == 0 {
		return ActionProposal{}, false
	}
	pa := p.Continuity.PreviousActions[0]
	hs := continuityAnyHandles(p, handle)
	switch pa.ActionType {
	case actioncatalog.TypeSKUAdjustPrice:
		curAmt := 1000.0
		cur := "RUB"
		if st := skuPrice(p, pa.Target.ID); st != nil {
			curAmt = st.Amount
			cur = st.Currency
		}
		amt := curAmt + 50
		if pa.Parameters.RecommendedPrice != nil && pa.Parameters.RecommendedPrice.Amount > curAmt {
			amt = curAmt - 50
			if amt <= 0 {
				amt = 1
			}
		} else if strings.EqualFold(pa.IntentSignature, "DECREASE") {
			amt = curAmt + 50
		}
		amt = float64(int64(amt))
		return ActionProposal{
			Ordinal: 1, ActionType: actioncatalog.TypeSKUAdjustPrice, Target: pa.Target,
			Parameters:            actioncatalog.Parameters{RecommendedPrice: &actioncatalog.Money{Amount: amt, Currency: cur}},
			RecommendationOrdinal: recOrdinal, FindingOrdinals: []int{1}, FactHandles: hs,
			Supports: continuityMirroredSupports(pa, hs),
		}, true
	case actioncatalog.TypeSKUIncreaseReplenishment, actioncatalog.TypeSKUReduceReplenishment:
		opp := actioncatalog.TypeSKUReduceReplenishment
		if pa.ActionType == actioncatalog.TypeSKUReduceReplenishment {
			opp = actioncatalog.TypeSKUIncreaseReplenishment
		}
		if menuHas(p.ActionMenu, opp, pa.Target.ID) {
			return ActionProposal{
				Ordinal: 1, ActionType: opp, Target: pa.Target,
				Parameters: actioncatalog.Parameters{}, RecommendationOrdinal: recOrdinal,
				FindingOrdinals: []int{1}, FactHandles: hs, Supports: continuityMirroredSupports(pa, hs),
			}, true
		}
		// Fallback: any other eligible action on same target → new signature.
		for _, it := range p.ActionMenu.Items {
			if it.ActionType == pa.ActionType {
				continue
			}
			okTarget := false
			for _, id := range it.EligibleTargetIDs {
				if id == pa.Target.ID {
					okTarget = true
					break
				}
			}
			if !okTarget {
				continue
			}
			params, target, okp := fixtureParams(it, p)
			if !okp {
				continue
			}
			return ActionProposal{
				Ordinal: 1, ActionType: it.ActionType, Target: target, Parameters: params,
				RecommendationOrdinal: recOrdinal, FindingOrdinals: []int{1}, FactHandles: hs,
				Supports: continuityMirroredSupports(pa, hs),
			}, true
		}
		return ActionProposal{}, false
	default:
		return ActionProposal{}, false
	}
}

func menuHas(m actioncatalog.Menu, actionType, targetID string) bool {
	for _, it := range m.Items {
		if it.ActionType != actionType {
			continue
		}
		for _, id := range it.EligibleTargetIDs {
			if id == targetID {
				return true
			}
		}
	}
	return false
}

func shouldMirrorContinuityProposal() bool {
	return envAssessment() == "FULLY_SUPPORTED"
}

func continuityMirroredProposal(p PayloadV1, handle string, recOrdinal int) (ActionProposal, bool) {
	if len(p.Continuity.PreviousActions) == 0 {
		return ActionProposal{}, false
	}
	pa := p.Continuity.PreviousActions[0]
	hs := continuityAnyHandles(p, handle)
	return ActionProposal{
		Ordinal:               1,
		ActionType:            pa.ActionType,
		Target:                pa.Target,
		Parameters:            pa.Parameters,
		RecommendationOrdinal: recOrdinal,
		FindingOrdinals:       []int{1},
		FactHandles:           hs,
		Supports:              continuityMirroredSupports(pa, hs),
	}, true
}

func continuityMirroredSupports(pa PreviousAction, hs []string) []ActionSupportBasis {
	out := make([]ActionSupportBasis, 0, len(pa.RequiredSupports))
	for _, sp := range pa.RequiredSupports {
		out = append(out, ActionSupportBasis{
			SupportKey:      sp.SupportKey,
			Ordinal:         len(out) + 1,
			Role:            "REQUIRED_SUPPORT",
			Claim:           sp.Claim,
			FindingOrdinals: []int{1},
			FactHandles:     hs,
			Revalidation:    sp.Revalidation,
		})
	}
	return out
}

func fixtureResolutions(threads []ThreadView, handle string) []ThreadResolution {
	out := make([]ThreadResolution, 0, len(threads))
	modes := []string{"SUBSTANTIATED", "REFRAMED", "NOT_SUBSTANTIATED", "INCONCLUSIVE"}
	for i, th := range threads {
		out = append(out, ThreadResolution{
			DiscoveryThreadRef: th.ThreadRef, Resolution: modes[i%len(modes)],
			ResolutionSummary:     "Нить сопоставлена с доступными фактами.",
			SupportingFactHandles: []string{handle}, CounterFactHandles: []string{},
			Limitations: []string{}, ResultingFindingOrdinals: []int{1},
		})
	}
	return out
}

func noMeaningfulFixture(p PayloadV1) string {
	handle := firstHandle(p)
	acc := p.AccountContext.Entity
	if acc.ID == "" {
		acc = EntityRef{Kind: discovery.KindAccount, ID: "account"}
	}
	hs := []string{}
	if handle != "" {
		hs = []string{handle}
	}
	out := OutputV1{
		ExecutiveSummary: ExecutiveSummary{
			Headline: "Нет содержательных нитей",
			Summary:  "Пакет не содержит substantiated threads; приводится account-level сводка.",
			KeyTakeaways: []Takeaway{{
				Statement: "На уровне аккаунта нет искусственных действий.",
				Direction: "NEUTRAL", PrimaryDomain: DomainSales, SupportingFactHandles: hs, FindingOrdinals: []int{1},
			}},
			BasisFactHandles: hs, ReferencedFindingOrdinals: []int{1}, Limitations: []string{},
		},
		ThreadResolutions: []ThreadResolution{},
		Findings: []Finding{{
			Ordinal: 1, OriginType: "ACCOUNT_CONTEXT", PrimaryEntity: acc,
			RelatedEntities: []EntityRef{}, Title: "Контекст аккаунта",
			FindingStatement:  "Содержательные нити Discovery отсутствуют.",
			BusinessDirection: "NEUTRAL", PrimaryDomain: DomainSales, OverviewBucket: BucketNone,
			RelationshipAssessments: []RelationshipAssessment{},
			SupportingFactHandles:   hs, CounterFactHandles: []string{}, Limitations: []string{},
			SourceThreadRefs: []string{},
		}},
		Recommendations: []Recommendation{{
			Ordinal: 1, TargetEntity: acc,
			RecommendationStatement: "Мониторить появление устойчивых сигналов.",
			Rationale:               "Недостаточно нитей для управленческого действия.",
			SupportFindingOrdinals:  []int{1}, SupportFactHandles: hs, Limitations: []string{},
			Actionability: "MONITOR",
		}},
		ActionProposals:           []ActionProposal{},
		PreviousActionAssessments: fixturePreviousAssessments(p, handle, false),
	}
	if handle == "" && len(p.Continuity.PreviousActions) == 0 {
		out.Findings = nil
		out.Recommendations = nil
		out.ExecutiveSummary.KeyTakeaways[0].FindingOrdinals = []int{}
		out.ExecutiveSummary.ReferencedFindingOrdinals = []int{}
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func fallbackAction(p PayloadV1, handle string, finding Finding) (ActionProposal, bool) {
	target := finding.PrimaryEntity
	if target.Kind != discovery.KindSKU {
		for _, e := range p.ThreadEntityContext {
			if e.Entity.Kind == discovery.KindSKU && e.Entity.ID != "" {
				target = e.Entity
				break
			}
		}
	}
	if target.ID == "" {
		return ActionProposal{}, false
	}
	hs := []string{}
	if handle != "" {
		hs = []string{handle}
	}
	return ActionProposal{
		Ordinal: 1, ActionType: actioncatalog.TypeSKUIncreaseReplenishment, Target: target,
		Parameters:            actioncatalog.Parameters{},
		RecommendationOrdinal: 1, FindingOrdinals: []int{1}, FactHandles: hs,
		Supports: []ActionSupportBasis{
			{
				SupportKey: "1", Ordinal: 1, Role: "REQUIRED_SUPPORT", Claim: "Цена подтверждает действие.",
				FindingOrdinals: []int{1}, FactHandles: hs,
				Revalidation: SupportRevalidationSpec{EvidenceType: "SKU_PRICE_HISTORY", Subject: target},
			},
			{
				SupportKey: "2", Ordinal: 2, Role: "REQUIRED_SUPPORT", Claim: "Продажи подтверждают действие.",
				FindingOrdinals: []int{1}, FactHandles: hs,
				Revalidation: SupportRevalidationSpec{EvidenceType: "SKU_SALES_HISTORY", Subject: target},
			},
		},
	}, true
}

func firstHandle(p PayloadV1) string {
	if len(p.FactRegistry) > 0 {
		return p.FactRegistry[0].Handle
	}
	return ""
}

func firstMenuAction(m actioncatalog.Menu) (actioncatalog.MenuItem, bool) {
	for _, it := range m.Items {
		if it.ActionType == actioncatalog.TypeSKUAdjustPrice && len(it.EligibleTargetIDs) > 0 {
			return it, true
		}
	}
	for _, it := range m.Items {
		if len(it.EligibleTargetIDs) > 0 {
			return it, true
		}
	}
	return actioncatalog.MenuItem{}, false
}

func ensureMenuActions(p *PayloadV1, actions []ActionProposal) {
	for _, a := range actions {
		found := false
		for i := range p.ActionMenu.Items {
			it := &p.ActionMenu.Items[i]
			if it.ActionType != a.ActionType {
				continue
			}
			for _, id := range it.EligibleTargetIDs {
				if id == a.Target.ID {
					found = true
					break
				}
			}
			if !found {
				it.EligibleTargetIDs = append(it.EligibleTargetIDs, a.Target.ID)
				found = true
			}
			break
		}
		if !found {
			p.ActionMenu.Items = append(p.ActionMenu.Items, actioncatalog.MenuItem{
				ActionType:        a.ActionType,
				TargetKind:        string(a.Target.Kind),
				EligibleTargetIDs: []string{a.Target.ID},
			})
		}
	}
}

func priceMenuItem(p PayloadV1) (actioncatalog.MenuItem, bool) {
	for _, it := range p.ActionMenu.Items {
		if it.ActionType == actioncatalog.TypeSKUAdjustPrice && len(it.EligibleTargetIDs) > 0 {
			return it, true
		}
	}
	if len(p.DiscoveryResult.Threads) > 0 && p.DiscoveryResult.Threads[0].PrimaryEntity.ID != "" {
		ent := p.DiscoveryResult.Threads[0].PrimaryEntity
		return actioncatalog.MenuItem{
			ActionType:        actioncatalog.TypeSKUAdjustPrice,
			TargetKind:        string(ent.Kind),
			EligibleTargetIDs: []string{ent.ID},
		}, true
	}
	return actioncatalog.MenuItem{}, false
}

func fixtureParams(item actioncatalog.MenuItem, p PayloadV1) (actioncatalog.Parameters, EntityRef, bool) {
	id := item.EligibleTargetIDs[0]
	target := EntityRef{Kind: discovery.EntityKind(item.TargetKind), ID: id}
	var pr actioncatalog.Parameters
	switch item.ActionType {
	case actioncatalog.TypeSKUAdjustPrice:
		st := skuPrice(p, id)
		amt := 1000.0
		cur := "RUB"
		if st != nil {
			amt = st.Amount + 10
			if strings.EqualFold(strings.TrimSpace(os.Getenv("DEEP_FAKE_PRICE_INTENT")), "DECREASE") {
				amt = st.Amount - 10
				if amt <= 0 {
					amt = 1
				}
			} else if amt == st.Amount {
				amt = st.Amount + 1
			}
			cur = st.Currency
		}
		amt = float64(int64(amt))
		pr.RecommendedPrice = &actioncatalog.Money{Amount: amt, Currency: cur}
		return pr, target, true
	case actioncatalog.TypeSKUIncreaseReplenishment:
		return pr, target, true
	default:
		return pr, target, false
	}
}

func skuPrice(p PayloadV1, id string) *actioncatalog.Money {
	for _, item := range p.ActionMenu.Items {
		for _, c := range item.HardConstraints {
			if strings.HasPrefix(c, "current_price:"+id+"=") {
				return parseAmtCur(strings.TrimPrefix(c, "current_price:"+id+"="))
			}
		}
	}
	return nil
}

func fixturePreviousAssessments(p PayloadV1, handle string, hasNewAction bool) []PreviousActionAssessment {
	out := make([]PreviousActionAssessment, 0, len(p.Continuity.PreviousActions))
	assessment := envAssessment()
	resulting := []int{}
	if hasNewAction {
		resulting = []int{1}
	}
	for _, pa := range p.Continuity.PreviousActions {
		sas := make([]PreviousSupportAssessment, 0, len(pa.RequiredSupports))
		for _, sp := range pa.RequiredSupports {
			hs := continuityFactHandles(p, pa.CaseRef, sp.SupportKey)
			if len(hs) == 0 {
				hs = continuityAnyHandles(p, handle)
			}
			status := "HOLDS"
			if len(hs) == 0 {
				status = "NOT_VERIFIABLE"
				hs = []string{}
			}
			sas = append(sas, PreviousSupportAssessment{
				SupportKey: sp.SupportKey, Status: status, CurrentFactHandles: hs,
				AssessmentSummary: "Основание оценено по текущим фактам пакета.",
			})
		}
		out = append(out, PreviousActionAssessment{
			PreviousActionRef: pa.PreviousActionRef, Assessment: assessment,
			AssessmentSummary:     "Предыдущее действие оценено по текущим фактам.",
			SupportingFactHandles: continuityAnyHandles(p, handle), CounterFactHandles: []string{},
			Limitations: []string{}, ResultingActionProposalOrdinals: resulting,
			SupportAssessments: sas,
		})
	}
	return out
}

func continuityFactHandles(p PayloadV1, caseRef, supportKey string) []string {
	for _, link := range p.ContinuitySupportEvidenceLinks {
		if link.PreviousCaseRef == caseRef && link.SupportKey == supportKey {
			for _, it := range p.EvidenceRegistry {
				if it.EvidenceID == link.EvidenceID {
					for _, f := range p.FactRegistry {
						if f.SourceKind == "EVIDENCE" && f.SourceRef == it.EvidenceID {
							return []string{f.Handle}
						}
					}
				}
			}
		}
	}
	return nil
}

func continuityAnyHandles(p PayloadV1, fallback string) []string {
	for _, f := range p.FactRegistry {
		if strings.HasPrefix(f.Handle, "ef_") {
			return []string{f.Handle}
		}
	}
	if fallback != "" {
		return []string{fallback}
	}
	return []string{}
}
