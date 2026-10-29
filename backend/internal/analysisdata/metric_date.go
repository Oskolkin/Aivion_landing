package analysisdata

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// DateMode selects how the analysis metric date is resolved.
type DateMode string

const (
	DateModeCurrent DateMode = "current"
	DateModeFixed   DateMode = "fixed"
)

// Resolution reasons returned by ResolveAnalysisMetricDate.
const (
	ReasonCurrentDateComplete                 = "current_date_complete"
	ReasonCurrentDateAdjustedToLatestComplete = "current_date_adjusted_to_latest_complete"
	ReasonFixedDateAccepted                   = "fixed_date_accepted"
	ReasonFixedDateIncomplete                 = "fixed_date_incomplete"
	ReasonNoCompleteData                      = "no_complete_data"
	ReasonFixedDateInFuture                   = "fixed_date_in_future"
)

// Error code for fixed-mode rejection before provider call.
const ErrCodeRequestedFixedDateIncomplete = "requested_fixed_date_incomplete"

// DefaultTimezone is used when the caller does not specify a seller timezone.
const DefaultTimezone = "Europe/Moscow"

// DefaultStaleLagDays marks data as stale when freshness lag is at least this many days.
const DefaultStaleLagDays = 3

// DefaultCoverageLookbackDays is how far back we search for a complete data date.
const DefaultCoverageLookbackDays = 90

// Required coverage dataset names (Analysis Pack inputs).
const (
	DatasetDailySKUMetrics   = "daily_sku_metrics"
	DatasetProductAnalytics  = "product_analytics"
	DatasetOrdersSales       = "orders_sales" // daily_account_metrics
	DatasetStocks            = "stocks"
	DatasetAdvertising       = "advertising"
	DatasetFunnelAccount     = "funnel_account_metrics"
	DatasetCampaignMetrics   = "campaign_metrics"
	DatasetProductEnrichment = "product_enrichment"
)

// DatasetCoverage describes presence of one required Analysis Pack dataset on a candidate date.
type DatasetCoverage struct {
	Dataset      string `json:"dataset"`
	Present      bool   `json:"present"`
	Required     bool   `json:"required"`
	RowCountHint int    `json:"row_count_hint,omitempty"`
	LatestDate   string `json:"latest_date,omitempty"`
	Status       string `json:"status"` // present|missing|optional_absent
	Detail       string `json:"detail,omitempty"`
}

// MetricDateResolution is the output of ResolveAnalysisMetricDate.
//
// Freshness has two distinct dimensions that must never be conflated:
//   - FreshnessLagDays/IsStale always describe SOURCE staleness — how many
//     days the latest complete underlying data (LatestCompleteDataDate) lags
//     behind Today, regardless of date_mode. This is what should ever
//     trigger a "data not updated" note.
//   - AnalysisAgeDays/HistoricalAnalysis describe how far the *chosen*
//     analysis date (EffectiveMetricDate) is from Today. For a deliberate
//     fixed/historical analysis this is expected to be large and must NOT
//     by itself be treated as staleness.
type MetricDateResolution struct {
	DateMode                DateMode          `json:"date_mode"`
	RequestedDate           string            `json:"requested_date"`
	EffectiveMetricDate     string            `json:"effective_metric_date,omitempty"`
	LatestAvailableDate     string            `json:"latest_available_date,omitempty"`
	LatestCompleteDataDate  string            `json:"latest_complete_data_date,omitempty"`
	FreshnessLagDays        int               `json:"freshness_lag_days"`
	IsStale                 bool              `json:"is_stale"`
	HistoricalAnalysis      bool              `json:"historical_analysis"`
	AnalysisAgeDays         int               `json:"analysis_age_days"`
	SourceFreshnessLagDays  int               `json:"source_freshness_lag_days"`
	MissingRequiredDatasets []string          `json:"missing_required_datasets,omitempty"`
	IncompleteDatasets      []string          `json:"incomplete_datasets,omitempty"`
	Coverage                []DatasetCoverage `json:"coverage,omitempty"`
	ResolutionReason        string            `json:"resolution_reason"`
	Timezone                string            `json:"timezone"`
	Today                   string            `json:"today"`
	Allowed                 bool              `json:"allowed"`
	ErrorCode               string            `json:"error_code,omitempty"`
	PublishAsCurrentDefault bool              `json:"publish_as_current_default"`
}

