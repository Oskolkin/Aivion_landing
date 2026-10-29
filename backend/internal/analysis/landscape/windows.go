package landscape

import (
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

// FactualPeriodPair is the source-native current/previous window for one fact family.
type FactualPeriodPair struct {
	Current  factualapi.Period
	Previous factualapi.Period
}

// ResolveFactFamilyPeriodPair resolves 7d current/previous windows anchored at a fact family's coverage_to.
func ResolveFactFamilyPeriodPair(sealed factualapi.FactualReadContext, domain, factFamily string) (FactualPeriodPair, error) {
	b, ok := sealed.Binding(domain, factFamily)
	if !ok {
		return FactualPeriodPair{}, fmt.Errorf("fact family binding missing: %s/%s", domain, factFamily)
	}
	if b.CoverageTo == nil {
		return FactualPeriodPair{}, fmt.Errorf("fact family coverage_to missing: %s/%s", domain, factFamily)
	}
	anchor := dateOnlyUTC(*b.CoverageTo)
	return FactualPeriodPair{
		Current: factualapi.Period{
			From: anchor.AddDate(0, 0, -6),
			To:   anchor,
		},
		Previous: factualapi.Period{
			From: anchor.AddDate(0, 0, -13),
			To:   anchor.AddDate(0, 0, -7),
		},
	}, nil
}

func dateOnlyUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ResolveBlockPeriodsV3 resolves named periods for one block from its PeriodSpec list.
func ResolveBlockPeriodsV3(sealed factualapi.FactualReadContext, spec BlockSpecV3) (map[string]factualapi.Period, error) {
	out := make(map[string]factualapi.Period, len(spec.Periods))
	for _, ps := range spec.Periods {
		switch ps.Anchor {
		case PeriodAnchorAnalysisDate:
			cur, err := factualapi.ResolvePeriod(sealed, factualapi.PeriodCurrent7D, nil)
			if err != nil {
				return nil, err
			}
			switch ps.Name {
			case "CURRENT_7D":
				out[ps.Name] = cur
			case "PREVIOUS_7D":
				prev, err := factualapi.ResolvePeriod(sealed, factualapi.PeriodPrevious7D, nil)
				if err != nil {
					return nil, err
				}
				out[ps.Name] = prev
			case "TREND_30D":
				trend, err := factualapi.ResolvePeriod(sealed, factualapi.PeriodTrend30D, nil)
				if err != nil {
					return nil, err
				}
				out[ps.Name] = trend
			case "ANALYSIS_DAY":
				day, err := factualapi.ResolvePeriod(sealed, factualapi.PeriodAnalysisDay, nil)
				if err != nil {
					return nil, err
				}
				out[ps.Name] = day
			case "PREVIOUS_DAY":
				day, err := factualapi.ResolvePeriod(sealed, factualapi.PeriodPreviousDay, nil)
				if err != nil {
					return nil, err
				}
				out[ps.Name] = day
			default:
				out[ps.Name] = cur
			}
		case PeriodAnchorFactFamilyCoverageTo:
			if ps.FactFamily == "" {
				return nil, fmt.Errorf("period %s missing fact_family", ps.Name)
			}
			domain := factFamilyDomain(ps.FactFamily)
			pair, err := ResolveFactFamilyPeriodPair(sealed, domain, ps.FactFamily)
			if err != nil {
				return nil, err
			}
			switch ps.Name {
			case "PREVIOUS_7D":
				out[ps.Name] = pair.Previous
			default:
				out[ps.Name] = pair.Current
			}
		case PeriodAnchorSnapshot:
			cutoff := dateOnlyUTC(sealed.CutoffAt())
			out[ps.Name] = factualapi.Period{From: cutoff, To: cutoff}
		default:
			return nil, fmt.Errorf("unsupported period anchor %q for %s", ps.Anchor, ps.Name)
		}
	}
	return out, nil
}

func factFamilyDomain(family string) string {
	switch {
	case strings.HasPrefix(family, "SALES."):
		return "SALES_DEMAND"
	case strings.HasPrefix(family, "STOCK."):
		return "STOCK"
	case strings.HasPrefix(family, "PRICE."):
		return "PRICE"
	case strings.HasPrefix(family, "ADVERTISING."):
		return "ADVERTISING"
	case strings.HasPrefix(family, "SEARCH."):
		return "SEARCH"
	case strings.HasPrefix(family, "FUNNEL."):
		return "FUNNEL"
	case strings.HasPrefix(family, "FINANCE."):
		return "FINANCE"
	case strings.HasPrefix(family, "PROMOTIONS."):
		return "PROMOTIONS"
	case strings.HasPrefix(family, "SUPPLY."):
		return "SUPPLY"
	case strings.HasPrefix(family, "ACCOUNT_HEALTH.") || strings.HasPrefix(family, "HEALTH."):
		return "ACCOUNT_HEALTH"
	case strings.HasPrefix(family, "CATALOG."):
		return "CATALOG"
	default:
		parts := strings.SplitN(family, ".", 2)
		if len(parts) > 0 {
			return parts[0]
		}
		return family
	}
}
