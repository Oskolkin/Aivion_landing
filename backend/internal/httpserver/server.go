package httpserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/health"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/httpserver/handlers"
	appmw "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/httpserver/middleware"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

type Server struct {
	httpServer *http.Server
}

func New(
	port string,
	httpCfg config.HTTPConfig,
	corsAllowedOrigins []string,
	authRateLimiter *appmw.AuthRateLimiter,
	healthHandler *health.Handler,
	authHandler *handlers.AuthHandler,
	adminHandler *handlers.AdminHandler,
	accountHandler *handlers.AccountHandler,
	chatHandler *handlers.ChatHandler,
	analyticsDashboardHandler *handlers.AnalyticsDashboardHandler,
	dashboardHomeHandler *handlers.DashboardHomeHandler,
	systemHandler *handlers.SystemHandler,
	pricingConstraintsHandler *handlers.PricingConstraintsHandler,
	productsHandler *handlers.ProductsHandler,
	alertsHandler *handlers.AlertsHandler,
	actionsHandler *handlers.ActionsHandler,
	actionViewHandler *handlers.ActionViewHandler,
	recommendationsHandler *handlers.RecommendationsHandler,
	aiSummariesHandler *handlers.AISummariesHandler,
	ozonHandler *handlers.OzonHandler,
	ozonIngestionSyncHandler *handlers.OzonIngestionSyncHandler,
	ozonIngestionStatusHandler *handlers.OzonIngestionStatusHandler,
	ozonEnrichmentHandler *handlers.OzonEnrichmentHandler,
	ozonOAuthCallback *handlers.OzonOAuthCallback,
	aiDataPackHandler *handlers.AIDataPackHandler,
	aiPipelineAdminHandler *handlers.AIPipelineAdminHandler,
	authMiddleware func(http.Handler) http.Handler,
	sellerMiddleware func(http.Handler) http.Handler,
	connectionRequiredMiddleware func(http.Handler) http.Handler,
	adminMiddleware func(http.Handler) http.Handler,
	log *zap.Logger,
	m *metrics.Metrics,
	registry *prometheus.Registry,
) *Server {
	r := chi.NewRouter()

	r.Use(appmw.CORS(corsAllowedOrigins))
	r.Use(appmw.RequestID)
	r.Use(appmw.Recovery(log))
	r.Use(appmw.Logging(log))
	r.Use(appmw.RecoverMaxBytesError)
	bodyLimit := httpCfg.JSONBodyLimitBytes
	if httpCfg.ChatBodyLimitBytes > bodyLimit {
		bodyLimit = httpCfg.ChatBodyLimitBytes
	}
	r.Use(appmw.MaxBytes(bodyLimit))
	r.Use(appmw.Metrics(m))

	r.Get("/health/live", healthHandler.Live)
	r.Get("/health/ready", healthHandler.Ready)
	r.With(appmw.MetricsAuth(httpCfg.MetricsAuthToken, httpCfg.MetricsAllowUnauthenticated)).Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	register := authHandler.Register
	login := authHandler.Login
	if authRateLimiter != nil {
		register = authRateLimiter.WrapRegister(authHandler.Register)
		login = authRateLimiter.WrapLogin(authHandler.Login)
	}
	if !authHandler.PublicRegistrationEnabled() {
		register = authHandler.RegisterDisabled
	}

	r.Post("/api/v1/auth/register", register)
	r.Post("/api/v1/auth/login", login)
	r.Get("/api/v1/auth/public-config", authHandler.PublicConfig)
	r.Get("/api/v1/auth/review/redeem/{token}", authHandler.RedeemReviewInvite)
	r.Get("/api/v1/system/build-info", systemHandler.GetBuildInfo)
	if ozonOAuthCallback != nil {
		r.With(authMiddleware, sellerMiddleware).Get("/oauth/start", ozonOAuthCallback.Start)
		r.Get("/oauth/callback", ozonOAuthCallback.Get)
	}

	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(auth.ReadOnlyGuard())

		r.Get("/api/v1/auth/me", authHandler.Me)
		r.Post("/api/v1/auth/logout", authHandler.Logout)
		r.With(adminMiddleware).Get("/api/v1/admin/me", adminHandler.Me)
		r.With(adminMiddleware).Get("/api/v1/admin/clients", adminHandler.ListClients)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}", adminHandler.GetClientDetail)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/sync-jobs", adminHandler.ListSyncJobs)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/import-jobs", adminHandler.ListImportJobs)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/import-errors", adminHandler.ListImportErrors)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/sync-cursors", adminHandler.ListSyncCursors)
		r.With(adminMiddleware).Post("/api/v1/admin/clients/{seller_account_id}/actions/rerun-sync", adminHandler.RerunSync)
		r.With(adminMiddleware).Post("/api/v1/admin/clients/{seller_account_id}/actions/reset-cursor", adminHandler.ResetCursor)
		r.With(adminMiddleware).Post("/api/v1/admin/clients/{seller_account_id}/actions/rerun-metrics", adminHandler.RerunMetrics)
		r.With(adminMiddleware).Post("/api/v1/admin/clients/{seller_account_id}/actions/rerun-alerts", adminHandler.RerunAlerts)
		r.With(adminMiddleware).Post("/api/v1/admin/clients/{seller_account_id}/actions/rerun-recommendations", adminHandler.RerunRecommendations)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/ai/recommendation-runs", adminHandler.ListRecommendationRuns)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/ai/recommendation-runs/{run_id}", adminHandler.GetRecommendationRunDetail)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/ai/recommendations/{id}", adminHandler.GetRecommendationRawAI)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/ai/chat-traces", adminHandler.ListChatTraces)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/ai/chat-traces/{trace_id}", adminHandler.GetChatTraceDetail)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/chat/sessions", adminHandler.ListChatSessions)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/chat/sessions/{session_id}/messages", adminHandler.ListChatMessages)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/feedback/chat", adminHandler.ListChatFeedbackByClient)
		r.With(adminMiddleware).Get("/api/v1/admin/feedback/chat", adminHandler.ListAllChatFeedback)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/feedback/recommendations", adminHandler.ListRecommendationFeedbackByClient)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/billing", adminHandler.GetBillingStateByClient)
		r.With(adminMiddleware).Put("/api/v1/admin/clients/{seller_account_id}/billing", adminHandler.UpdateBillingStateByClient)
		r.With(adminMiddleware).Get("/api/v1/admin/billing", adminHandler.ListBillingStates)
		r.With(adminMiddleware).Get("/api/v1/admin/actions/generation-runs", actionsHandler.AdminListGenerationRuns)
		r.With(adminMiddleware).Get("/api/v1/admin/actions/generation-runs/{run_id}", actionsHandler.AdminGetGenerationRun)
		r.With(adminMiddleware).Get("/api/v1/admin/clients/{seller_account_id}/actions/id/{id}", actionsHandler.GetHistoricalActionByID)
		r.With(adminMiddleware).Post("/api/v1/admin/actions/rerun", actionsHandler.AdminRerunActions)
		r.With(adminMiddleware).Post("/api/v1/admin/ozon/enrichment/sync", ozonEnrichmentHandler.Sync)
		r.With(adminMiddleware).Get("/api/v1/admin/ozon/enrichment/status", ozonEnrichmentHandler.Status)
		r.With(adminMiddleware).Get("/api/v1/admin/ai/data-pack", aiDataPackHandler.Get)
		if aiPipelineAdminHandler != nil {
			r.With(adminMiddleware).Get("/api/v1/admin/ai-pipeline/{pipeline_run_id}/diagnostics", aiPipelineAdminHandler.GetPipelineDiagnostics)
			r.With(adminMiddleware).Get("/api/v1/admin/ai-pipeline/dispatches/sync/{sync_job_id}", aiPipelineAdminHandler.GetDispatchDiagnostics)
			r.With(adminMiddleware).Post("/api/v1/admin/ai-pipeline/dispatches/{dispatch_id}/retry", aiPipelineAdminHandler.RetryDispatch)
		}

		r.Group(func(r chi.Router) {
			r.Use(sellerMiddleware)

			r.Route("/api/v1/integrations/ozon", func(r chi.Router) {
				// Active seller-facing onboarding (used by frontend ozon-api.ts).
				r.Get("/connection", ozonHandler.GetUserConnection)
				r.Put("/connection", ozonHandler.PutUserConnection)
				r.Post("/disconnect", ozonHandler.DisconnectUserConnection)

				// Legacy/technical Ozon API — admin/support diagnostics only.
				// Not used by seller UI; initial sync is also triggered from PUT /connection when valid.
				r.Group(func(r chi.Router) {
					r.Use(adminMiddleware)
					r.Get("/", ozonHandler.GetConnection)
					r.Post("/", ozonHandler.CreateConnection)
					r.Put("/", ozonHandler.UpdateConnection)
					r.Put("/performance-token", ozonHandler.PutPerformanceToken)
					r.Post("/check", ozonHandler.CheckConnection)
					r.Post("/check-performance", ozonHandler.CheckPerformanceConnection)
					r.Post("/initial-sync", ozonIngestionSyncHandler.StartInitialSync)
					r.Get("/status", ozonIngestionStatusHandler.GetStatus)
				})
			})

			r.Group(func(r chi.Router) {
				r.Use(connectionRequiredMiddleware)

				r.Get("/api/v1/account", accountHandler.GetCurrentAccount)
				r.Post("/api/v1/chat/ask", chatHandler.Ask)
				r.Post("/api/v1/chat/ask/stream", chatHandler.AskStream)
				r.Post("/api/v1/chat/action-handoffs", chatHandler.ActionHandoff)
				r.Get("/api/v1/chat/sessions/by-action", chatHandler.GetSessionByAction)
				r.Get("/api/v1/chat/sessions/by-action/{action_id}", chatHandler.GetSessionByActionID)
				r.Post("/api/v1/chat/sessions/lookup-by-action", chatHandler.LookupSessionByAction)
				r.Get("/api/v1/chat/sessions", chatHandler.ListSessions)
				r.Get("/api/v1/chat/sessions/{id}", chatHandler.GetSession)
				r.Get("/api/v1/chat/sessions/{id}/messages", chatHandler.ListSessionMessages)
				r.Patch("/api/v1/chat/sessions/{id}", chatHandler.RenameSession)
				r.Delete("/api/v1/chat/sessions/{id}", chatHandler.DeleteSession)
				r.Post("/api/v1/chat/sessions/{id}/archive", chatHandler.ArchiveSession)
				r.Post("/api/v1/chat/messages/{id}/feedback", chatHandler.AddMessageFeedback)
				r.Post("/api/v1/chat/attachments", chatHandler.UploadAttachment)
				r.Delete("/api/v1/chat/attachments/{id}", chatHandler.DeleteAttachment)
				r.Get("/api/v1/chat/attachments/{id}/download", chatHandler.DownloadAttachment)
				r.Get("/api/v1/dashboard/home", dashboardHomeHandler.GetHome)
				r.Get("/api/v1/dashboard/stock/skus", dashboardHomeHandler.GetStockSKUs)
				r.Get("/api/v1/dashboard/advertising/campaigns", dashboardHomeHandler.GetAdCampaigns)
				r.Get("/api/v1/analytics/dashboard", analyticsDashboardHandler.GetDashboardSummary)
				r.Get("/api/v1/analytics/sku-table", analyticsDashboardHandler.GetSKUTable)
				r.Get("/api/v1/analytics/stocks", analyticsDashboardHandler.GetStocksTable)
				r.Get("/api/v1/analytics/critical-skus", analyticsDashboardHandler.GetCriticalSKUs)
				r.Get("/api/v1/analytics/stocks-replenishment", analyticsDashboardHandler.GetStocksReplenishment)
				r.Get("/api/v1/analytics/advertising", analyticsDashboardHandler.GetAdvertisingAnalytics)
				r.Get("/api/v1/products", productsHandler.ListProducts)
				// Deprecated/future module — not part of active seller-facing MVP (UI redirects to Products).
				r.Get("/api/v1/pricing-constraints", pricingConstraintsHandler.GetPricingConstraints)
				r.Put("/api/v1/pricing-constraints/global", pricingConstraintsHandler.PutGlobalDefault)
				r.Post("/api/v1/pricing-constraints/category-rules", pricingConstraintsHandler.PostCategoryRule)
				r.Post("/api/v1/pricing-constraints/category-rules/deactivate", pricingConstraintsHandler.PostDeactivateCategoryRule)
				r.Post("/api/v1/pricing-constraints/sku-overrides", pricingConstraintsHandler.PostSKUOverride)
				r.Post("/api/v1/pricing-constraints/sku-overrides/deactivate", pricingConstraintsHandler.PostDeactivateSKUOverride)
				r.Get("/api/v1/pricing-constraints/effective", pricingConstraintsHandler.GetEffectiveConstraints)
				r.Post("/api/v1/pricing-constraints/preview", pricingConstraintsHandler.PostPreview)
				r.Get("/api/v1/actions", actionViewHandler.List)
				r.Post("/api/v1/actions/bulk", actionViewHandler.Bulk)
				r.Post("/api/v1/actions/cases/{case_id}/done", actionViewHandler.Done)
				r.Post("/api/v1/actions/cases/{case_id}/dismiss", actionViewHandler.Dismiss)
				r.Post("/api/v1/actions/cases/{case_id}/reopen", actionViewHandler.Reopen)
				r.Get("/api/v1/actions/id/{id}", actionsHandler.GetActionByID)
				r.Get("/api/v1/actions/{action_view_id}", actionViewHandler.Get)
				r.Post("/api/v1/actions/{action_key}/check", actionsHandler.CheckAction)
				r.Post("/api/v1/actions/{action_key}/uncheck", actionsHandler.UncheckAction)
				r.Post("/api/v1/actions/{action_key}/dismiss", actionsHandler.DismissAction)
				r.Get("/api/v1/actions/{action_key}/export", actionsHandler.ExportAction)
				r.Get("/api/v1/alerts", alertsHandler.GetAlerts)
				r.Get("/api/v1/alerts/summary", alertsHandler.GetSummary)
				r.Post("/api/v1/alerts/run", alertsHandler.RunAlerts)
				r.Post("/api/v1/alerts/{id}/dismiss", alertsHandler.DismissAlert)
				r.Post("/api/v1/alerts/{id}/resolve", alertsHandler.ResolveAlert)
				r.Get("/api/v1/recommendations", recommendationsHandler.ListRecommendations)
				r.Get("/api/v1/recommendations/summary", recommendationsHandler.GetSummary)
				r.Get("/api/v1/recommendations/{id}", recommendationsHandler.GetRecommendationByID)
				r.Post("/api/v1/recommendations/generate", recommendationsHandler.GenerateRecommendations)
				r.Post("/api/v1/recommendations/{id}/accept", recommendationsHandler.AcceptRecommendation)
				r.Post("/api/v1/recommendations/{id}/dismiss", recommendationsHandler.DismissRecommendation)
				r.Post("/api/v1/recommendations/{id}/resolve", recommendationsHandler.ResolveRecommendation)
				r.Post("/api/v1/recommendations/{id}/feedback", recommendationsHandler.AddFeedback)
				r.Get("/api/v1/ai-summaries/latest", aiSummariesHandler.GetLatest)
			})
		})
	})

	return &Server{
		httpServer: &http.Server{
			Addr:              fmt.Sprintf(":%s", port),
			Handler:           r,
			ReadHeaderTimeout: httpCfg.ReadHeaderTimeout,
			ReadTimeout:       httpCfg.ReadTimeout,
			WriteTimeout:      httpCfg.WriteTimeout,
			IdleTimeout:       httpCfg.IdleTimeout,
			MaxHeaderBytes:    httpCfg.MaxHeaderBytes,
		},
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}