// ResolveAnalysisMetricDateInput is the input to ResolveAnalysisMetricDate.
type ResolveAnalysisMetricDateInput struct {
	SellerAccountID     int64
	DateMode            DateMode
	RequestedMetricDate string // YYYY-MM-DD; required for fixed, ignored for current
	Timezone            string
	Now                 time.Time // optional; defaults to time.Now
	StaleLagDays        int       // optional; defaults to DefaultStaleLagDays
	LookbackDays        int       // optional; defaults to DefaultCoverageLookbackDays
}

// CoverageSource loads per-dataset date presence for completeness checks.
type CoverageSource interface {
	// DatasetDates returns distinct YYYY-MM-DD dates with data in [from, to] inclusive.
	DatasetDates(ctx context.Context, sellerAccountID int64, dataset string, from, to time.Time) ([]string, error)
	// HasCampaigns reports whether the seller has any advertising campaigns.
	HasCampaigns(ctx context.Context, sellerAccountID int64) (bool, error)
}

// ResolveAnalysisMetricDate picks effective_metric_date for current/fixed modes.
func ResolveAnalysisMetricDate(ctx context.Context, src CoverageSource, in ResolveAnalysisMetricDateInput) (*MetricDateResolution, error) {
	if src == nil {
		return nil, fmt.Errorf("analysisdata: CoverageSource is required")
	}
	if in.SellerAccountID <= 0 {
		return nil, fmt.Errorf("%w", ErrInvalidSellerAccountID)
	}
	mode := DateMode(strings.ToLower(strings.TrimSpace(string(in.DateMode))))
	if mode == "" {
		mode = DateModeCurrent
	}
	if mode != DateModeCurrent && mode != DateModeFixed {
		return nil, fmt.Errorf("analysisdata: unsupported date_mode %q", in.DateMode)
	}

	tzName := strings.TrimSpace(in.Timezone)
	if tzName == "" {
		tzName = DefaultTimezone
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, fmt.Errorf("analysisdata: timezone %q: %w", tzName, err)
	}

	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	today := DateOnlyUTC(now.In(loc))
	todayStr := FormatDate(today)

	staleLag := in.StaleLagDays
	if staleLag <= 0 {
		staleLag = DefaultStaleLagDays
	}
	lookback := in.LookbackDays
	if lookback <= 0 {
		lookback = DefaultCoverageLookbackDays
	}
	from := today.AddDate(0, 0, -(lookback - 1))

	presence, err := loadPresenceMaps(ctx, src, in.SellerAccountID, from, today)
	if err != nil {
		return nil, err
	}

	latestAvailable := latestDateAcross(presence.availableAny)
	latestComplete := latestDateInSet(presence.complete, todayStr)

	out := &MetricDateResolution{
		DateMode:                mode,
		Timezone:                tzName,
		Today:                   todayStr,
		LatestAvailableDate:     latestAvailable,
		LatestCompleteDataDate:  latestComplete,
		PublishAsCurrentDefault: mode == DateModeCurrent,
	}

	switch mode {
	case DateModeCurrent:
		out.RequestedDate = todayStr
		out.HistoricalAnalysis = false
		if latestComplete == "" {
			out.Allowed = false
			out.ResolutionReason = ReasonNoCompleteData
			out.MissingRequiredDatasets = requiredMissingOn(presence, todayStr)
			out.IncompleteDatasets = out.MissingRequiredDatasets
			out.Coverage = coverageForDate(presence, todayStr)
			return out, nil
		}
		out.EffectiveMetricDate = latestComplete
		out.Allowed = true
		out.Coverage = coverageForDate(presence, latestComplete)
		// In current mode the analysis date IS the latest complete source
		// date, so source staleness and analysis age are the same number.
		lag := daysBetween(ParseDate(latestComplete), today)
		out.FreshnessLagDays = lag
		out.SourceFreshnessLagDays = lag
		out.AnalysisAgeDays = lag
		out.IsStale = lag >= staleLag
		if latestComplete == todayStr {
			out.ResolutionReason = ReasonCurrentDateComplete
		} else {
			out.ResolutionReason = ReasonCurrentDateAdjustedToLatestComplete
		}
		return out, nil

	case DateModeFixed:
		requested := strings.TrimSpace(in.RequestedMetricDate)
		if requested == "" {
			return nil, fmt.Errorf("analysisdata: metric_date is required for date_mode=fixed")
		}
		reqTime := ParseDate(requested)
		if reqTime.IsZero() {
			return nil, fmt.Errorf("analysisdata: metric_date: expected YYYY-MM-DD")
		}
		requested = FormatDate(reqTime)
		out.RequestedDate = requested
		out.PublishAsCurrentDefault = false
		// A fixed-date request is, by definition, a deliberate historical
		// analysis — this must never be inferred from lag alone.
		out.HistoricalAnalysis = true
		out.AnalysisAgeDays = daysBetween(reqTime, today)

		if requested > todayStr {
			out.Allowed = false
			out.ResolutionReason = ReasonFixedDateInFuture
			out.ErrorCode = ErrCodeRequestedFixedDateIncomplete
			out.Coverage = coverageForDate(presence, requested)
			out.MissingRequiredDatasets = requiredDatasetNames()
			out.IncompleteDatasets = out.MissingRequiredDatasets
			return out, nil
		}

		// SourceFreshnessLagDays/FreshnessLagDays/IsStale always reflect how
		// stale the underlying SOURCE data is (latest complete data date vs
		// today) — never how far the requested/effective analysis date is
		// from today (that is AnalysisAgeDays above).
		if latestComplete != "" {
			sourceLag := daysBetween(ParseDate(latestComplete), today)
			out.SourceFreshnessLagDays = sourceLag
			out.FreshnessLagDays = sourceLag
			out.IsStale = sourceLag >= staleLag
		}

		cov := coverageForDate(presence, requested)
		out.Coverage = cov
		missing := missingRequired(cov)
		if len(missing) > 0 {
			out.Allowed = false
			out.ResolutionReason = ReasonFixedDateIncomplete
			out.ErrorCode = ErrCodeRequestedFixedDateIncomplete
			out.MissingRequiredDatasets = missing
			out.IncompleteDatasets = missing
			return out, nil
		}

		out.EffectiveMetricDate = requested
		out.Allowed = true
		out.ResolutionReason = ReasonFixedDateAccepted
		return out, nil
	}

	return nil, fmt.Errorf("analysisdata: unreachable date_mode")
}

