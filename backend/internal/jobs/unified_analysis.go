package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/prompt"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

const (
	TaskTypeUnifiedAnalysisStart    = "unified_analysis.start"
	TaskTypeUnifiedAnalysisPoll     = "unified_analysis.poll"
	TaskTypeUnifiedAnalysisCleanup  = "unified_analysis.cleanup"
	TaskTypeUnifiedAnalysisCancel   = "unified_analysis.cancel"
	TaskTypeUnifiedAnalysisValidate = "unified_analysis.validate_output"
)

// Asynq option defaults for Unified Analysis tasks (documented for Stage 3/4 acceptance).
const (
	UnifiedAnalysisStartTimeout       = 15 * time.Minute
	UnifiedAnalysisPollTimeout        = 2 * time.Minute
	UnifiedAnalysisCleanupTimeout     = 5 * time.Minute
	UnifiedAnalysisCancelTimeout      = 2 * time.Minute
	UnifiedAnalysisValidateTimeout    = 5 * time.Minute
	UnifiedAnalysisStartUniqueTTL     = 10 * time.Minute
	UnifiedAnalysisCleanupUniqueTTL   = 10 * time.Minute
	UnifiedAnalysisValidateUniqueTTL  = 10 * time.Minute
	UnifiedAnalysisMaxStartRetries    = 3
	UnifiedAnalysisMaxPollRetries     = 5
	UnifiedAnalysisMaxCleanupRetries  = 5
	UnifiedAnalysisMaxValidateRetries = 5
)

type StartUnifiedAnalysisPayload struct {
	RunID int64 `json:"run_id"`
}

type PollUnifiedAnalysisPayload struct {
	RunID       int64 `json:"run_id"`
	PollAttempt int   `json:"poll_attempt"`
}

type CleanupUnifiedAnalysisPayload struct {
	RunID int64 `json:"run_id"`
}

type CancelUnifiedAnalysisPayload struct {
	RunID int64 `json:"run_id"`
}

type ValidateUnifiedAnalysisOutputPayload struct {
	RunID int64 `json:"run_id"`
}

func NewUnifiedAnalysisStartTask(runID int64) (*asynq.Task, error) {
	payload, err := json.Marshal(StartUnifiedAnalysisPayload{RunID: runID})
	if err != nil {
		return nil, fmt.Errorf("marshal unified analysis start payload: %w", err)
	}
	return asynq.NewTask(TaskTypeUnifiedAnalysisStart, payload), nil
}

func NewUnifiedAnalysisPollTask(runID int64, pollAttempt int) (*asynq.Task, error) {
	payload, err := json.Marshal(PollUnifiedAnalysisPayload{RunID: runID, PollAttempt: pollAttempt})
	if err != nil {
		return nil, fmt.Errorf("marshal unified analysis poll payload: %w", err)
	}
	return asynq.NewTask(TaskTypeUnifiedAnalysisPoll, payload), nil
}

func NewUnifiedAnalysisCleanupTask(runID int64) (*asynq.Task, error) {
	payload, err := json.Marshal(CleanupUnifiedAnalysisPayload{RunID: runID})
	if err != nil {
		return nil, fmt.Errorf("marshal unified analysis cleanup payload: %w", err)
	}
	return asynq.NewTask(TaskTypeUnifiedAnalysisCleanup, payload), nil
}

func NewUnifiedAnalysisCancelTask(runID int64) (*asynq.Task, error) {
	payload, err := json.Marshal(CancelUnifiedAnalysisPayload{RunID: runID})
	if err != nil {
		return nil, fmt.Errorf("marshal unified analysis cancel payload: %w", err)
	}
	return asynq.NewTask(TaskTypeUnifiedAnalysisCancel, payload), nil
}

func NewUnifiedAnalysisValidateOutputTask(runID int64) (*asynq.Task, error) {
	payload, err := json.Marshal(ValidateUnifiedAnalysisOutputPayload{RunID: runID})
	if err != nil {
		return nil, fmt.Errorf("marshal unified analysis validate payload: %w", err)
	}
	return asynq.NewTask(TaskTypeUnifiedAnalysisValidate, payload), nil
}

