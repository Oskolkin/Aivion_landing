package jobs

import (
	"context"
	"regexp"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncdomain"
)

var legacyImportDedupDomainPattern = regexp.MustCompile(`^(.+)__dedup_\d+$`)

func resolveImportJobExecutionTarget(ctx context.Context, q *dbgen.Queries, importJobID int64) (dbgen.ImportJob, error) {
	job, err := q.GetImportJobByID(ctx, importJobID)
	if err != nil {
		return dbgen.ImportJob{}, err
	}
	if job.SupersededByImportJobID.Valid && job.SupersededByImportJobID.Int64 > 0 {
		return q.GetImportJobByID(ctx, job.SupersededByImportJobID.Int64)
	}
	if job.Status == syncdomain.ImportStatusSuperseded.String() {
		if job.SupersededByImportJobID.Valid && job.SupersededByImportJobID.Int64 > 0 {
			return q.GetImportJobByID(ctx, job.SupersededByImportJobID.Int64)
		}
	}
	if matches := legacyImportDedupDomainPattern.FindStringSubmatch(job.Domain); len(matches) == 2 {
		canonical, cerr := q.GetImportJobBySyncJobAndDomain(ctx, dbgen.GetImportJobBySyncJobAndDomainParams{
			SyncJobID: job.SyncJobID,
			Domain:    matches[1],
		})
		if cerr == nil {
			return canonical, nil
		}
	}
	return job, nil
}

func legacyImportDedupBaseDomain(domain string) string {
	if matches := legacyImportDedupDomainPattern.FindStringSubmatch(domain); len(matches) == 2 {
		return matches[1]
	}
	return strings.TrimSpace(domain)
}
