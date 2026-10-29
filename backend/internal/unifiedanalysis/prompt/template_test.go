package prompt

import (
	"strings"
	"testing"
)

// Pinned hashes for published versioned templates. Changing them requires a
// deliberate new version file — do not edit historical bodies in place.
const (
	pinnedAnalystV1_22Hash = "67858013bb6e2f48eeb214338a891e977bd93508410d0a18d09a6e3aa0715180"
	pinnedAnalystV1_24Hash = "fd41711bfb17d783e3efce7e66e7003cb214538662990ae5ff1a2a061aa955d6"
	pinnedAnalystV1_25Hash = "b12f7a6b3745d9b9ece860bc04d77eae3f2f68140fdbdf8e0c055a453eee34fb"
	pinnedAnalystV1_26Hash = "9b0fd12db06cb28acf3505d254b47b73303cdbd6c6b7c876142d58ff807e3308"
	pinnedAnalystV1_27Hash = "ff70ed2316e09c9455a4b13e1a52a0720ef788c65c4fdf22320a59a6b99acc2f"
	pinnedAnalystV1_28Hash = "9edf9789afe1127ef94c3c78d83f20adc7f10105bbc174fb8166e717aa27e966"
	pinnedAnalystV1_29Hash = "0f19a402ebea7d7f5ddd28525ec3eca5e87209118352f6282db1e6a19c2b4c49"
	pinnedAnalystV1_30Hash = "b32dd1e87040218b360d26c02b92f71703bb8012d5b93636c27297f86b5fc21c"
	pinnedAnalystV1_31Hash = "e7dc8b317cbb7eb7ce6dae632cd4487edd19db1a3685f6098ce4f9296ccb1a45"
)