// UnifiedAnalysisPollTaskID returns a unique task id per poll attempt so ProcessIn
// scheduling is not blocked by Asynq retention of the previous poll TaskID.
func UnifiedAnalysisPollTaskID(runID int64, pollAttempt int) string {
	if pollAttempt <= 0 {
		pollAttempt = 1
	}
	return fmt.Sprintf("unified-analysis-poll:%d:%d", runID, pollAttempt)
}

// UnifiedAnalysisEnqueue schedules follow-up unified analysis tasks.
type UnifiedAnalysisEnqueue interface {
	EnqueueStart(ctx context.Context, runID int64) error
	EnqueuePoll(ctx context.Context, runID int64, pollAttempt int, delay time.Duration) error
	EnqueueCleanup(ctx context.Context, runID int64) error
	EnqueueCancel(ctx context.Context, runID int64) error
	EnqueueValidateOutput(ctx context.Context, runID int64) error
	// EnqueuePublish schedules Stage 5 unified_analysis.publish (see
	// unified_analysis_publish.go).
	EnqueuePublish(ctx context.Context, runID int64) error
}

type AsynqUnifiedAnalysisEnqueue struct {
	Client *asynq.Client
}

func (e *AsynqUnifiedAnalysisEnqueue) EnqueueStart(_ context.Context, runID int64) error {
	if e == nil || e.Client == nil {
		return fmt.Errorf("asynq client is not configured")
	}
	task, err := NewUnifiedAnalysisStartTask(runID)
	if err != nil {
		return err
	}
	_, err = e.Client.Enqueue(task,
		asynq.TaskID(fmt.Sprintf("unified-analysis-start:%d", runID)),
		asynq.Unique(UnifiedAnalysisStartUniqueTTL),
		asynq.Timeout(UnifiedAnalysisStartTimeout),
		asynq.MaxRetry(UnifiedAnalysisMaxStartRetries),
	)
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) && !errors.Is(err, asynq.ErrTaskIDConflict) {
		return err
	}
	return nil
}

func (e *AsynqUnifiedAnalysisEnqueue) EnqueuePoll(_ context.Context, runID int64, pollAttempt int, delay time.Duration) error {
	if e == nil || e.Client == nil {
		return fmt.Errorf("asynq client is not configured")
	}
	if pollAttempt <= 0 {
		pollAttempt = 1
	}
	task, err := NewUnifiedAnalysisPollTask(runID, pollAttempt)
	if err != nil {
		return err
	}
	opts := []asynq.Option{
		asynq.TaskID(UnifiedAnalysisPollTaskID(runID, pollAttempt)),
		asynq.Timeout(UnifiedAnalysisPollTimeout),
		asynq.MaxRetry(UnifiedAnalysisMaxPollRetries),
	}
	if delay > 0 {
		opts = append(opts, asynq.ProcessIn(delay))
	}
	_, err = e.Client.Enqueue(task, opts...)
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) && !errors.Is(err, asynq.ErrTaskIDConflict) {
		return err
	}
	return nil
}

func (e *AsynqUnifiedAnalysisEnqueue) EnqueueCleanup(_ context.Context, runID int64) error {
	if e == nil || e.Client == nil {
		return fmt.Errorf("asynq client is not configured")
	}
	task, err := NewUnifiedAnalysisCleanupTask(runID)
	if err != nil {
		return err
	}
	_, err = e.Client.Enqueue(task,
		asynq.TaskID(fmt.Sprintf("unified-analysis-cleanup:%d", runID)),
		asynq.Unique(UnifiedAnalysisCleanupUniqueTTL),
		asynq.Timeout(UnifiedAnalysisCleanupTimeout),
		asynq.MaxRetry(UnifiedAnalysisMaxCleanupRetries),
	)
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) && !errors.Is(err, asynq.ErrTaskIDConflict) {
		return err
	}
	return nil
}

func (e *AsynqUnifiedAnalysisEnqueue) EnqueueCancel(_ context.Context, runID int64) error {
	if e == nil || e.Client == nil {
		return fmt.Errorf("asynq client is not configured")
	}
	task, err := NewUnifiedAnalysisCancelTask(runID)
	if err != nil {
		return err
	}
	_, err = e.Client.Enqueue(task,
		asynq.TaskID(fmt.Sprintf("unified-analysis-cancel:%d", runID)),
		asynq.Unique(5*time.Minute),
		asynq.Timeout(UnifiedAnalysisCancelTimeout),
		asynq.MaxRetry(3),
	)
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) && !errors.Is(err, asynq.ErrTaskIDConflict) {
		return err
	}
	return nil
}

