package pack

import (
	"sort"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// campaignWindowSums aggregates ad_metrics_daily for one campaign over a date range.
type campaignWindowSums struct {
	Impressions int64
	Clicks      int64
	Spend       float64
	Orders      int64
	Revenue     float64
}

func sumCampaignWindow(idx *Indexes, campaignID int64, from, to string) campaignWindowSums {
	var out campaignWindowSums
	byDate := idx.CampaignMetricsByDate[campaignID]
	for _, d := range dateRangeInclusive(from, to) {
		day, ok := byDate[d]
		if !ok {
			continue
		}
		out.Impressions += day.Impressions
		out.Clicks += day.Clicks
		out.Spend += day.Spend
		out.Orders += day.Orders
		out.Revenue += day.Revenue
	}
	return out
}

// BuildCampaignSummaryTable renders campaign_summary.csv rows in campaign-id-ascending order.
func BuildCampaignSummaryTable(idx *Indexes, periods Periods) csvTable {
	rows := make([][]string, 0, len(idx.CampaignUniverse))
	for _, cid := range idx.CampaignUniverse {
		m := idx.CampaignMeta[cid]
		current := sumCampaignWindow(idx, cid, periods.Current7D.From, periods.Current7D.To)
		previous := sumCampaignWindow(idx, cid, periods.Previous7D.From, periods.Previous7D.To)

		spendAbs := DeltaAbs(current.Spend, previous.Spend)
		spendPct := DeltaPct(current.Spend, previous.Spend)
		revAbs := DeltaAbs(current.Revenue, previous.Revenue)
		revPct := DeltaPct(current.Revenue, previous.Revenue)
		ordersAbs := DeltaAbs(float64(current.Orders), float64(previous.Orders))
		ordersPct := DeltaPct(float64(current.Orders), float64(previous.Orders))

		ctr := CTRPct(current.Clicks, current.Impressions)
		cpc := CPC(current.Spend, current.Clicks)
		conversion := ConversionPct(float64(current.Clicks), float64(current.Orders))
		drr := DRRPct(current.Spend, current.Revenue)

		rows = append(rows, []string{
			"campaign:" + strconv.FormatInt(cid, 10),
			strconv.FormatInt(cid, 10),
			m.Name,
			m.Type,
			m.Status,
			FormatMoneyCSV(current.Spend),
			FormatMoneyCSV(previous.Spend),
			FormatMoneyCSV(spendAbs),
			FormatPctCSV(spendPct),
			FormatMoneyCSV(current.Revenue),
			FormatMoneyCSV(previous.Revenue),
			FormatMoneyCSV(revAbs),
			FormatPctCSV(revPct),
			FormatIntCSV(current.Orders),
			FormatIntCSV(previous.Orders),
			FormatFloatCSV(&ordersAbs),
			FormatPctCSV(ordersPct),
			FormatIntCSV(current.Impressions),
			FormatIntCSV(current.Clicks),
			FormatPctCSV(ctr),
			FormatFloatCSV(cpc),
			FormatPctCSV(conversion),
			FormatPctCSV(drr),
			FormatIntCSV(int64(idx.CampaignSKUCount[cid])),
		})
	}
	return csvTable{Name: "campaign_summary.csv", Columns: contract.CampaignSummaryColumnsV1, Rows: rows}
}

// BuildCampaignDailyTable renders campaign_daily.csv rows sorted by (campaign, date) ascending.
// Only dates with an observed ad_metrics_daily row are emitted (sparse daily series).
func BuildCampaignDailyTable(idx *Indexes) csvTable {
	var rows [][]string
	for _, cid := range idx.CampaignUniverse {
		byDate := idx.CampaignMetricsByDate[cid]
		if len(byDate) == 0 {
			continue
		}
		dates := make([]string, 0, len(byDate))
		for d := range byDate {
			dates = append(dates, d)
		}
		sort.Strings(dates)
		for _, d := range dates {
			day := byDate[d]
			ctr := CTRPct(day.Clicks, day.Impressions)
			cpc := CPC(day.Spend, day.Clicks)
			conversion := ConversionPct(float64(day.Clicks), float64(day.Orders))
			drr := DRRPct(day.Spend, day.Revenue)
			rows = append(rows, []string{
				"campaign:" + strconv.FormatInt(cid, 10) + ":" + d,
				strconv.FormatInt(cid, 10),
				d,
				FormatMoneyCSV(day.Spend),
				FormatMoneyCSV(day.Revenue),
				FormatIntCSV(day.Orders),
				FormatIntCSV(day.Impressions),
				FormatIntCSV(day.Clicks),
				FormatPctCSV(ctr),
				FormatFloatCSV(cpc),
				FormatPctCSV(conversion),
				FormatPctCSV(drr),
			})
		}
	}
	return csvTable{Name: "campaign_daily.csv", Columns: contract.CampaignDailyColumnsV1, Rows: rows}
}

// BuildCampaignSKUTable renders campaign_sku.csv link rows sorted by (campaign, sku) ascending.
// Per-pair metrics are not attributable from ad_campaign_skus and are always left null.
func BuildCampaignSKUTable(idx *Indexes, limitations *limitationCollector) csvTable {
	rows := make([][]string, 0, len(idx.CampaignSKULinks))
	for _, link := range idx.CampaignSKULinks {
		if link.SKU == "" {
			continue
		}
		limitations.Add(LimitationCampaignSKUSpendUnknown)
		rows = append(rows, []string{
			"campaign:" + strconv.FormatInt(link.CampaignExternalID, 10) + ":sku:" + link.SKU,
			strconv.FormatInt(link.CampaignExternalID, 10),
			link.SKU,
			"", // spend_7d
			"", // revenue_7d
			"", // orders_7d
			"", // impressions_7d
			"", // clicks_7d
			"", // ctr_pct
			"", // conversion_pct
			"", // drr_pct
		})
	}
	return csvTable{Name: "campaign_sku.csv", Columns: contract.CampaignSKUColumnsV1, Rows: rows}
}