func TestTemplateByVersion_PublishedBodiesDiffer(t *testing.T) {
	t22, err := TemplateByVersion(VersionAnalystV1_22)
	if err != nil {
		t.Fatal(err)
	}
	t24, err := TemplateByVersion(VersionAnalystV1_24)
	if err != nil {
		t.Fatal(err)
	}
	t25, err := TemplateByVersion(VersionAnalystV1_25)
	if err != nil {
		t.Fatal(err)
	}
	t26, err := TemplateByVersion(VersionAnalystV1_26)
	if err != nil {
		t.Fatal(err)
	}
	t27, err := TemplateByVersion(VersionAnalystV1_27)
	if err != nil {
		t.Fatal(err)
	}
	t28, err := TemplateByVersion(VersionAnalystV1_28)
	if err != nil {
		t.Fatal(err)
	}
	t29, err := TemplateByVersion(VersionAnalystV1_29)
	if err != nil {
		t.Fatal(err)
	}
	t30, err := TemplateByVersion(VersionAnalystV1_30)
	if err != nil {
		t.Fatal(err)
	}
	t31, err := TemplateByVersion(VersionAnalystV1_31)
	if err != nil {
		t.Fatal(err)
	}
	if t22.Instructions == t24.Instructions {
		t.Fatal("v1_22 and v1_24 instructions must differ")
	}
	if t24.Instructions == t25.Instructions {
		t.Fatal("v1_24 and v1_25 instructions must differ")
	}
	if t25.Instructions == t26.Instructions {
		t.Fatal("v1_25 and v1_26 instructions must differ")
	}
	if t26.Instructions == t27.Instructions {
		t.Fatal("v1_26 and v1_27 instructions must differ")
	}
	if t27.Instructions == t28.Instructions {
		t.Fatal("v1_27 and v1_28 instructions must differ")
	}
	if t28.Instructions == t29.Instructions {
		t.Fatal("v1_28 and v1_29 instructions must differ")
	}
	if t29.Instructions == t30.Instructions {
		t.Fatal("v1_29 and v1_30 instructions must differ")
	}
	if t30.Instructions == t31.Instructions {
		t.Fatal("v1_30 and v1_31 instructions must differ")
	}
	if t22.Input == t24.Input {
		t.Fatal("v1_22 and v1_24 input must differ")
	}
	if !strings.Contains(t22.Instructions, "aivion_unified_analysis_v8") {
		t.Fatal("v1_22 must target schema v8")
	}
	if !strings.Contains(t24.Instructions, "aivion_unified_analysis_v9") {
		t.Fatal("v1_24 must target schema v9")
	}
	if !strings.Contains(t25.Instructions, "aivion_unified_analysis_v9") {
		t.Fatal("v1_25 must target schema v9")
	}
	if !strings.Contains(t26.Instructions, "aivion_unified_analysis_v10") {
		t.Fatal("v1_26 must target schema v10")
	}
	if !strings.Contains(t27.Instructions, "aivion_unified_analysis_v10") {
		t.Fatal("v1_27 must target schema v10")
	}
	if strings.Contains(t22.Instructions, "ad_efficiency.narrative.metric_claims") {
		t.Fatal("v1_22 must not require ad metric_claims (Wave 2 addition)")
	}
	if !strings.Contains(t24.Instructions, "ad_efficiency.narrative.metric_claims") {
		t.Fatal("v1_24 must require ad metric_claims")
	}
	if !strings.Contains(t25.Instructions, "HARD STORE-SHARE LABELING") {
		t.Fatal("v1_25 must forbid bare ДРР for store share")
	}
	if !strings.Contains(t25.Instructions, "sku:stock_coverage_days_7d:") {
		t.Fatal("v1_25 must document forbidden invented fact_id shapes from run 191")
	}
	if strings.Contains(t25.Instructions, "produce 5–15 Actions") {
		t.Fatal("v1_25 must not require a 5-Action quota")
	}
	if !strings.Contains(t26.Instructions, "threshold_claims") {
		t.Fatal("v1_26 must require threshold_claims")
	}
	if !strings.Contains(t26.Instructions, "high_drr_campaign_threshold") {
		t.Fatal("v1_26 must document high_drr_campaign_threshold")
	}
	if !strings.Contains(t27.Instructions, "Never add a recommended Action merely to fill three positions") {
		t.Fatal("v1_27 must forbid padding Top 3 with recommended")
	}
	if !strings.Contains(t27.Instructions, "NEGATIVE") {
		t.Fatal("v1_27 must include negative Top Actions example")
	}
	if !strings.Contains(t28.Instructions, "Never append SKU, campaign, category, or other entity IDs") {
		t.Fatal("v1_28 must forbid entity-suffixed risk driver codes")
	}
	if !strings.Contains(t28.Instructions, "id=high_drr_campaign") {
		t.Fatal("v1_28 must document canonical high_drr_campaign id")
	}
	if !strings.Contains(t29.Instructions, "SECTION-AWARE FACT GROUNDING") {
		t.Fatal("v1_29 must document section-aware fact grounding")
	}
	if !strings.Contains(t29.Instructions, "campaign:882000021528:DRR") {
		t.Fatal("v1_29 must include forbidden Seller Pulse campaign DRR example")
	}
	if !strings.Contains(t29.Instructions, "ad_spend_share_of_total_store_revenue_pct") {
		t.Fatal("v1_29 must document allowed account-level store ad share fact")
	}
	if !strings.Contains(t30.Instructions, "material impact") {
		t.Fatal("v1_30 must require materiality-ranked Top Actions")
	}
	if !strings.Contains(t30.Instructions, "product_content_score_unavailable") {
		t.Fatal("v1_30 must ban unsupported content recommendations")
	}
	if !strings.Contains(t31.Instructions, "prefer emitting 8–12 Actions") {
		t.Fatal("v1_31 must prefer 8-12 Actions when multiple material problems exist")
	}
	if !strings.Contains(t31.Instructions, "hard max 15") {
		t.Fatal("v1_31 must document the hard max of 15 Actions")
	}
	if !strings.Contains(t31.Instructions, "out_of_stock, critical (0–3 days coverage), short (4–7 days coverage), planning") {
		t.Fatal("v1_31 must require splitting stock Actions by urgency band")
	}
	if !strings.Contains(t31.Instructions, "high-DRR campaigns") || !strings.Contains(t31.Instructions, "spend-without-orders campaigns") {
		t.Fatal("v1_31 must require splitting ads Actions into high-DRR vs spend-without-orders")
	}
	if !strings.Contains(t31.Instructions, "SELLER PULSE NARRATIVE STRUCTURE") {
		t.Fatal("v1_31 must document the Seller Pulse narrative structure")
	}
	if !strings.Contains(t31.Instructions, "MAXIMUM 140 characters") {
		t.Fatal("v1_31 must cap the Seller Pulse headline at 140 characters")
	}
	if !strings.Contains(t31.Instructions, "exactly THREE short paragraphs") {
		t.Fatal("v1_31 must require a 3-paragraph Seller Pulse summary")
	}
	if !strings.Contains(t31.Instructions, "FUNNEL WORDING") || !strings.Contains(t31.Instructions, "«доля завершённых заказов»") {
		t.Fatal("v1_31 must ban «доля завершённых заказов» via FUNNEL WORDING")
	}
	if !strings.Contains(t31.Instructions, "Конверсия из корзины в заказ") || !strings.Contains(t31.Instructions, "Доля доставленных единиц от заказанных") {
		t.Fatal("v1_31 must mandate the exact funnel step wording")
	}
	if !strings.Contains(t31.Instructions, "BANNED PHRASES") || !strings.Contains(t31.Instructions, "продавцы-лидеры") {
		t.Fatal("v1_31 must document the expanded banned-phrase list")
	}
	if !strings.Contains(t31.Instructions, "aivion_unified_analysis_v10") {
		t.Fatal("v1_31 must keep targeting schema v10")
	}
}