func (e *AsynqUnifiedAnalysisEnqueue) EnqueueValidateOutput(_ context.Context, runID int64) error {
	if e == nil || e.Client == nil {
		return fmt.Errorf("asynq client is not configured")
	}
	task, err := NewUnifiedAnalysisValidateOutputTask(runID)
	if err != nil {
		return err
	}
	_, err = e.Client.Enqueue(task,
		asynq.TaskID(fmt.Sprintf("unified-analysis-validate:%d", runID)),
		asynq.Unique(UnifiedAnalysisValidateUniqueTTL),
		asynq.Timeout(UnifiedAnalysisValidateTimeout),
		asynq.MaxRetry(UnifiedAnalysisMaxValidateRetries),
	)
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) && !errors.Is(err, asynq.ErrTaskIDConflict) {
		return err
	}
	return nil
}

// UnifiedAnalysisHandler processes Stage 3/4 provider orchestration tasks.
type UnifiedAnalysisHandler struct {
	svc     *unifiedanalysis.Service
	enqueue UnifiedAnalysisEnqueue
	log     *zap.Logger

	// autoPublishQueries/autoPublishGlobal are optional (nil-safe): when set via
	// EnableAutoPublish, HandleValidateOutput enqueues Stage 5 publish for runs that
	// reach publication_ready and whose seller has EffectiveAutoPublish enabled.
	autoPublishQueries *dbgen.Queries
	autoPublishGlobal  publication.GlobalFlags
}

func NewUnifiedAnalysisHandler(svc *unifiedanalysis.Service, enqueue UnifiedAnalysisEnqueue, log *zap.Logger) *UnifiedAnalysisHandler {
	return &UnifiedAnalysisHandler{svc: svc, enqueue: enqueue, log: log}
}

// EnableAutoPublish wires seller_analysis_settings + global rollout flags so
// HandleValidateOutput can auto-enqueue Stage 5 publish. Not calling this (queries
// stays nil) is safe and simply disables the auto-publish check.
func (h *UnifiedAnalysisHandler) EnableAutoPublish(queries *dbgen.Queries, global publication.GlobalFlags) {
	if h == nil {
		return
	}
	h.autoPublishQueries = queries
	h.autoPublishGlobal = global
}

// RegisterUnifiedAnalysisHandlers registers Stage 3/4 handlers on mux.
func RegisterUnifiedAnalysisHandlers(mux *asynq.ServeMux, handler *UnifiedAnalysisHandler) {
	if mux == nil || handler == nil {
		return
	}
	mux.HandleFunc(TaskTypeUnifiedAnalysisStart, func(ctx context.Context, t *asynq.Task) error {
		return handler.HandleStart(ctx, t.Payload())
	})
	mux.HandleFunc(TaskTypeUnifiedAnalysisPoll, func(ctx context.Context, t *asynq.Task) error {
		return handler.HandlePoll(ctx, t.Payload())
	})
	mux.HandleFunc(TaskTypeUnifiedAnalysisCleanup, func(ctx context.Context, t *asynq.Task) error {
		return handler.HandleCleanup(ctx, t.Payload())
	})
	mux.HandleFunc(TaskTypeUnifiedAnalysisCancel, func(ctx context.Context, t *asynq.Task) error {
		return handler.HandleCancel(ctx, t.Payload())
	})
	mux.HandleFunc(TaskTypeUnifiedAnalysisValidate, func(ctx context.Context, t *asynq.Task) error {
		return handler.HandleValidateOutput(ctx, t.Payload())
	})
}

