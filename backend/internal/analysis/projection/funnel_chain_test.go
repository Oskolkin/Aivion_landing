package projection

import "testing"

func TestFunnelPickChainDoesNotInventBroadMetrics(t *testing.T) {
	avail := map[string]bool{"BROAD_IMPRESSIONS": true, "PDP_IMPRESSIONS": true, "PDP_CART_ADDS": true}
	got := pickFunnelChain(avail)
	if len(got) != 2 || got[0] != "PDP_IMPRESSIONS" || got[1] != "PDP_CART_ADDS" {
		t.Fatalf("%v", got)
	}
}

func TestFunnelNoCompatibleChain(t *testing.T) {
	if pickFunnelChain(map[string]bool{"BROAD_IMPRESSIONS": true}) != nil {
		t.Fatal("broad-only is not a compatible presentation chain")
	}
}
