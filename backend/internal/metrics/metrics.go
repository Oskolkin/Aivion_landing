package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	HTTPRequestTotal                 *prometheus.CounterVec
	HTTPRequestDuration              *prometheus.HistogramVec
	AppInfo                          *prometheus.GaugeVec
	DBUp                             prometheus.Gauge
	RedisUp                          prometheus.Gauge
	S3Up                             prometheus.Gauge
	JobsProcessedTotal               *prometheus.CounterVec
	AIPipelineDispatchTotal          *prometheus.CounterVec
	AIPipelineDispatchRecoveryTotal  *prometheus.CounterVec
	AIPipelineDispatchRetryTotal     *prometheus.CounterVec
	AIPipelineDispatchStaleTotal     *prometheus.CounterVec
	AIPipelineExecutionClaimTotal    *prometheus.CounterVec
	AIPipelineExecutionLeaseTotal    *prometheus.CounterVec
	AIPipelineDuplicateDeliveryTotal prometheus.Counter
	AIPipelineResumeTotal            *prometheus.CounterVec
	ParentSyncGuardTotal             *prometheus.CounterVec
	LiveReviewRequestsTotal          *prometheus.CounterVec
	LiveReviewAuthFailuresTotal      *prometheus.CounterVec
	LiveReviewWriteBlockedTotal      prometheus.Counter
}

func New(registry *prometheus.Registry) *Metrics {
	m := &Metrics{
		HTTPRequestTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests.",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request latency in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),
		AppInfo: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "app_info",
				Help: "Application info.",
			},
			[]string{"service", "env", "version"},
		),
		DBUp: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "db_up",
				Help: "Database connectivity status.",
			},
		),
		RedisUp: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "redis_up",
				Help: "Redis connectivity status.",
			},
		),
		S3Up: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "s3_up",
				Help: "S3 connectivity status.",
			},
		),
		JobsProcessedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "jobs_processed_total",
				Help: "Total number of processed jobs.",
			},
			[]string{"task_type", "queue", "status"},
		),
		AIPipelineDispatchTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_pipeline_dispatch_total",
				Help: "AI pipeline dispatch lifecycle transitions.",
			},
			[]string{"status"},
		),
		AIPipelineDispatchRecoveryTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_pipeline_dispatch_recovery_total",
				Help: "AI pipeline dispatch recovery outcomes.",
			},
			[]string{"result"},
		),
		AIPipelineDispatchRetryTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_pipeline_dispatch_retry_total",
				Help: "AI pipeline dispatch retries by stage.",
			},
			[]string{"stage"},
		),
		AIPipelineDispatchStaleTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_pipeline_dispatch_stale_total",
				Help: "Stale dispatch states reconciled.",
			},
			[]string{"state"},
		),
		AIPipelineExecutionClaimTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_pipeline_execution_claim_total",
				Help: "Pipeline execution claim outcomes.",
			},
			[]string{"result"},
		),
		AIPipelineExecutionLeaseTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_pipeline_execution_lease_total",
				Help: "Pipeline execution lease operations.",
			},
			[]string{"result"},
		),
		AIPipelineDuplicateDeliveryTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "ai_pipeline_duplicate_delivery_total",
				Help: "Duplicate AI pipeline task deliveries treated as idempotent success.",
			},
		),
		AIPipelineResumeTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_pipeline_resume_total",
				Help: "Pipeline stage resumes without repeating provider calls.",
			},
			[]string{"stage"},
		),
		ParentSyncGuardTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "parent_sync_guard_total",
				Help: "Parent sync guard evaluations for downstream handlers.",
			},
			[]string{"handler", "result", "reason"},
		),
		LiveReviewRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "live_review_requests_total",
				Help: "Live review authenticated requests by mode/status.",
			},
			[]string{"mode", "status"},
		),
		LiveReviewAuthFailuresTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "live_review_auth_failures_total",
				Help: "Live review authentication failures by reason.",
			},
			[]string{"reason"},
		),
		LiveReviewWriteBlockedTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "live_review_write_blocked_total",
				Help: "Mutating requests blocked for live review sessions.",
			},
		),
	}

	registry.MustRegister(
		m.HTTPRequestTotal,
		m.HTTPRequestDuration,
		m.AppInfo,
		m.DBUp,
		m.RedisUp,
		m.S3Up,
		m.JobsProcessedTotal,
		m.AIPipelineDispatchTotal,
		m.AIPipelineDispatchRecoveryTotal,
		m.AIPipelineDispatchRetryTotal,
		m.AIPipelineDispatchStaleTotal,
		m.AIPipelineExecutionClaimTotal,
		m.AIPipelineExecutionLeaseTotal,
		m.AIPipelineDuplicateDeliveryTotal,
		m.AIPipelineResumeTotal,
		m.ParentSyncGuardTotal,
		m.LiveReviewRequestsTotal,
		m.LiveReviewAuthFailuresTotal,
		m.LiveReviewWriteBlockedTotal,
	)

	return m
}