func (h *UnifiedAnalysisHandler) HandleStart(ctx context.Context, taskPayload []byte) error {
	started := time.Now()
	var payload StartUnifiedAnalysisPayload
	if err := json.Unmarshal(taskPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal unified analysis start: %w", err)
	}
	if payload.RunID <= 0 {
		return fmt.Errorf("invalid run_id")
	}
	h.logEvent("unified_analysis.start", payload.RunID, 0, "", "", started)
	if err := h.svc.StartRun(ctx, payload.RunID); err != nil {
		h.logError("unified_analysis.start", payload.RunID, err)
		return err
	}
	run, err := h.svc.GetRun(ctx, payload.RunID)
	if err != nil {
		return err
	}
	if run != nil && run.ProviderResponseID != "" && run.Status == unifiedanalysis.StatusProviderRunning {
		if h.enqueue != nil {
			if err := h.enqueue.EnqueuePoll(ctx, payload.RunID, 1, 0); err != nil {
				return fmt.Errorf("enqueue poll after start: %w", err)
			}
		}
	}
	h.logEvent("unified_analysis.start_done", payload.RunID, 0, runStatus(run), runProviderStatus(run), started)
	return nil
}

func (h *UnifiedAnalysisHandler) HandlePoll(ctx context.Context, taskPayload []byte) error {
	started := time.Now()
	var payload PollUnifiedAnalysisPayload
	if err := json.Unmarshal(taskPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal unified analysis poll: %w", err)
	}
	if payload.RunID <= 0 {
		return fmt.Errorf("invalid run_id")
	}
	if payload.PollAttempt <= 0 {
		payload.PollAttempt = 1
	}
	h.logEvent("unified_analysis.poll", payload.RunID, payload.PollAttempt, "", "", started)

	result, err := h.svc.PollRun(ctx, payload.RunID)
	if result != nil && (result.Completed || result.Failed || result.Cancelled) {
		if h.enqueue != nil {
			// Validate before cleanup so analysis runs always reach Stage 4 even if
			// cleanup tasks race ahead (container teardown is independent of
			// artifact-based output validation).
			if result.Run != nil && result.Completed && isAnalysisRunPrompt(result.Run.PromptVersion) {
				if enqErr := h.enqueue.EnqueueValidateOutput(ctx, payload.RunID); enqErr != nil {
					return fmt.Errorf("enqueue validate after poll: %w", enqErr)
				}
			}
			if enqErr := h.enqueue.EnqueueCleanup(ctx, payload.RunID); enqErr != nil {
				return fmt.Errorf("enqueue cleanup after poll: %w", enqErr)
			}
		}
		h.logEvent("unified_analysis.poll_terminal", payload.RunID, payload.PollAttempt, runStatus(result.Run), "", started)
		// Terminal provider state is persisted; do not retry the poll task.
		return nil
	}
	if err != nil {
		h.logError("unified_analysis.poll", payload.RunID, err)
		return err
	}
	if result == nil {
		return nil
	}
	if h.enqueue != nil {
		delay := result.NextPollAfter
		if delay <= 0 {
			delay = 5 * time.Second
		}
		nextAttempt := payload.PollAttempt + 1
		if err := h.enqueue.EnqueuePoll(ctx, payload.RunID, nextAttempt, delay); err != nil {
			return fmt.Errorf("enqueue next poll: %w", err)
		}
		if h.log != nil {
			h.log.Info("unified_analysis.poll_scheduled",
				zap.Int64("run_id", payload.RunID),
				zap.Int("poll_attempt", payload.PollAttempt),
				zap.Int("next_poll_attempt", nextAttempt),
				zap.Duration("delay", delay),
				zap.Duration("duration", time.Since(started)),
			)
		}
	}
	return nil
}

func (h *UnifiedAnalysisHandler) HandleCleanup(ctx context.Context, taskPayload []byte) error {
	started := time.Now()
	var payload CleanupUnifiedAnalysisPayload
	if err := json.Unmarshal(taskPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal unified analysis cleanup: %w", err)
	}
	if payload.RunID <= 0 {
		return fmt.Errorf("invalid run_id")
	}
	h.logEvent("unified_analysis.cleanup", payload.RunID, 0, "", "", started)
	if err := h.svc.CleanupRun(ctx, payload.RunID); err != nil {
		h.logError("unified_analysis.cleanup", payload.RunID, err)
		return err
	}
	h.logEvent("unified_analysis.cleanup_done", payload.RunID, 0, "", "", started)
	return nil
}

