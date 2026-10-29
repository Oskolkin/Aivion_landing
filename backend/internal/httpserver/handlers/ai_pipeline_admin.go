package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/admin"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/jobs"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type AIPipelineAdminHandler struct {
	inspectStore aipipeline.PipelineInspectStore
	queries      *dbgen.Queries
	adminSvc     *admin.Service
}

func NewAIPipelineAdminHandler(store aipipeline.PipelineInspectStore, queries *dbgen.Queries, adminSvc *admin.Service) *AIPipelineAdminHandler {
	return &AIPipelineAdminHandler{inspectStore: store, queries: queries, adminSvc: adminSvc}
}

func (h *AIPipelineAdminHandler) GetPipelineDiagnostics(w http.ResponseWriter, r *http.Request) {
	if h.inspectStore == nil || h.queries == nil {
		writeJSONError(w, http.StatusInternalServerError, "pipeline diagnostics not configured")
		return
	}
	pipelineRunID, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "pipeline_run_id")), 10, 64)
	if err != nil || pipelineRunID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid pipeline_run_id")
		return
	}

	topActions := aipipeline.DashboardTopActionsDiag{Source: "unknown"}
	if run, err := h.inspectStore.GetPipelineRun(r.Context(), pipelineRunID); err == nil && run.ActionsRunID.Valid {
		topActions.ActionsRunID = run.ActionsRunID.Int64
		rows, listErr := h.queries.ListActionItemsWithUserStateByRunID(r.Context(), pgtype.Int8{Int64: run.ActionsRunID.Int64, Valid: true})
		if listErr == nil {
			var topKeysJSON []byte
			if genRun, genErr := h.queries.GetActionGenerationRunForAdmin(r.Context(), run.ActionsRunID.Int64); genErr == nil {
				topKeysJSON = genRun.TopActionKeysJson
			}
			ids := dashboardhome.DashboardTopActionIDsFromRows(rows, topKeysJSON, 3)
			if len(ids) > 0 {
				topActions.Source = "actions_final"
				topActions.ActionIDs = ids
			} else if len(rows) > 0 {
				topActions.Source = "actions_final"
			}
		}
	}

	diag, err := aipipeline.LoadAdminPipelineDiagnostics(r.Context(), h.inspectStore, pipelineRunID, topActions)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "pipeline_run_not_found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to load pipeline diagnostics")
		return
	}
	if err := aipipeline.AttachUsageDiagnostics(r.Context(), h.queries, &diag); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load pipeline usage diagnostics")
		return
	}
	writeJSON(w, http.StatusOK, diag)
}

func (h *AIPipelineAdminHandler) GetDispatchDiagnostics(w http.ResponseWriter, r *http.Request) {
	if h.queries == nil {
		writeJSONError(w, http.StatusInternalServerError, "dispatch diagnostics not configured")
		return
	}
	syncJobID, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "sync_job_id")), 10, 64)
	if err != nil || syncJobID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid sync_job_id")
		return
	}
	row, err := h.queries.GetAIPipelineDispatchBySyncAndStage(r.Context(), dbgen.GetAIPipelineDispatchBySyncAndStageParams{
		SyncJobID: syncJobID,
		Stage:     "ai_pipeline",
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "dispatch_not_found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to load dispatch")
		return
	}
	writeJSON(w, http.StatusOK, jobs.BuildAdminDispatchDiagnostics(row))
}

func (h *AIPipelineAdminHandler) RetryDispatch(w http.ResponseWriter, r *http.Request) {
	if h.queries == nil {
		writeJSONError(w, http.StatusInternalServerError, "dispatch retry not configured")
		return
	}
	dispatchID, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "dispatch_id")), 10, 64)
	if err != nil || dispatchID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid dispatch_id")
		return
	}
	dispatchSvc := jobs.NewAIPipelineDispatchService(h.queries, jobs.LoadDispatchConfig(), nil)
	row, err := dispatchSvc.AdminRetry(r.Context(), dispatchID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusConflict, "dispatch_not_failed")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "dispatch retry failed")
		return
	}
	if h.adminSvc != nil {
		actor, ok := adminActorFromRequest(r)
		if ok {
			_, _ = h.adminSvc.RetryAIPipelineDispatch(r.Context(), actor, admin.RetryAIPipelineDispatchInput{
				DispatchID:      dispatchID,
				SellerAccountID: row.SellerAccountID,
				SyncJobID:       row.SyncJobID,
			})
		}
	}
	writeJSON(w, http.StatusOK, jobs.BuildAdminDispatchDiagnostics(row))
}