func TestHashForVersion_PinnedPublishedHashes(t *testing.T) {
	h22, err := HashForVersion(VersionAnalystV1_22)
	if err != nil {
		t.Fatal(err)
	}
	h24, err := HashForVersion(VersionAnalystV1_24)
	if err != nil {
		t.Fatal(err)
	}
	h25, err := HashForVersion(VersionAnalystV1_25)
	if err != nil {
		t.Fatal(err)
	}
	h26, err := HashForVersion(VersionAnalystV1_26)
	if err != nil {
		t.Fatal(err)
	}
	h27, err := HashForVersion(VersionAnalystV1_27)
	if err != nil {
		t.Fatal(err)
	}
	h28, err := HashForVersion(VersionAnalystV1_28)
	if err != nil {
		t.Fatal(err)
	}
	h29, err := HashForVersion(VersionAnalystV1_29)
	if err != nil {
		t.Fatal(err)
	}
	h30, err := HashForVersion(VersionAnalystV1_30)
	if err != nil {
		t.Fatal(err)
	}
	h31, err := HashForVersion(VersionAnalystV1_31)
	if err != nil {
		t.Fatal(err)
	}
	if h22 != pinnedAnalystV1_22Hash {
		t.Fatalf("v1_22 hash=%s want=%s", h22, pinnedAnalystV1_22Hash)
	}
	if h24 != pinnedAnalystV1_24Hash {
		t.Fatalf("v1_24 hash=%s want=%s", h24, pinnedAnalystV1_24Hash)
	}
	if h25 != pinnedAnalystV1_25Hash {
		t.Fatalf("v1_25 hash=%s want=%s", h25, pinnedAnalystV1_25Hash)
	}
	if pinnedAnalystV1_26Hash == "PLACEHOLDER_COMPUTE" {
		t.Fatalf("set pinnedAnalystV1_26Hash to %s", h26)
	}
	if h26 != pinnedAnalystV1_26Hash {
		t.Fatalf("v1_26 hash=%s want=%s", h26, pinnedAnalystV1_26Hash)
	}
	if h27 != pinnedAnalystV1_27Hash {
		t.Fatalf("v1_27 hash=%s want=%s", h27, pinnedAnalystV1_27Hash)
	}
	if h28 != pinnedAnalystV1_28Hash {
		t.Fatalf("v1_28 hash=%s want=%s", h28, pinnedAnalystV1_28Hash)
	}
	if h29 != pinnedAnalystV1_29Hash {
		t.Fatalf("v1_29 hash=%s want=%s", h29, pinnedAnalystV1_29Hash)
	}
	if h30 != pinnedAnalystV1_30Hash {
		t.Fatalf("v1_30 hash=%s want=%s", h30, pinnedAnalystV1_30Hash)
	}
	if pinnedAnalystV1_31Hash == "PLACEHOLDER_COMPUTE" {
		t.Fatalf("set pinnedAnalystV1_31Hash to %s", h31)
	}
	if h31 != pinnedAnalystV1_31Hash {
		t.Fatalf("v1_31 hash=%s want=%s", h31, pinnedAnalystV1_31Hash)
	}
	if h22 == h24 || h24 == h25 || h25 == h26 || h26 == h27 || h27 == h28 || h28 == h29 || h29 == h30 || h30 == h31 {
		t.Fatal("published version hashes must all differ")
	}
}

func TestTemplateByVersion_V123NeverPublished(t *testing.T) {
	meta, err := RegistryMeta(VersionAnalystV1_23)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Status != PromptStatusNeverPublished {
		t.Fatalf("status=%q want=%q", meta.Status, PromptStatusNeverPublished)
	}
	if _, err := TemplateByVersion(VersionAnalystV1_23); err == nil {
		t.Fatal("v1_23 must not resolve a template body")
	}
	if _, err := HashForVersion(VersionAnalystV1_23); err == nil {
		t.Fatal("v1_23 must not resolve a hash via fallback")
	}
	if _, err := RenderByVersion(VersionAnalystV1_23); err == nil {
		t.Fatal("v1_23 must not render via fallback to current")
	}
}

