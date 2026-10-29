package migrationpreflight

import (
	"regexp"
	"strings"
	"time"
)

type ImportJobRankInput struct {
	Status          string
	FinishedAt      *time.Time
	RecordsImported int32
	UpdatedAt       time.Time
	CreatedAt       time.Time
	ID              int64
}

var legacyImportDedupDomain = regexp.MustCompile(`^(.+)__dedup_\d+$`)

// ImportJobStatusRank returns lower-is-better rank for canonical selection.
func ImportJobStatusRank(status string) int {
	switch status {
	case "completed":
		return 0
	case "importing":
		return 1
	case "fetching":
		return 2
	case "retrying":
		return 3
	case "pending":
		return 4
	case "failed":
		return 5
	case "superseded":
		return 6
	default:
		return 7
	}
}

// PickCanonicalImportJob chooses deterministic winner among duplicates.
func PickCanonicalImportJob(rows []ImportJobRankInput) ImportJobRankInput {
	if len(rows) == 0 {
		return ImportJobRankInput{}
	}
	best := rows[0]
	for _, row := range rows[1:] {
		if importJobBetter(row, best) {
			best = row
		}
	}
	return best
}

func importJobBetter(a, b ImportJobRankInput) bool {
	ar, br := ImportJobStatusRank(a.Status), ImportJobStatusRank(b.Status)
	if ar != br {
		return ar < br
	}
	af, bf := timeOrZero(a.FinishedAt), timeOrZero(b.FinishedAt)
	if !af.Equal(bf) {
		return af.After(bf)
	}
	if a.RecordsImported != b.RecordsImported {
		return a.RecordsImported > b.RecordsImported
	}
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		return a.UpdatedAt.After(b.UpdatedAt)
	}
	if !a.CreatedAt.Equal(b.CreatedAt) {
		return a.CreatedAt.After(b.CreatedAt)
	}
	return a.ID > b.ID
}

func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.UTC()
}

// LegacyDedupBaseDomain strips __dedup_<id> suffix from legacy loser domains.
func LegacyDedupBaseDomain(domain string) string {
	if matches := legacyImportDedupDomain.FindStringSubmatch(strings.TrimSpace(domain)); len(matches) == 2 {
		return matches[1]
	}
	return domain
}

// ActiveSyncStatusRank returns lower-is-better rank for active sync canonical selection.
func ActiveSyncStatusRank(status string) int {
	switch status {
	case "running":
		return 0
	case "pending":
		return 1
	default:
		return 2
	}
}

// ActionCandidateCount classifies backfill ambiguity.
func ActionCandidateCount(candidates int) string {
	switch {
	case candidates == 0:
		return "no_candidate"
	case candidates == 1:
		return "single_candidate"
	default:
		return "ambiguous"
	}
}
