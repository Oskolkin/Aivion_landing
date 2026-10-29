package alerts

import (
	"encoding/json"
	"strings"
	"testing"
)

func baseFBOContext() FBOStorageRiskSKUContext {
	return FBOStorageRiskSKUContext{
		SKU:                 1001,
		OzonProductID:       501,
		AvailableStockCount: 50,
	}
}

func baseFBOInput(ctxs ...FBOStorageRiskSKUContext) StockRuleEvaluationInput {
	bySKU := make(FBOStorageRiskBySKU, len(ctxs))
	for _, c := range ctxs {
		bySKU[c.SKU] = c
	}
	return StockRuleEvaluationInput{
		SellerAccountID:     1,
		AsOfDate:            "2026-04-30",
		FBOStorageRiskBySKU: bySKU,
	}
}

func TestFBOSurplusStorageRiskTurnoverSurplus(t *testing.T) {
	ctx := baseFBOContext()
	ctx.TurnoverGrade = fboTurnoverGradeSurplus
	idc := int32(130)
	ctx.IDC = &idc

	rr, ok := evaluateFBOSurplusStorageRisk(baseFBOInput(ctx), ctx)
	if !ok {
		t.Fatal("expected fbo_surplus_storage_risk")
	}
	if rr.Severity != SeverityMedium && rr.Severity != SeverityHigh {
		t.Fatalf("unexpected severity: %s", rr.Severity)
	}
	if !containsStringReason(rr.EvidencePayload["reasons"], fboReasonTurnoverSurplus) {
		t.Fatalf("expected turnover_grade_surplus reason, got %v", rr.EvidencePayload["reasons"])
	}
}

func TestFBOSurplusStorageRiskExcessStock(t *testing.T) {
	ctx := baseFBOContext()
	ctx.AvailableStockCount = 100
	ctx.ExcessStockCount = 20
	ctx.TurnoverGrade = "ACTUAL"

	rr, ok := evaluateFBOSurplusStorageRisk(baseFBOInput(ctx), ctx)
	if !ok {
		t.Fatal("expected alert for excess_stock_count")
	}
	if !containsStringReason(rr.EvidencePayload["reasons"], fboReasonExcessPositive) {
		t.Fatalf("expected excess reason, got %v", rr.EvidencePayload["reasons"])
	}
}

func TestFBOSurplusStorageRiskIDCGte120(t *testing.T) {
	ctx := baseFBOContext()
	idc := int32(130)
	ctx.IDC = &idc
	ctx.TurnoverGrade = "ACTUAL"

	rr, ok := evaluateFBOSurplusStorageRisk(baseFBOInput(ctx), ctx)
	if !ok {
		t.Fatal("expected alert for idc >= 120")
	}
	if !containsStringReason(rr.EvidencePayload["reasons"], fboReasonIDCGte120) {
		t.Fatalf("expected idc reason, got %v", rr.EvidencePayload["reasons"])
	}
}

func TestFBOSurplusStorageRiskHighSeverityByIDC(t *testing.T) {
	ctx := baseFBOContext()
	idc := int32(190)
	ctx.IDC = &idc

	rr, ok := evaluateFBOSurplusStorageRisk(baseFBOInput(ctx), ctx)
	if !ok || rr.Severity != SeverityHigh {
		t.Fatalf("expected high severity, ok=%v severity=%s", ok, rr.Severity)
	}
}

func TestFBOSurplusStorageRiskHighSeverityByExcessShare(t *testing.T) {
	ctx := baseFBOContext()
	ctx.AvailableStockCount = 100
	ctx.ExcessStockCount = 60
	ctx.HasExcessShare = true
	ctx.ExcessShare = 0.6

	rr, ok := evaluateFBOSurplusStorageRisk(baseFBOInput(ctx), ctx)
	if !ok || rr.Severity != SeverityHigh {
		t.Fatalf("expected high severity, ok=%v severity=%s", ok, rr.Severity)
	}
	if !containsStringReason(rr.EvidencePayload["reasons"], fboReasonExcessShareGte50) {
		t.Fatalf("expected excess share reason, got %v", rr.EvidencePayload["reasons"])
	}
}

