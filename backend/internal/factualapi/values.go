package factualapi

import (
	"encoding/json"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/sales"
	"time"
)

type ValueState string

const (
	ValueKnown         ValueState = "KNOWN"
	ValueUnavailable   ValueState = "UNAVAILABLE"
	ValueUnknown       ValueState = "UNKNOWN"
	ValueNotApplicable ValueState = "NOT_APPLICABLE"
	ValueNoDenominator ValueState = "NO_DENOMINATOR"
)

type Value[T any] struct {
	State      ValueState `json:"state"`
	Value      *T         `json:"value,omitempty"`
	ReasonCode string     `json:"reason_code,omitempty"`
}

func Known[T any](v T) Value[T] { return Value[T]{State: ValueKnown, Value: &v} }
func Missing[T any](s ValueState, reason string) Value[T] {
	return Value[T]{State: s, ReasonCode: reason}
}

type Decimal = sales.Decimal

func ParseDecimal(v string) (Decimal, error) { return sales.ParseAmount(json.Number(v)) }

type Money struct {
	Amount       Decimal `json:"amount"`
	CurrencyCode string  `json:"currency_code"`
}

type Limitation struct{ Code, Message string }
type Period struct{ From, To time.Time }
type FactMeta struct {
	Readiness                string
	Period                   *Period
	AsOfAt                   *time.Time
	CoverageFrom, CoverageTo *time.Time
	Limitations              []Limitation
	FactRef                  string
}

type StandardPeriod string

const (
	PeriodAnalysisDay StandardPeriod = "ANALYSIS_DAY"
	PeriodPreviousDay StandardPeriod = "PREVIOUS_DAY"
	PeriodCurrent7D   StandardPeriod = "CURRENT_7D"
	PeriodPrevious7D  StandardPeriod = "PREVIOUS_7D"
	PeriodTrend30D    StandardPeriod = "TREND_30D"
)

func LastCompletedBusinessDate(cutoff time.Time) time.Time {
	loc, _ := time.LoadLocation("Europe/Moscow")
	local := cutoff.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -1)
}

func ResolvePeriod(ctx FactualReadContext, standard StandardPeriod, explicit *Period) (Period, error) {
	if explicit != nil {
		p := Period{dateOnly(explicit.From), dateOnly(explicit.To)}
		if p.From.After(p.To) || p.To.After(ctx.AnalysisDate()) {
			return Period{}, NewError(CodePeriodUnsupported, "explicit period is invalid or exceeds analysis date")
		}
		return p, nil
	}
	d := ctx.AnalysisDate()
	var from, to time.Time
	switch standard {
	case PeriodAnalysisDay:
		from, to = d, d
	case PeriodPreviousDay:
		from, to = d.AddDate(0, 0, -1), d.AddDate(0, 0, -1)
	case PeriodCurrent7D:
		from, to = d.AddDate(0, 0, -6), d
	case PeriodPrevious7D:
		from, to = d.AddDate(0, 0, -13), d.AddDate(0, 0, -7)
	case PeriodTrend30D:
		from, to = d.AddDate(0, 0, -29), d
	default:
		return Period{}, NewError(CodePeriodUnsupported, "unsupported standard period")
	}
	return Period{from, to}, nil
}
