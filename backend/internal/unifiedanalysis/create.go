package unifiedanalysis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
)

// CreateRunOptions controls CreateRun date-mode metadata and publish defaults.
type CreateRunOptions struct {
	Force            bool
	DateResolution   *analysisdata.MetricDateResolution
	PublishAsCurrent *bool  // nil → derive from date_mode defaults
	Trigger          string // empty → TriggerManual
}

// CreateRun creates a new unified_analysis run for seller/metricDate.
// When force is false and an active run exists, returns ErrActiveRunExists.
func (s *Service) CreateRun(ctx context.Context, sellerID int64, metricDate time.Time, force bool) (*Run, error) {
	return s.CreateRunWithOptions(ctx, sellerID, metricDate, CreateRunOptions{Force: force})
}

// CreateRunWithOptions creates a run and optionally persists date-resolution metadata.
func (s *Service) CreateRunWithOptions(ctx context.Context, sellerID int64, metricDate time.Time, opts CreateRunOptions) (*Run, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("unified analysis service is not configured")
	}
	if sellerID <= 0 {
		return nil, fmt.Errorf("seller_account_id must be positive")
	}
	metricDate = time.Date(metricDate.Year(), metricDate.Month(), metricDate.Day(), 0, 0, 0, 0, time.UTC)
	date := toDate(metricDate)

	active, err := s.repo.FindActiveRun(ctx, sellerID, date)
	if err != nil {
		return nil, err
	}
	if active != nil && !opts.Force {
		return nil, ErrActiveRunExists
	}

	now := s.now()
	trigger := strings.TrimSpace(opts.Trigger)
	if trigger == "" {
		trigger = TriggerManual
	}
	run, err := s.repo.CreateRun(ctx, sellerID, date, s.cfg, timestamptzNow(now), trigger)
	if err != nil {
		return nil, err
	}

	if s.artifacts != nil && opts.DateResolution != nil {
		publishAsCurrent := opts.DateResolution.PublishAsCurrentDefault
		if opts.PublishAsCurrent != nil {
			publishAsCurrent = *opts.PublishAsCurrent
		}
		_ = s.artifacts.SaveDateResolution(ctx, run.ID, DateResolutionArtifact{
			Resolution:       *opts.DateResolution,
			PublishAsCurrent: publishAsCurrent,
		})
	}
	return run, nil
}

func (s *Service) now() time.Time {
	if s.clock != nil {
		return s.clock.Now().UTC()
	}
	return time.Now().UTC()
}