type presenceMaps struct {
	byDataset    map[string]map[string]int // dataset -> date -> count hint
	complete     map[string]struct{}
	availableAny map[string]struct{}
	hasCampaigns bool
}

func loadPresenceMaps(ctx context.Context, src CoverageSource, sellerID int64, from, to time.Time) (*presenceMaps, error) {
	pm := &presenceMaps{
		byDataset:    map[string]map[string]int{},
		complete:     map[string]struct{}{},
		availableAny: map[string]struct{}{},
	}
	hasCampaigns, err := src.HasCampaigns(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	pm.hasCampaigns = hasCampaigns

	for _, ds := range allProbeDatasets() {
		dates, err := src.DatasetDates(ctx, sellerID, ds, from, to)
		if err != nil {
			return nil, fmt.Errorf("coverage %s: %w", ds, err)
		}
		m := make(map[string]int, len(dates))
		for _, d := range dates {
			m[d]++
			pm.availableAny[d] = struct{}{}
		}
		pm.byDataset[ds] = m
	}

	// Union stock signals: dedicated snapshots OR sku metrics that imply stock day.
	stockDates := map[string]struct{}{}
	for d := range pm.byDataset[DatasetStocks] {
		stockDates[d] = struct{}{}
	}
	// SKU metric days also count as stock-capable days when dedicated snapshots are absent.
	for d := range pm.byDataset[DatasetDailySKUMetrics] {
		stockDates[d] = struct{}{}
	}
	pm.byDataset[DatasetStocks] = countMapFromSet(stockDates)

	for d := range pm.availableAny {
		if dateIsComplete(pm, d) {
			pm.complete[d] = struct{}{}
		}
	}
	return pm, nil
}

func dateIsComplete(pm *presenceMaps, date string) bool {
	required := []string{
		DatasetOrdersSales,
		DatasetDailySKUMetrics,
		DatasetProductAnalytics,
		DatasetFunnelAccount,
		DatasetStocks,
		DatasetProductEnrichment,
	}
	for _, ds := range required {
		if pm.byDataset[ds][date] <= 0 {
			return false
		}
	}
	// Advertising: required only when the seller has campaigns; then we need
	// either campaign_metrics on that day OR accept optional_absent only when
	// no campaigns exist. Campaign list presence without daily metrics is still
	// acceptable (sparse ads days are real).
	if pm.hasCampaigns {
		// Soft requirement: campaigns exist, but a day without ad spend is still
		// analyzable. Mark campaign_metrics present if any row, else optional.
		_ = pm.byDataset[DatasetCampaignMetrics][date]
		_ = pm.byDataset[DatasetAdvertising][date]
	}
	return true
}

func coverageForDate(pm *presenceMaps, date string) []DatasetCoverage {
	specs := []struct {
		name     string
		required bool
	}{
		{DatasetOrdersSales, true},
		{DatasetDailySKUMetrics, true},
		{DatasetProductAnalytics, true},
		{DatasetFunnelAccount, true},
		{DatasetStocks, true},
		{DatasetAdvertising, false},
		{DatasetCampaignMetrics, false},
		{DatasetProductEnrichment, true},
	}
	out := make([]DatasetCoverage, 0, len(specs))
	for _, sp := range specs {
		count := pm.byDataset[sp.name][date]
		present := count > 0
		status := "missing"
		if present {
			status = "present"
		} else if !sp.required {
			status = "optional_absent"
		}
		latest := latestInMap(pm.byDataset[sp.name])
		out = append(out, DatasetCoverage{
			Dataset:      sp.name,
			Present:      present,
			Required:     sp.required,
			RowCountHint: count,
			LatestDate:   latest,
			Status:       status,
		})
	}
	return out
}

func missingRequired(cov []DatasetCoverage) []string {
	var missing []string
	for _, c := range cov {
		if c.Required && !c.Present {
			missing = append(missing, c.Dataset)
		}
	}
	sort.Strings(missing)
	return missing
}

func requiredMissingOn(pm *presenceMaps, date string) []string {
	return missingRequired(coverageForDate(pm, date))
}

func requiredDatasetNames() []string {
	return []string{
		DatasetOrdersSales,
		DatasetDailySKUMetrics,
		DatasetProductAnalytics,
		DatasetFunnelAccount,
		DatasetStocks,
		DatasetProductEnrichment,
	}
}

func allProbeDatasets() []string {
	return []string{
		DatasetOrdersSales,
		DatasetDailySKUMetrics,
		DatasetProductAnalytics,
		DatasetFunnelAccount,
		DatasetStocks,
		DatasetAdvertising,
		DatasetCampaignMetrics,
		DatasetProductEnrichment,
	}
}

func latestDateAcross(set map[string]struct{}) string {
	latest := ""
	for d := range set {
		if d > latest {
			latest = d
		}
	}
	return latest
}

func latestDateInSet(set map[string]struct{}, maxInclusive string) string {
	latest := ""
	for d := range set {
		if d > maxInclusive {
			continue
		}
		if d > latest {
			latest = d
		}
	}
	return latest
}

func latestInMap(m map[string]int) string {
	latest := ""
	for d, n := range m {
		if n <= 0 {
			continue
		}
		if d > latest {
			latest = d
		}
	}
	return latest
}

func countMapFromSet(set map[string]struct{}) map[string]int {
	out := make(map[string]int, len(set))
	for d := range set {
		out[d] = 1
	}
	return out
}

func daysBetween(earlier, later time.Time) int {
	e := DateOnlyUTC(earlier)
	l := DateOnlyUTC(later)
	if e.IsZero() || l.IsZero() {
		return 0
	}
	if l.Before(e) {
		return 0
	}
	return int(l.Sub(e).Hours() / 24)
}
