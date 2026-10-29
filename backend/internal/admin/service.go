package admin

import (
	"context"
	"strings"
)

type Service struct {
	repo Repository

	ingestionService       IngestionRerunner
	metricsService         MetricsRerunner
	alertsService          AlertsRerunner
	recommendationsService RecommendationsRerunner
	cursorService          CursorResetter
}

type ServiceDeps struct {
	Repo Repository

	IngestionService       IngestionRerunner
	MetricsService         MetricsRerunner
	AlertsService          AlertsRerunner
	RecommendationsService RecommendationsRerunner
	CursorService          CursorResetter
}

func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Repo == nil {
		return nil, ErrRepositoryRequired
	}
	return &Service{
		repo:                   deps.Repo,
		ingestionService:       deps.IngestionService,
		metricsService:         deps.MetricsService,
		alertsService:          deps.AlertsService,
		recommendationsService: deps.RecommendationsService,
		cursorService:          deps.CursorService,
	}, nil
}

func (s *Service) ListClients(ctx context.Context, filter ClientListFilter) (*ClientListResult, error) {
	filterPage := NormalizePage(filter.Limit, filter.Offset, 50, 200)
	filter.Limit = filterPage.Limit
	filter.Offset = filterPage.Offset
	return s.repo.ListClients(ctx, filter)
}

func (s *Service) GetClientDetail(ctx context.Context, sellerAccountID int64) (*ClientDetail, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	overview, err := s.repo.GetClientOverview(ctx, sellerAccountID)
	if err != nil {
		return nil, err
	}
	connections, err := s.repo.GetClientConnections(ctx, sellerAccountID)
	if err != nil {
		return nil, err
	}
	operational, err := s.GetOperationalStatus(ctx, sellerAccountID)
	if err != nil {
		return nil, err
	}
	billing, err := s.repo.GetBillingState(ctx, sellerAccountID)
	if err != nil {
		return nil, err
	}
	return &ClientDetail{
		Overview:          *overview,
		Connections:       connections,
		OperationalStatus: *operational,
		Billing:           billing,
	}, nil
}

func (s *Service) GetOperationalStatus(ctx context.Context, sellerAccountID int64) (*OperationalStatus, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	page := NormalizePage(10, 0, 10, 50)

	syncJobs, err := s.repo.ListSyncJobs(ctx, sellerAccountID, SyncJobFilter{}, page)
	if err != nil {
		return nil, err
	}
	importJobs, err := s.repo.ListImportJobs(ctx, sellerAccountID, ImportJobFilter{}, page)
	if err != nil {
		return nil, err
	}
	alertRuns, err := s.repo.ListAlertRuns(ctx, sellerAccountID, NormalizePage(1, 0, 1, 10))
	if err != nil {
		return nil, err
	}
	recommendationRuns, err := s.repo.ListRecommendationRuns(ctx, sellerAccountID, RecommendationRunLogFilter{}, NormalizePage(1, 0, 1, 10))
	if err != nil {
		return nil, err
	}
	chatTraces, err := s.repo.ListChatTraces(ctx, sellerAccountID, ChatTraceFilter{}, NormalizePage(1, 0, 1, 10))
	if err != nil {
		return nil, err
	}

	status := &OperationalStatus{
		LatestImportJobs: importJobs,
		Limitations:      []string{"open alerts/recommendations count is not yet directly sourced in admin repository"},
	}
	if len(syncJobs) > 0 {
		job := syncJobs[0]
		status.LatestSyncJob = &job
	}
	if len(alertRuns) > 0 {
		run := alertRuns[0]
		status.LatestAlertRun = &run
	}
	if len(recommendationRuns) > 0 {
		run := recommendationRuns[0]
		status.LatestRecommendationRun = &run
	}
	if len(chatTraces) > 0 {
		trace := chatTraces[0]
		status.LatestChatTrace = &trace
	}

	return status, nil
}

func (s *Service) GetFeedbackSummary(ctx context.Context, sellerAccountID int64) (*FeedbackSummary, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	chatFeedback, err := s.repo.ListChatFeedback(ctx, ChatFeedbackFilter{SellerAccountID: &sellerAccountID}, NormalizePage(200, 0, 200, 200))
	if err != nil {
		return nil, err
	}
	recommendationRuns, err := s.repo.ListRecommendationRuns(ctx, sellerAccountID, RecommendationRunLogFilter{}, NormalizePage(200, 0, 200, 200))
	if err != nil {
		return nil, err
	}

	summary := &FeedbackSummary{}
	for _, item := range chatFeedback {
		switch item.Rating {
		case "positive":
			summary.ChatPositive++
		case "negative":
			summary.ChatNegative++
		case "neutral":
			summary.ChatNeutral++
		}
	}
	for _, run := range recommendationRuns {
		summary.RecommendationAccepted += int64(run.AcceptedRecommendationsCount)
	}
	return summary, nil
}

func (s *Service) ListSyncJobs(ctx context.Context, sellerAccountID int64, filter SyncJobFilter) (*SyncJobListResult, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	filter.Status = strings.TrimSpace(filter.Status)
	page := NormalizePage(filter.Limit, filter.Offset, 50, 200)
	items, err := s.repo.ListSyncJobs(ctx, sellerAccountID, filter, page)
	if err != nil {
		return nil, err
	}
	return &SyncJobListResult{Items: items, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *Service) ListImportJobs(ctx context.Context, sellerAccountID int64, filter ImportJobFilter) (*ImportJobListResult, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Domain = strings.TrimSpace(filter.Domain)
	page := NormalizePage(filter.Limit, filter.Offset, 50, 200)
	items, err := s.repo.ListImportJobs(ctx, sellerAccountID, filter, page)
	if err != nil {
		return nil, err
	}
	return &ImportJobListResult{Items: items, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *Service) ListImportErrors(ctx context.Context, sellerAccountID int64, filter ImportErrorFilter) (*ImportErrorListResult, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Domain = strings.TrimSpace(filter.Domain)
	page := NormalizePage(filter.Limit, filter.Offset, 50, 200)
	items, err := s.repo.ListImportErrors(ctx, sellerAccountID, filter, page)
	if err != nil {
		return nil, err
	}
	return &ImportErrorListResult{Items: items, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *Service) ListSyncCursors(ctx context.Context, sellerAccountID int64, filter SyncCursorFilter) (*SyncCursorListResult, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	filter.Domain = strings.TrimSpace(filter.Domain)
	page := NormalizePage(filter.Limit, filter.Offset, 50, 200)
	items, err := s.repo.ListSyncCursors(ctx, sellerAccountID, filter, page)
	if err != nil {
		return nil, err
	}
	return &SyncCursorListResult{Items: items, Limit: page.Limit, Offset: page.Offset}, nil
}
