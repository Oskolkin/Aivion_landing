package aidatapack

func availabilitySplitStock(rows []StockPositionRow) DataPackAvailabilitySplit {
	return countDataStatus(len(rows), func(i int) string { return rows[i].DataStatus })
}

func availabilitySplitEnrichment(rows []ProductEnrichmentRow) DataPackAvailabilitySplit {
	return countDataStatus(len(rows), func(i int) string { return rows[i].DataStatus })
}

func availabilitySplitRollups(rows []SKURollupRow) DataPackAvailabilitySplit {
	out := DataPackAvailabilitySplit{Total: len(rows)}
	for _, row := range rows {
		switch row.ProductEnrichmentStatus {
		case "not_available":
			out.NotAvailable++
		default:
			out.Available++
		}
	}
	return out
}

func availabilitySplitCampaigns(rows []CampaignRollupRow) DataPackAvailabilitySplit {
	out := DataPackAvailabilitySplit{Total: len(rows), Available: len(rows)}
	return out
}

func availabilitySplit(rows []StockPositionRow) DataPackAvailabilitySplit {
	return availabilitySplitStock(rows)
}

func countDataStatus(total int, statusAt func(int) string) DataPackAvailabilitySplit {
	out := DataPackAvailabilitySplit{Total: total}
	for i := 0; i < total; i++ {
		switch statusAt(i) {
		case "available":
			out.Available++
		default:
			out.NotAvailable++
		}
	}
	return out
}
