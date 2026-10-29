package devseed

import "testing"

func TestDeriveStockScenarioFromFacts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		stock    int32
		coverage float64
		orders   int32
		want     stockScenario
	}{
		{"out_of_stock", 0, 0, 5, stockScenarioWaitingForSupply},
		{"critical", 5, 2, 10, stockScenarioDeficit},
		{"surplus", 400, 90, 4, stockScenarioSurplus},
		{"no_sales", 50, 0, 0, stockScenarioNoSales},
		{"healthy", 80, 25, 5, stockScenarioActual},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveStockScenarioFromFacts(tc.stock, tc.coverage, tc.orders, tc.orders)
			if got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}
