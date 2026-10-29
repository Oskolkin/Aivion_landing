package dashboardhome

import "time"

type sparklineDayMetrics struct {
	orders     float64
	revenue    float64
	hasOrders  bool
	hasRevenue bool
}

type dailyRatioPoint struct {
	value   float64
	hasData bool
}

// buildCalendarSparklineSeries returns one point per UTC day in [from, to].
// Value is set only when a metric exists for that day; missing days keep Value nil (not zero).
func buildCalendarSparklineSeries(from, to time.Time, valueByDate map[string]float64) []SparklinePoint {
	points := make([]SparklinePoint, 0, 8)
	for d := from.UTC(); !d.After(to.UTC()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		pt := SparklinePoint{Date: key}
		if v, ok := valueByDate[key]; ok {
			pt.HasData = true
			pt.Value = &v
		}
		points = append(points, pt)
	}
	return points
}

// buildCalendarOrdersRevenueSparklineSeries returns a dense daily series for SKU focus charts.
// Every calendar day in [from, to] is included. Real zeros are preserved; missing DB rows use has_data=false.
func buildCalendarOrdersRevenueSparklineSeries(from, to time.Time, metricsByDate map[string]sparklineDayMetrics) []SparklinePoint {
	points := make([]SparklinePoint, 0, 8)
	zero := 0.0
	for d := from.UTC(); !d.After(to.UTC()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		pt := SparklinePoint{Date: key}
		if m, ok := metricsByDate[key]; ok && m.hasOrders {
			pt.HasData = true
			v := m.orders
			pt.Value = &v
			if m.hasRevenue {
				r := m.revenue
				pt.Revenue = &r
			}
		} else {
			pt.HasData = false
			pt.Value = &zero
		}
		points = append(points, pt)
	}
	return points
}

func buildAOVSparklineSeries(from, to time.Time, metricsByDate map[string]sparklineDayMetrics) []SparklinePoint {
	points := make([]SparklinePoint, 0, 8)
	for d := from.UTC(); !d.After(to.UTC()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		pt := SparklinePoint{Date: key}
		if m, ok := metricsByDate[key]; ok && m.hasOrders && m.hasRevenue && m.orders > 0 {
			v := m.revenue / m.orders
			pt.HasData = true
			pt.Value = &v
		}
		points = append(points, pt)
	}
	return points
}

func buildRatioSparklineSeries(from, to time.Time, ratioByDate map[string]dailyRatioPoint) []SparklinePoint {
	points := make([]SparklinePoint, 0, 8)
	for d := from.UTC(); !d.After(to.UTC()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		pt := SparklinePoint{Date: key}
		if row, ok := ratioByDate[key]; ok && row.hasData {
			v := row.value
			pt.HasData = true
			pt.Value = &v
		}
		points = append(points, pt)
	}
	return points
}
