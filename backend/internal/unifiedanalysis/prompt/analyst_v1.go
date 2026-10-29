package prompt

import (
	"strings"
)

// AppendPackIdentityInput appends exact pack identity values to the stable input
// template. These values are not part of the prompt hash (hash covers the template only).
func AppendPackIdentityInput(baseInput, packID, packHash, metricDate string) string {
	baseInput = strings.TrimSpace(baseInput)
	packID = strings.TrimSpace(packID)
	packHash = strings.TrimSpace(packHash)
	metricDate = strings.TrimSpace(metricDate)
	if packID == "" && packHash == "" && metricDate == "" {
		return baseInput
	}
	var b strings.Builder
	b.WriteString(baseInput)
	b.WriteString("\n\nExact analysis_meta values (copy verbatim into the structured output):\n")
	if packID != "" {
		b.WriteString("pack_id=")
		b.WriteString(packID)
		b.WriteByte('\n')
	}
	if packHash != "" {
		b.WriteString("pack_hash=")
		b.WriteString(packHash)
		b.WriteByte('\n')
	}
	if metricDate != "" {
		b.WriteString("metric_date=")
		b.WriteString(metricDate)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

// AppendAccountSummaryGroundingInput appends exact Pack account_summary numbers
// for seller_pulse metric_claims. Not part of the prompt hash.
func AppendAccountSummaryGroundingInput(baseInput string, lines []string) string {
	baseInput = strings.TrimSpace(baseInput)
	if len(lines) == 0 {
		return baseInput
	}
	var b strings.Builder
	b.WriteString(baseInput)
	b.WriteString("\n\nExact seller_pulse Pack facts (copy value/fact_id verbatim into metric_claims and numeric key_points/important_facts; never invent weekly totals):\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString("period_type current_7d = last 7 days ending metric_date; previous_7d = the 7 days before that.\n")
	b.WriteString("account:drr:current_7d, account:ad_spend_share_of_total_revenue_pct:current_7d, and account:account_drr_pct:current_7d are the SAME store-level share (ad_spend/store_revenue*100). Attributed campaign DRR is account:ad_spend_share_of_attributed_revenue_pct:current_7d (ad_spend/ad_revenue*100) — never mix denominators.\n")
	b.WriteString("section_fact_policy_v1: Seller Pulse may use ONLY these account:* facts. Campaign facts (campaign:<id>:drr_pct / campaign_drr_pct) belong in ad_efficiency / Actions / health entity evidence — never in seller_pulse claims or important_facts.\n")
	return strings.TrimSpace(b.String())
}

// AppendEntityIDExamplesInput appends a small allowlist of real Pack entity IDs
// so the model anchors Action scopes to literal CSV values. Not part of the
// prompt hash (same policy as AppendPackIdentityInput).
func AppendEntityIDExamplesInput(baseInput string, skuIDs, campaignIDs []string) string {
	baseInput = strings.TrimSpace(baseInput)
	skuIDs = compactNonEmpty(skuIDs, 12)
	campaignIDs = compactNonEmpty(campaignIDs, 12)
	if len(skuIDs) == 0 && len(campaignIDs) == 0 {
		return baseInput
	}
	var b strings.Builder
	b.WriteString(baseInput)
	b.WriteString("\n\nExact Pack entity_id examples (copy verbatim when referencing these objects; never invent aliases):\n")
	if len(skuIDs) > 0 {
		b.WriteString("example_sku_ids=")
		b.WriteString(strings.Join(skuIDs, ","))
		b.WriteByte('\n')
	}
	if len(campaignIDs) > 0 {
		b.WriteString("example_campaign_ids=")
		b.WriteString(strings.Join(campaignIDs, ","))
		b.WriteByte('\n')
	}
	b.WriteString("All Action scope.entity_ids and step.entity_ids MUST appear as literal sku_id / campaign_id values in the Pack CSVs.\n")
	return strings.TrimSpace(b.String())
}

func compactNonEmpty(ids []string, limit int) []string {
	out := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
