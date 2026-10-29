package testozon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"
)

const (
	TestClientID         = "DEMO_CLIENT_ID"
	TestAPIKey           = "DEMO_API_KEY"
	TestPerformanceToken = "DEMO_PERFORMANCE_TOKEN"

	TestProductID   int64 = 1001
	TestSKU         int64 = 1001
	TestCampaignID  int64 = 5001
	TestWarehouseID int64 = 77
)

// Options configures deterministic Ozon fake API responses.
type Options struct {
	MetricDate time.Time
}

// Servers hosts httptest servers for seller and performance APIs.
type Servers struct {
	Seller      *httptest.Server
	Performance *httptest.Server
}

// Start spins up seller + performance fake APIs. Caller must call Close on both servers.
func Start(opts Options) *Servers {
	if opts.MetricDate.IsZero() {
		opts.MetricDate = time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	}
	metricDay := opts.MetricDate.UTC().Format("2006-01-02")
	orderTS := opts.MetricDate.UTC().Format(time.RFC3339)

	seller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/seller/info":
			writeJSON(w, map[string]any{"company_name": "Test Seller", "seller_id": 1})
		case "/v1/roles":
			writeJSON(w, map[string]any{
				"expires_at": "2026-12-31T23:59:59Z",
				"roles": []any{
					map[string]any{
						"name": "Admin",
						"methods": []any{
							"/v3/product/list",
							"/v3/product/info/list",
							"/v1/analytics/stocks",
							"/v1/analytics/data",
						},
					},
				},
			})
		case "/v3/product/list":
			writeJSON(w, map[string]any{
				"result": map[string]any{
					"items": []any{
						map[string]any{
							"id":         TestProductID,
							"name":       "Runtime Test Product",
							"offer_id":   "OFFER-1001",
							"sku":        TestSKU,
							"status":     "price_sent",
							"updated_at": orderTS,
						},
					},
					"last_id": "",
					"total":   1,
				},
			})
		case "/v3/product/info/list":
			writeJSON(w, map[string]any{
				"items": []any{
					map[string]any{
						"id":                      TestProductID,
						"name":                    "Runtime Test Product",
						"offer_id":                "OFFER-1001",
						"sku":                     TestSKU,
						"price":                   "999.00",
						"description_category_id": 17036108,
						"updated_at":              orderTS,
						"stocks": map[string]any{
							"has_stock": true,
							"stocks": []any{
								map[string]any{"present": 12, "reserved": 1, "sku": TestSKU},
							},
						},
					},
				},
			})
		case "/v3/posting/fbs/list":
			writeJSON(w, map[string]any{
				"result": map[string]any{
					"postings": []any{
						map[string]any{
							"order_id":       9001,
							"posting_number": "POST-9001",
							"status":         "delivered",
							"created_at":     orderTS,
							"in_process_at":  orderTS,
							"financial_data": map[string]any{
								"products": []any{
									map[string]any{"price": "999.00", "quantity": 1, "currency_code": "RUB"},
								},
							},
						},
					},
					"has_next": false,
				},
			})
		case "/v3/finance/transaction/list":
			writeJSON(w, map[string]any{
				"result": map[string]any{
					"operations": []any{
						map[string]any{
							"operation_id":   "1",
							"operation_type": "OperationAgentDeliveredToCustomer",
							"operation_date": orderTS,
							"amount":         "999.00",
							"currency_code":  "RUB",
							"posting": map[string]any{
								"posting_number": "POST-9001",
								"order_id":       "9001",
							},
							"items": []any{
								map[string]any{"sku": TestSKU, "name": "Runtime Test Product", "quantity": 1},
							},
						},
					},
				},
			})
		case "/v1/product/info/warehouse/stocks":
			writeJSON(w, map[string]any{
				"stocks": []any{
					map[string]any{
						"product_id":   TestProductID,
						"offer_id":     "OFFER-1001",
						"sku":          TestSKU,
						"warehouse_id": TestWarehouseID,
						"present":      12,
						"reserved":     1,
						"free_stock":   11,
						"updated_at":   orderTS,
					},
				},
				"has_next": false,
				"cursor":   "",
			})
		case "/v1/product/related-sku/get":
			writeJSON(w, map[string]any{
				"items": []any{
					map[string]any{
						"sku":             TestSKU,
						"product_id":      TestProductID,
						"delivery_schema": "FBO",
						"availability":    "AVAILABLE",
					},
				},
				"errors": []any{},
			})
		case "/v1/analytics/stocks":
			writeJSON(w, map[string]any{
				"items": []any{
					map[string]any{
						"sku":                   TestSKU,
						"offer_id":              "OFFER-1001",
						"name":                  "Runtime Test Product",
						"warehouse_id":          TestWarehouseID,
						"warehouse_name":        "WH-MOSCOW",
						"available_stock_count": 11,
						"turnover_grade":        "ACTUAL",
						"item_tags":             []any{},
					},
				},
			})
		case "/v1/analytics/data":
			writeJSON(w, map[string]any{
				"result": map[string]any{
					"data": []any{
						map[string]any{
							"dimensions": []any{
								map[string]any{"id": metricDay, "name": metricDay},
							},
							"metrics": []float64{100, 80, 20, 5, 4, 4500},
						},
					},
					"totals": []float64{100, 80, 20, 5, 4, 4500},
				},
			})
		case "/v1/description-category/tree":
			writeJSON(w, map[string]any{"result": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))

	performance := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/api/client/campaign":
			writeJSON(w, map[string]any{
				"list": []any{
					map[string]any{
						"id":            TestCampaignID,
						"title":         "Runtime SKU Campaign",
						"advObjectType": "SKU",
						"state":         "CAMPAIGN_STATE_RUNNING",
					},
				},
			})
		case path == "/api/client/statistics/daily/json":
			writeJSON(w, map[string]any{
				"rows": []any{
					map[string]any{
						"campaign_id":    TestCampaignID,
						"date":           metricDay,
						"impressions":    1200,
						"clicks":         48,
						"spend":          1500000000,
						"orders":         2,
						"ordersInRubles": 1998000000,
					},
				},
			})
		case strings.HasPrefix(path, "/api/client/campaign/") && strings.HasSuffix(path, "/v2/products"):
			parts := strings.Split(strings.Trim(path, "/"), "/")
			campaignID := TestCampaignID
			if len(parts) >= 4 {
				if id, err := strconv.ParseInt(parts[3], 10, 64); err == nil {
					campaignID = id
				}
			}
			_ = campaignID
			writeJSON(w, map[string]any{
				"products": []any{
					map[string]any{
						"sku":       TestSKU,
						"title":     "Runtime Test Product",
						"is_active": true,
						"status":    "active",
					},
				},
			})
		case path == "/api/client/campaign/search_promo/v2/products":
			writeJSON(w, map[string]any{"products": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))

	return &Servers{Seller: seller, Performance: performance}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