func (h *UnifiedAnalysisHandler) HandleCancel(ctx context.Context, taskPayload []byte) error {
	started := time.Now()
	var payload CancelUnifiedAnalysisPayload
	if err := json.Unmarshal(taskPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal unified analysis cancel: %w", err)
	}
	if payload.RunID <= 0 {
		return fmt.Errorf("invalid run_id")
	}
	h.logEvent("unified_analysis.cancel", payload.RunID, 0, "", "", started)
	if err := h.svc.CancelRun(ctx, payload.RunID); err != nil {
		h.logError("unified_analysis.cancel", payload.RunID, err)
		return err
	}
	if h.enqueue != nil {
		_ = h.enqueue.EnqueueCleanup(ctx, payload.RunID)
	}
	h.logEvent("unified_analysis.cancel_done", payload.RunID, 0, "", "", started)
	return nil
}

func (h *UnifiedAnalysisHandler) HandleValidateOutput(ctx context.Context, taskPayload []byte) error {
	started := time.Now()
	var payload ValidateUnifiedAnalysisOutputPayload
	if err := json.Unmarshal(taskPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal unified analysis validate: %w", err)
	}
	if payload.RunID <= 0 {
		return fmt.Errorf("invalid run_id")
	}
	h.logEvent("unified_analysis.validate_output", payload.RunID, 0, "", "", started)
	result, err := h.svc.ValidateOutput(ctx, payload.RunID)
	if err != nil {
		h.logError("unified_analysis.validate_output", payload.RunID, err)
		return err
	}
	status := ""
	if result != nil && result.Run != nil {
		status = result.Run.Status
		if status == unifiedanalysis.StatusPublicationReady {
			if enqErr := h.maybeEnqueueAutoPublish(ctx, result.Run); enqErr != nil {
				h.logError("unified_analysis.auto_publish_enqueue", payload.RunID, enqErr)
			}
		}
	}
	h.logEvent("unified_analysis.validate_output_done", payload.RunID, 0, status, "", started)
	return nil
}

// maybeEnqueueAutoPublish enqueues Stage 5 publish for run when the seller's resolved
// EffectiveAutoPublish is true. No-op (nil error) when auto-publish was never enabled
// via EnableAutoPublish, when the seller has no settings row, or when it resolves to
// false (e.g. legacy mode, or rollout/auto-publish disabled globally or per-seller).
func (h *UnifiedAnalysisHandler) maybeEnqueueAutoPublish(ctx context.Context, run *unifiedanalysis.Run) error {
	if h == nil || h.autoPublishQueries == nil || h.enqueue == nil || run == nil {
		return nil
	}
	var settingsPtr *dbgen.SellerAnalysisSetting
	row, err := h.autoPublishQueries.GetSellerAnalysisSettings(ctx, run.SellerAccountID)
	if err == nil {
		settingsPtr = &row
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load seller analysis settings for auto-publish: %w", err)
	}
	settings := publication.EffectiveSettings(settingsPtr, h.autoPublishGlobal)
	if !settings.EffectiveAutoPublish {
		return nil
	}
	return h.enqueue.EnqueuePublish(ctx, run.ID)
}

func (h *UnifiedAnalysisHandler) logEvent(event string, runID int64, pollAttempt int, status, providerStatus string, started time.Time) {
	if h.log == nil {
		return
	}
	fields := []zap.Field{
		zap.String("event", event),
		zap.Int64("run_id", runID),
		zap.String("task_type", event),
		zap.Duration("duration_ms", time.Since(started)),
	}
	if pollAttempt > 0 {
		fields = append(fields, zap.Int("poll_attempt", pollAttempt))
	}
	if status != "" {
		fields = append(fields, zap.String("run_status", status))
	}
	if providerStatus != "" {
		fields = append(fields, zap.String("provider_status", providerStatus))
	}
	h.log.Info(event, fields...)
}

func (h *UnifiedAnalysisHandler) logError(event string, runID int64, err error) {
	if h.log == nil || err == nil {
		return
	}
	h.log.Warn(event+"_error", zap.Int64("run_id", runID), zap.Error(err))
}

func runStatus(run *unifiedanalysis.Run) string {
	if run == nil {
		return ""
	}
	return run.Status
}

func runProviderStatus(run *unifiedanalysis.Run) string {
	if run == nil {
		return ""
	}
	return run.ProviderStatus
}

func isAnalysisRunPrompt(version string) bool {
	return prompt.ProfileForVersion(version) == prompt.PromptProfileAnalysis
}
