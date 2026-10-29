package devseed

import (
	"math/rand"
	"testing"
)

func TestMVPSeedFunnelMonotonic(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for kind := mvpFunnelDayNormal; kind <= mvpFunnelDayHighTrafficLowOrders; kind++ {
		impr, views, cart, ord, completed, _ := mvpSynthesizeFunnelMetrics(kind, 50, 10000, rng)
		if impr < views || views < cart || cart < ord || ord < completed {
			t.Fatalf("kind %d: not monotonic impr=%d views=%d cart=%d ord=%d completed=%d",
				kind, impr, views, cart, ord, completed)
		}
	}
}

func TestMVPSeedFunnelOrdersMatchAccount(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for _, accountOrders := range []int64{0, 1, 50, 120} {
		impr, views, cart, ord, completed, rev := mvpSynthesizeFunnelMetrics(mvpFunnelDayNormal, accountOrders, 1000, rng)
		if ord != accountOrders {
			t.Fatalf("funnel orders=%d must equal account orders=%d", ord, accountOrders)
		}
		if accountOrders == 0 {
			if cart != 0 || completed != 0 || rev != 0 {
				t.Fatalf("zero account orders must keep zero cart/completed/revenue")
			}
			continue
		}
		if impr < views || views < cart || cart < ord || ord < completed {
			t.Fatalf("not monotonic for orders=%d", accountOrders)
		}
	}
}