func TestFBOSurplusStorageRiskNoAlertZeroStock(t *testing.T) {
	ctx := baseFBOContext()
	ctx.AvailableStockCount = 0
	ctx.TurnoverGrade = fboTurnoverGradeSurplus
	idc := int32(200)
	ctx.IDC = &idc

	if _, ok := evaluateFBOSurplusStorageRisk(baseFBOInput(ctx), ctx); ok {
		t.Fatal("must not alert when available_stock_count is zero")
	}
}

func TestFBOSurplusStorageRiskNoAlertWithoutSignals(t *testing.T) {
	ctx := baseFBOContext()
	ctx.TurnoverGrade = "ACTUAL"
	idc := int32(60)
	ctx.IDC = &idc

	if _, ok := evaluateFBOSurplusStorageRisk(baseFBOInput(ctx), ctx); ok {
		t.Fatal("must not alert without reliable signals")
	}
}

func TestFBOSurplusStorageRiskEvidenceNoNaN(t *testing.T) {
	ctx := baseFBOContext()
	ctx.AvailableStockCount = 100
	ctx.ExcessStockCount = 10
	ctx.HasExcessShare = true
	ctx.ExcessShare = 0.1

	rr, ok := evaluateFBOSurplusStorageRisk(baseFBOInput(ctx), ctx)
	if !ok {
		t.Fatal("expected alert")
	}
	raw, err := json.Marshal(rr.EvidencePayload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "NaN") || strings.Contains(string(raw), "Infinity") {
		t.Fatalf("invalid evidence: %s", raw)
	}
}

func TestFBOSurplusStorageRiskMultipleReasons(t *testing.T) {
	ctx := baseFBOContext()
	ctx.TurnoverGrade = fboTurnoverGradeSurplus
	ctx.ExcessStockCount = 5
	idc := int32(180)
	ctx.IDC = &idc

	rr, ok := evaluateFBOSurplusStorageRisk(baseFBOInput(ctx), ctx)
	if !ok {
		t.Fatal("expected alert")
	}
	reasons, ok := rr.EvidencePayload["reasons"].([]string)
	if !ok || len(reasons) < 2 {
		t.Fatalf("expected multiple reasons, got %v", rr.EvidencePayload["reasons"])
	}
}

func TestFBOSurplusStorageRiskFingerprintStable(t *testing.T) {
	ctx := baseFBOContext()
	ctx.TurnoverGrade = fboTurnoverGradeSurplus
	idc := int32(150)
	ctx.IDC = &idc

	rr, ok := evaluateFBOSurplusStorageRisk(baseFBOInput(ctx), ctx)
	if !ok {
		t.Fatal("expected alert")
	}
	inA := RuleResultToUpsertInput(1, rr)
	inB := RuleResultToUpsertInput(1, rr)
	if inA.Fingerprint != inB.Fingerprint {
		t.Fatal("fingerprint must be stable")
	}
}

func TestEvaluateStockRulesIncludesFBOSurplus(t *testing.T) {
	ctx := baseFBOContext()
	ctx.TurnoverGrade = fboTurnoverGradeSurplus
	idc := int32(130)
	ctx.IDC = &idc

	res := EvaluateStockRules(baseFBOInput(ctx))
	found := false
	for _, rr := range res.RuleResults {
		if rr.AlertType == AlertTypeFBOSurplusStorageRisk {
			found = true
		}
	}
	if !found {
		t.Fatal("EvaluateStockRules must include fbo_surplus_storage_risk")
	}
}

func containsStringReason(v any, want string) bool {
	reasons, ok := v.([]string)
	if !ok {
		return false
	}
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}
