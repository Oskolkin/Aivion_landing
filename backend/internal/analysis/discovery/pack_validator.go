package discovery

import (
	"regexp"
	"strings"
)

var piiPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)authorization:`),
	regexp.MustCompile(`(?i)api[_-]?key`),
	regexp.MustCompile(`(?i)bearer\s+[a-z0-9\._\-]+`),
	regexp.MustCompile(`(?i)sk-[a-z0-9]{10,}`),
}

var leakagePatterns = []string{
	"materiality_score", "fusion_utility", "lane_rank", "reason_codes",
	"selected_as_anchor", "selection_reason", "diversity_penalty", "rrf",
	"seller_account_id", "universe_id", "factual_universe_id",
	"authorization", "api_key",
}

func ValidatePack(pack PackArtifactV1, handles FactHandleMapV1, idx PackEntityIndex, payloadBytes []byte) error {
	if pack.FactualUniverseHash == "" || pack.MaterialitySelectionHash == "" || pack.LandscapeDetailHash == "" {
		return packErr(CodePackInvalid, "missing input hashes")
	}
	acct, ok := idx.Lookup(KindAccount, "account")
	if !ok || !acct.Present {
		return packErr(CodePackInvalid, "account missing from pack entity index")
	}
	seen := map[string]bool{}
	mapped := handles.ByHandle()
	for _, e := range handles.Entries {
		if e.Handle == "" || seen[e.Handle] {
			return packErr(CodePackInvalid, "duplicate or empty fact handle")
		}
		seen[e.Handle] = true
		if _, ok := mapped[e.Handle]; !ok {
			return packErr(CodePackInvalid, "handle missing from map")
		}
		if len(e.FactRefs) == 0 && (e.Derivation == nil || len(e.Derivation.InputFactRefs) == 0) {
			return packErr(CodePackInvalid, "ungrounded fact handle "+e.Handle)
		}
	}
	visible := CollectVisibleFactHandles(pack.Payload)
	visibleByHandle := map[string]VisibleFact{}
	for _, v := range visible {
		if v.Handle == "" {
			return packErr(CodePackInvalid, "visible factual object missing fact_handle")
		}
		if _, ok := mapped[v.Handle]; !ok {
			return packErr(CodePackInvalid, "visible fact_handle missing from map: "+v.Handle)
		}
		if _, dup := visibleByHandle[v.Handle]; dup {
			return packErr(CodePackInvalid, "duplicate visible fact_handle "+v.Handle)
		}
		visibleByHandle[v.Handle] = v
	}
	for _, e := range handles.Entries {
		if _, ok := visibleByHandle[e.Handle]; !ok {
			return packErr(CodePackInvalid, "fact handle not visible in payload: "+e.Handle)
		}
	}
	if len(visible) != len(handles.Entries) {
		return packErr(CodePackInvalid, "visible factual objects must equal usable fact handles")
	}
	raw := strings.ToLower(string(payloadBytes))
	for _, p := range leakagePatterns {
		if strings.Contains(raw, p) {
			return packErr(CodePackInvalid, "AI payload leakage: "+p)
		}
	}
	for _, re := range piiPatterns {
		if re.Match(payloadBytes) {
			return packErr(CodePackInvalid, "PII or credential pattern in payload")
		}
	}
	if strings.Contains(raw, "canonical_skus") || strings.Contains(raw, "sales_demand_daily") {
		return packErr(CodePackInvalid, "SQL table name in payload")
	}
	return nil
}