func TestTemplateByVersion_UnknownErrors(t *testing.T) {
	if _, err := TemplateByVersion("unified_analyst_v9_99"); err == nil {
		t.Fatal("unknown version must error")
	}
	if _, err := RenderByVersion("not_a_prompt"); err == nil {
		t.Fatal("unknown version must error")
	}
}

func TestRenderByVersion_ReplayUsesHistoricalBody(t *testing.T) {
	r22, err := RenderByVersion(VersionAnalystV1_22)
	if err != nil {
		t.Fatal(err)
	}
	r24, err := RenderByVersion(VersionAnalystV1_24)
	if err != nil {
		t.Fatal(err)
	}
	r25, err := RenderByVersion(VersionAnalystV1_25)
	if err != nil {
		t.Fatal(err)
	}
	r26, err := RenderByVersion(VersionAnalystV1_26)
	if err != nil {
		t.Fatal(err)
	}
	r27, err := RenderByVersion(VersionAnalystV1_27)
	if err != nil {
		t.Fatal(err)
	}
	r28, err := RenderByVersion(VersionAnalystV1_28)
	if err != nil {
		t.Fatal(err)
	}
	r29, err := RenderByVersion(VersionAnalystV1_29)
	if err != nil {
		t.Fatal(err)
	}
	r30, err := RenderByVersion(VersionAnalystV1_30)
	if err != nil {
		t.Fatal(err)
	}
	r31, err := RenderByVersion(VersionAnalystV1_31)
	if err != nil {
		t.Fatal(err)
	}
	if r22.Version != VersionAnalystV1_22 || r22.Hash != pinnedAnalystV1_22Hash {
		t.Fatalf("replay v1_22 identity mismatch: %+v", r22)
	}
	if r24.Version != VersionAnalystV1_24 || r24.Hash != pinnedAnalystV1_24Hash {
		t.Fatalf("replay v1_24 identity mismatch: %+v", r24)
	}
	if r25.Version != VersionAnalystV1_25 || r25.Hash != pinnedAnalystV1_25Hash {
		t.Fatalf("replay v1_25 identity mismatch: %+v", r25)
	}
	if r26.Version != VersionAnalystV1_26 || r26.Hash != pinnedAnalystV1_26Hash {
		t.Fatalf("replay v1_26 identity mismatch: %+v", r26)
	}
	if r27.Version != VersionAnalystV1_27 || r27.Hash != pinnedAnalystV1_27Hash {
		t.Fatalf("replay v1_27 identity mismatch: %+v", r27)
	}
	if r28.Version != VersionAnalystV1_28 || r28.Hash != pinnedAnalystV1_28Hash {
		t.Fatalf("replay v1_28 identity mismatch: %+v", r28)
	}
	if r29.Version != VersionAnalystV1_29 || r29.Hash != pinnedAnalystV1_29Hash {
		t.Fatalf("replay v1_29 identity mismatch: %+v", r29)
	}
	if r30.Version != VersionAnalystV1_30 || r30.Hash != pinnedAnalystV1_30Hash {
		t.Fatalf("replay v1_30 identity mismatch: %+v", r30)
	}
	if r31.Version != VersionAnalystV1_31 || r31.Hash != pinnedAnalystV1_31Hash {
		t.Fatalf("replay v1_31 identity mismatch: %+v", r31)
	}
	if r22.Instructions == r30.Instructions || r29.Instructions == r30.Instructions || r30.Instructions == r31.Instructions {
		t.Fatal("historical replay must not substitute bodies across versions")
	}
}

func TestCurrentRenderMatchesV131(t *testing.T) {
	rendered, err := Render(PromptProfileAnalysis)
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Version != CurrentAnalystVersion {
		t.Fatalf("current version=%q want=%q", rendered.Version, CurrentAnalystVersion)
	}
	if CurrentAnalystVersion != VersionAnalystV1_31 {
		t.Fatalf("CurrentAnalystVersion=%q want=%q", CurrentAnalystVersion, VersionAnalystV1_31)
	}
	if rendered.Hash != pinnedAnalystV1_31Hash {
		t.Fatalf("current hash drift: %s", rendered.Hash)
	}
}
