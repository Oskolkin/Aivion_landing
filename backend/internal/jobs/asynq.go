package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const (
	TypeSystemPing          = "system.ping"
	TaskTypeOzonInitialSync = "ozon.initial_sync"

	TaskTypeOzonSyncCoordinator = "ozon.sync_coordinator"
	TaskTypeOzonImportProducts  = "ozon.import.products"
	TaskTypeOzonImportOrders    = "ozon.import.orders"
	TaskTypeOzonImportStocks    = "ozon.import.stocks"
	TaskTypeOzonImportAds       = "ozon.import.ads"
	// TaskTypePostSyncRecalculation runs metrics rebuild + alerts after a sync_job reaches completed.
	TaskTypePostSyncRecalculation      = "ozon.post_sync_recalculation"
	TaskTypeOzonEnrichmentP0           = "ozon.enrichment_p0"
	TaskTypeRunSellerAIPipeline        = "ai.run_seller_pipeline"
	TaskTypeAIPipelineDispatchRecovery = "ai.pipeline_dispatch_recovery"
)

type RedisConfig struct {
	Addr        string
	Password    string
	DB          int
	Concurrency int
}

type SystemPingPayload struct {
	Message string `json:"message"`
}

type OzonInitialSyncPayload struct {
	SellerAccountID int64 `json:"seller_account_id"`
	SyncJobID       int64 `json:"sync_job_id"`
}

type OzonSyncCoordinatorPayload struct {
	SellerAccountID int64  `json:"seller_account_id"`
	SyncJobID       int64  `json:"sync_job_id"`
	SyncType        string `json:"sync_type"`
}

type OzonImportJobPayload struct {
	SellerAccountID int64  `json:"seller_account_id"`
	SyncJobID       int64  `json:"sync_job_id"`
	ImportJobID     int64  `json:"import_job_id"`
	Domain          string `json:"domain"`
}

// RecalculateAfterSyncPayload is processed after ingestion completes a sync_job (status completed).
type RecalculateAfterSyncPayload struct {
	SellerAccountID int64 `json:"seller_account_id"`
	SyncJobID       int64 `json:"sync_job_id"`
}

func NewAsynqClient(cfg RedisConfig) *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}

func resolveConcurrency(cfg RedisConfig) int {
	if cfg.Concurrency > 0 {
		return cfg.Concurrency
	}
	return 10
}

func NewAsynqServer(cfg RedisConfig) *asynq.Server {
	return asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		},
		asynq.Config{
			Concurrency: resolveConcurrency(cfg),
			Queues: map[string]int{
				"default": 1,
			},
		},
	)
}

func NewSystemPingTask(message string) (*asynq.Task, error) {
	payload, err := json.Marshal(SystemPingPayload{
		Message: message,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal system ping payload: %w", err)
	}

	return asynq.NewTask(TypeSystemPing, payload), nil
}

func NewOzonInitialSyncTask(sellerAccountID, syncJobID int64) (*asynq.Task, error) {
	payload, err := json.Marshal(OzonInitialSyncPayload{
		SellerAccountID: sellerAccountID,
		SyncJobID:       syncJobID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal ozon initial sync payload: %w", err)
	}

	return asynq.NewTask(TaskTypeOzonInitialSync, payload), nil
}

func NewOzonSyncCoordinatorTask(sellerAccountID, syncJobID int64, syncType string) (*asynq.Task, error) {
	payload, err := json.Marshal(OzonSyncCoordinatorPayload{
		SellerAccountID: sellerAccountID,
		SyncJobID:       syncJobID,
		SyncType:        syncType,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal ozon sync coordinator payload: %w", err)
	}

	return asynq.NewTask(TaskTypeOzonSyncCoordinator, payload), nil
}

func NewOzonImportProductsTask(sellerAccountID, syncJobID, importJobID int64) (*asynq.Task, error) {
	return newOzonImportTask(TaskTypeOzonImportProducts, sellerAccountID, syncJobID, importJobID, "products")
}

func NewOzonImportOrdersTask(sellerAccountID, syncJobID, importJobID int64) (*asynq.Task, error) {
	return newOzonImportTask(TaskTypeOzonImportOrders, sellerAccountID, syncJobID, importJobID, "orders")
}

func NewOzonImportStocksTask(sellerAccountID, syncJobID, importJobID int64) (*asynq.Task, error) {
	return newOzonImportTask(TaskTypeOzonImportStocks, sellerAccountID, syncJobID, importJobID, "stocks")
}

func NewOzonImportAdsTask(sellerAccountID, syncJobID, importJobID int64) (*asynq.Task, error) {
	return newOzonImportTask(TaskTypeOzonImportAds, sellerAccountID, syncJobID, importJobID, "ads")
}

func newOzonImportTask(taskType string, sellerAccountID, syncJobID, importJobID int64, domain string) (*asynq.Task, error) {
	payload, err := json.Marshal(OzonImportJobPayload{
		SellerAccountID: sellerAccountID,
		SyncJobID:       syncJobID,
		ImportJobID:     importJobID,
		Domain:          domain,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal ozon import payload for %s: %w", domain, err)
	}

	return asynq.NewTask(taskType, payload), nil
}

// NewRecalculateAfterSyncTask enqueues downstream metrics + alerts for a completed sync_job.
func NewRecalculateAfterSyncTask(sellerAccountID, syncJobID int64) (*asynq.Task, error) {
	payload, err := json.Marshal(RecalculateAfterSyncPayload{
		SellerAccountID: sellerAccountID,
		SyncJobID:       syncJobID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal post-sync recalculation payload: %w", err)
	}
	return asynq.NewTask(TaskTypePostSyncRecalculation, payload), nil
}

type OzonEnrichmentP0Payload struct {
	SellerAccountID int64  `json:"seller_account_id"`
	SyncJobID       int64  `json:"sync_job_id"`
	MetricDate      string `json:"metric_date"`
	Scope           string `json:"scope"`
}

func NewOzonEnrichmentP0Task(sellerAccountID, syncJobID int64, metricDate, scope string) (*asynq.Task, error) {
	payload, err := json.Marshal(OzonEnrichmentP0Payload{
		SellerAccountID: sellerAccountID,
		SyncJobID:       syncJobID,
		MetricDate:      metricDate,
		Scope:           scope,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal ozon enrichment payload: %w", err)
	}
	return asynq.NewTask(TaskTypeOzonEnrichmentP0, payload), nil
}

type RunSellerAIPipelinePayload struct {
	SellerAccountID int64  `json:"seller_account_id"`
	SyncJobID       int64  `json:"sync_job_id"`
	MetricDate      string `json:"metric_date"`
	Trigger         string `json:"trigger"`
	Force           bool   `json:"force"`
}

func NewRunSellerAIPipelineTask(sellerAccountID, syncJobID int64, metricDate, trigger string, force bool) (*asynq.Task, error) {
	payload, err := json.Marshal(RunSellerAIPipelinePayload{
		SellerAccountID: sellerAccountID,
		SyncJobID:       syncJobID,
		MetricDate:      metricDate,
		Trigger:         trigger,
		Force:           force,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal seller ai pipeline payload: %w", err)
	}
	return asynq.NewTask(TaskTypeRunSellerAIPipeline, payload), nil
}

func NewAIPipelineDispatchRecoveryTask() (*asynq.Task, error) {
	return asynq.NewTask(TaskTypeAIPipelineDispatchRecovery, []byte("{}")), nil
}
