// Package postgres is the sole SQL adapter for analysis_* and factual_universe_* tables.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrLeaseLost                 = errors.New("LEASE_LOST")
	ErrActiveProductionRunExists = errors.New("ACTIVE_PRODUCTION_RUN_EXISTS")
	ErrStageNotClaimable         = errors.New("STAGE_NOT_CLAIMABLE")
	ErrStageAlreadySucceeded     = errors.New("STAGE_ALREADY_SUCCEEDED")
	ErrStageInputHashInvalid     = errors.New("STAGE_INPUT_HASH_INVALID")
)

const zeroInputHash = "0000000000000000000000000000000000000000000000000000000000000000"

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository  { return &Repository{pool: pool} }
func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

type Run struct {
	ID, SellerID            int64
	Kind, ExecutionClass    string
	Status, IntentHash      string
	AnalysisDate, CutoffAt  time.Time
	ContractHash            string
	UniverseID              string
	BasePublicationID       *int64
	CurrentStage            string
	CancelRequestedAt       *time.Time
	CancelReasonCode        string
	ErrorCode, ErrorMessage string
}

type Lease struct {
	RunID, StageID, AttemptID, SellerID int64
	Stage, Token, WorkerID, InputHash   string
	Version                             int64
	ExpiresAt                           time.Time
	AttemptCount                        int
}

type Stage struct {
	ID, RunID                             int64
	Key, Version, Status, InputHash       string
	OutputHash                            string
	AttemptCount                          int
	LeaseOwner, LeaseToken                string
	LeaseVersion                          int64
	LeaseExpiresAt, HeartbeatAt           *time.Time
	NextRetryAt, StartedAt, CompletedAt   *time.Time
	AcceptedAttemptID, AcceptedArtifactID *int64
	ErrorCode, ErrorMessage               string
	Ordinal                               int
}

type UniverseRecord struct {
	ID, ManifestHash, ContextHash, ObjectKey string
	SellerID                                 int64
	AnalysisDate, CutoffAt, CreatedAt        time.Time
	ManifestJSON                             []byte
	ManifestArtifactID                       *int64
	ManifestVersion, TemporalVersion         string
	CapabilityVersion, DQVersion, APIVersion string
	UniversePolicyVersion                    string
	Sources                                  []UniverseSource
	Capabilities                             []UniverseCapability
	DQ                                       []UniverseDQSummary
	EntitySets                               []UniverseEntitySet
}

type UniverseSource struct {
	Domain, FactFamily, SourceBindingKey, SourceState    string
	LastAcceptedDomainRunID                              *int64
	CoverageFrom, CoverageTo                             *time.Time
	SnapshotAsOf, LatestObservedAt                       *time.Time
	LatestCompleteBusinessDate, LatestMatureBusinessDate *time.Time
	LatestCompleteSnapshotAt                             *time.Time
	SourceContractKey, SourceContractVersion             string
	SourceContractHash, SemanticMappingHash              string
	SourceEndpointFamily, SemanticMappingVersion         string
	SourceRevisionHash, WatermarkHash                    string
	SourceFinalizedAt                                    *time.Time
	SourceHash, BindingHash                              string
}

type UniverseCapability struct {
	CapabilityKey, Scope, FinalState                            string
	ContractState, AccessState, SourceState, SemanticState      string
	CoverageState, TemporalState, HistoryState, PopulationState string
	ReasonCode, PolicyVersion, AssessmentHash                   string
	AssessmentID                                                *int64
	AssessedAt                                                  time.Time
}

type UniverseDQSummary struct {
	Domain, FactFamily, DQPolicyVersion, DQResultHash string
	HighestSeverity                                   string
	BlockingIssueCount, LimitationIssueCount          int64
	WarningCount                                      int64
	CanonicalIssueSetHash                             string
	SummaryJSON                                       []byte
	DQRunID                                           *int64
}

type UniverseEntitySet struct {
	EntityKind  string
	SetHash     string
	MemberCount int64
}

type UniverseSealInput struct {
	Universe     UniverseRecord
	Sources      []UniverseSource
	Capabilities []UniverseCapability
	DQ           []UniverseDQSummary
	EntitySets   []UniverseEntitySet
}

type RecoveryCandidate struct {
	RunID, StageID, SellerID int64
	StageKey, Status, Reason string
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func ValidateStageTransition(from, to string) error {
	allowed := map[string]map[string]bool{
		"PENDING":          {"READY": true, "CANCELLED": true, "SKIPPED": true},
		"READY":            {"RUNNING": true, "CANCELLED": true},
		"RUNNING":          {"SUCCEEDED": true, "SUCCEEDED_WITH_LIMITATIONS": true, "FAILED_RETRYABLE": true, "FAILED_TERMINAL": true, "CANCELLED": true},
		"FAILED_RETRYABLE": {"READY": true, "CANCELLED": true},
	}
	if allowed[from][to] {
		return nil
	}
	return fmt.Errorf("invalid stage transition %s -> %s", from, to)
}

func (r *Repository) CreateProductionRun(ctx context.Context, in Run) (Run, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Run{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, in.SellerID); err != nil {
		return Run{}, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO analysis_runs(
			seller_account_id,run_kind,execution_class,status,analysis_date,analysis_cutoff_at,
			contract_bundle_hash,run_intent_hash,trigger,base_publication_id)
		VALUES($1,$2,'PRODUCTION','PREPARING',$3,$4,$5,$6,'MANUAL',
			(SELECT publication_id FROM seller_current_publications WHERE seller_account_id=$1))
		RETURNING id,seller_account_id,run_kind,execution_class,status,analysis_date,analysis_cutoff_at,run_intent_hash,contract_bundle_hash`,
		in.SellerID, nz(in.Kind, "STANDARD_DAILY"), in.AnalysisDate, in.CutoffAt, in.ContractHash, in.IntentHash).
		Scan(&in.ID, &in.SellerID, &in.Kind, &in.ExecutionClass, &in.Status, &in.AnalysisDate, &in.CutoffAt, &in.IntentHash, &in.ContractHash)
	if err != nil {
		if isUniqueViolation(err) {
			return Run{}, ErrActiveProductionRunExists
		}
		return Run{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO analysis_run_events(run_id,event_type,payload_json)VALUES($1,'RUN_CREATED','{}')`, in.ID); err != nil {
		return Run{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Run{}, err
	}
	return r.GetRun(ctx, in.ID)
}

func (r *Repository) CreateEvaluationRun(ctx context.Context, in Run) (Run, error) {
	err := r.pool.QueryRow(ctx, `INSERT INTO analysis_runs(
			seller_account_id,run_kind,execution_class,status,analysis_date,analysis_cutoff_at,
			contract_bundle_hash,run_intent_hash,trigger)
		VALUES($1,$2,'EVALUATION','PREPARING',$3,$4,$5,$6,'EVALUATION')
		RETURNING id,seller_account_id,run_kind,execution_class,status,analysis_date,analysis_cutoff_at,run_intent_hash,contract_bundle_hash`,
		in.SellerID, nz(in.Kind, "EVALUATION"), in.AnalysisDate, in.CutoffAt, in.ContractHash, in.IntentHash).
		Scan(&in.ID, &in.SellerID, &in.Kind, &in.ExecutionClass, &in.Status, &in.AnalysisDate, &in.CutoffAt, &in.IntentHash, &in.ContractHash)
	return in, err
}

func (r *Repository) EnsureStages(ctx context.Context, runID int64, names []string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		for i, name := range names {
			status := "PENDING"
			if i == 0 {
				status = "READY"
			}
			if _, err := tx.Exec(ctx, `INSERT INTO analysis_run_stages(run_id,stage_key,stage_version,ordinal,status)
				VALUES($1,$2,'v1',$3,$4)ON CONFLICT(run_id,stage_key)DO NOTHING`, runID, name, i+1, status); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) GetRun(ctx context.Context, id int64) (Run, error) {
	var out Run
	var universe string
	var basePub int64
	err := r.pool.QueryRow(ctx, `SELECT id,seller_account_id,run_kind,execution_class,status,analysis_date,analysis_cutoff_at,
		contract_bundle_hash,run_intent_hash,COALESCE(factual_universe_id::text,''),COALESCE(base_publication_id,0),COALESCE(current_stage,''),
		cancel_requested_at,COALESCE(cancel_reason_code,''),COALESCE(error_code,''),COALESCE(error_message,'')
		FROM analysis_runs WHERE id=$1`, id).
		Scan(&out.ID, &out.SellerID, &out.Kind, &out.ExecutionClass, &out.Status, &out.AnalysisDate, &out.CutoffAt,
			&out.ContractHash, &out.IntentHash, &universe, &basePub, &out.CurrentStage, &out.CancelRequestedAt, &out.CancelReasonCode, &out.ErrorCode, &out.ErrorMessage)
	out.UniverseID = universe
	if basePub > 0 {
		out.BasePublicationID = &basePub
	}
	return out, err
}

func (r *Repository) GetStage(ctx context.Context, runID int64, stageKey string) (Stage, error) {
	var s Stage
	err := r.pool.QueryRow(ctx, `SELECT id,run_id,stage_key,stage_version,ordinal,status,COALESCE(input_hash,''),COALESCE(output_hash,''),
		attempt_count,COALESCE(lease_owner,''),COALESCE(lease_token::text,''),lease_version,lease_expires_at,heartbeat_at,
		next_retry_at,started_at,completed_at,accepted_attempt_id,accepted_artifact_id,COALESCE(error_code,''),COALESCE(error_message,'')
		FROM analysis_run_stages WHERE run_id=$1 AND stage_key=$2`, runID, stageKey).
		Scan(&s.ID, &s.RunID, &s.Key, &s.Version, &s.Ordinal, &s.Status, &s.InputHash, &s.OutputHash, &s.AttemptCount, &s.LeaseOwner, &s.LeaseToken,
			&s.LeaseVersion, &s.LeaseExpiresAt, &s.HeartbeatAt, &s.NextRetryAt, &s.StartedAt, &s.CompletedAt, &s.AcceptedAttemptID, &s.AcceptedArtifactID, &s.ErrorCode, &s.ErrorMessage)
	return s, err
}

func (r *Repository) ListStages(ctx context.Context, runID int64) ([]Stage, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,run_id,stage_key,stage_version,ordinal,status,COALESCE(input_hash,''),COALESCE(output_hash,''),
		attempt_count,COALESCE(lease_owner,''),COALESCE(lease_token::text,''),lease_version,lease_expires_at,heartbeat_at,
		next_retry_at,started_at,completed_at,accepted_attempt_id,accepted_artifact_id,COALESCE(error_code,''),COALESCE(error_message,'')
		FROM analysis_run_stages WHERE run_id=$1 ORDER BY ordinal`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Stage
	for rows.Next() {
		var s Stage
		if err = rows.Scan(&s.ID, &s.RunID, &s.Key, &s.Version, &s.Ordinal, &s.Status, &s.InputHash, &s.OutputHash, &s.AttemptCount, &s.LeaseOwner, &s.LeaseToken,
			&s.LeaseVersion, &s.LeaseExpiresAt, &s.HeartbeatAt, &s.NextRetryAt, &s.StartedAt, &s.CompletedAt, &s.AcceptedAttemptID, &s.AcceptedArtifactID, &s.ErrorCode, &s.ErrorMessage); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) SetStageInputHash(ctx context.Context, stageID int64, hash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE analysis_run_stages SET input_hash=$2 WHERE id=$1 AND (input_hash IS NULL OR input_hash=$2)`, stageID, hash)
	return err
}

func (r *Repository) PromoteRetryableReady(ctx context.Context, now time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE analysis_run_stages SET status='READY',next_retry_at=NULL
		WHERE status='FAILED_RETRYABLE' AND next_retry_at IS NOT NULL AND next_retry_at<=$1`, now.UTC())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *Repository) Claim(ctx context.Context, worker string, leaseFor time.Duration, now time.Time) (Lease, error) {
	var stageID int64
	err := r.pool.QueryRow(ctx, `SELECT s.id FROM analysis_run_stages s JOIN analysis_runs r ON r.id=s.run_id
		WHERE r.status IN('PREPARING','RUNNING')
		  AND(s.status='READY' OR(s.status='RUNNING' AND s.lease_expires_at IS NOT NULL AND s.lease_expires_at<$1))
		ORDER BY r.requested_at,s.ordinal LIMIT 1`, now.UTC()).Scan(&stageID)
	if err != nil {
		return Lease{}, err
	}
	return r.claimStageID(ctx, stageID, worker, leaseFor, now)
}

func (r *Repository) ClaimByRunStage(ctx context.Context, runID int64, stageKey, worker string, leaseFor time.Duration, now time.Time) (Lease, error) {
	stage, err := r.GetStage(ctx, runID, stageKey)
	if err != nil {
		return Lease{}, err
	}
	return r.claimStageID(ctx, stage.ID, worker, leaseFor, now)
}

func (r *Repository) claimStageID(ctx context.Context, stageID int64, worker string, leaseFor time.Duration, now time.Time) (Lease, error) {
	out := Lease{WorkerID: worker, StageID: stageID}
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var status string
		var expires *time.Time
		var prevToken *uuid.UUID
		err := tx.QueryRow(ctx, `SELECT s.status,s.lease_expires_at,s.lease_token,s.run_id,r.seller_account_id,s.stage_key,COALESCE(s.input_hash,'')
			FROM analysis_run_stages s JOIN analysis_runs r ON r.id=s.run_id WHERE s.id=$1 FOR UPDATE OF s`, stageID).
			Scan(&status, &expires, &prevToken, &out.RunID, &out.SellerID, &out.Stage, &out.InputHash)
		if err != nil {
			return err
		}
		if status == "SUCCEEDED" || status == "SUCCEEDED_WITH_LIMITATIONS" {
			return ErrStageAlreadySucceeded
		}
		claimable := status == "READY" || (status == "RUNNING" && expires != nil && !expires.After(now.UTC()))
		if !claimable {
			return ErrStageNotClaimable
		}
		if out.InputHash == "" || out.InputHash == zeroInputHash || len(out.InputHash) != 64 {
			return ErrStageInputHashInvalid
		}
		if status == "RUNNING" && prevToken != nil {
			if _, err = tx.Exec(ctx, `UPDATE analysis_stage_attempts SET status='FAILED',failure_class='LEASE_EXPIRED',
				failure_code='WORKER_LOST',error_summary='lease expired before reclaim',finished_at=$2
				WHERE stage_id=$1 AND lease_token=$3 AND status='RUNNING'`, stageID, now.UTC(), *prevToken); err != nil {
				return err
			}
		}
		token := uuid.New()
		err = tx.QueryRow(ctx, `UPDATE analysis_run_stages SET status='RUNNING',lease_owner=$2,lease_token=$3,
				lease_version=lease_version+1,lease_expires_at=$4,heartbeat_at=$5,
				attempt_count=attempt_count+1,started_at=COALESCE(started_at,$5),error_code=NULL,error_message=NULL
			WHERE id=$1 RETURNING lease_token::text,lease_version,lease_expires_at,attempt_count`,
			stageID, worker, token, now.UTC().Add(leaseFor), now.UTC()).
			Scan(&out.Token, &out.Version, &out.ExpiresAt, &out.AttemptCount)
		if err != nil {
			return err
		}
		return tx.QueryRow(ctx, `INSERT INTO analysis_stage_attempts(
				stage_id,attempt_no,worker_id,lease_token,lease_version,started_at,status,input_hash)
			VALUES($1,$2,$3,$4::uuid,$5,$6,'RUNNING',$7)RETURNING id`,
			stageID, out.AttemptCount, worker, out.Token, out.Version, now.UTC(), out.InputHash).Scan(&out.AttemptID)
	})
	return out, err
}

func (r *Repository) Heartbeat(ctx context.Context, l Lease, expires time.Time) error {
	tag, err := r.pool.Exec(ctx, `UPDATE analysis_run_stages SET heartbeat_at=NOW(),lease_expires_at=$4
		WHERE id=$1 AND lease_token=$2::uuid AND lease_version=$3 AND status='RUNNING'`, l.StageID, l.Token, l.Version, expires.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	return nil
}

type RunEventInput struct {
	Kind    string
	Payload any
}

func (r *Repository) FinalizeSuccess(ctx context.Context, l Lease, outputArtifactID *int64, outputHash string, extra ...RunEventInput) error {
	return r.FinalizeCompleted(ctx, l, "SUCCEEDED", outputArtifactID, outputHash, extra...)
}

func (r *Repository) FinalizeCompleted(ctx context.Context, l Lease, status string, outputArtifactID *int64, outputHash string, extra ...RunEventInput) error {
	if status != "SUCCEEDED" && status != "SUCCEEDED_WITH_LIMITATIONS" {
		status = "SUCCEEDED"
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE analysis_run_stages SET status=$7,accepted_attempt_id=$4,
				accepted_artifact_id=$5,output_hash=NULLIF($6,''),lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL,
				completed_at=NOW(),error_code=NULL,error_message=NULL
			WHERE id=$1 AND lease_token=$2::uuid AND lease_version=$3 AND status='RUNNING'`,
			l.StageID, l.Token, l.Version, l.AttemptID, outputArtifactID, outputHash, status)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrLeaseLost
		}
		tag, err = tx.Exec(ctx, `UPDATE analysis_stage_attempts SET status=$6,output_hash=NULLIF($2,''),
				output_artifact_id=$3,finished_at=NOW()
			WHERE id=$1 AND lease_token=$4::uuid AND lease_version=$5 AND status='RUNNING'`,
			l.AttemptID, outputHash, outputArtifactID, l.Token, l.Version, status)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrLeaseLost
		}
		if _, err = tx.Exec(ctx, `UPDATE analysis_runs SET status='RUNNING',started_at=COALESCE(started_at,NOW()),current_stage=$2 WHERE id=$1 AND status IN('PREPARING','RUNNING')`,
			l.RunID, l.Stage); err != nil {
			return err
		}
		evType := "STAGE_SUCCEEDED"
		if status == "SUCCEEDED_WITH_LIMITATIONS" {
			evType = "STAGE_SUCCEEDED_WITH_LIMITATIONS"
		}
		_, err = tx.Exec(ctx, `INSERT INTO analysis_run_events(run_id,event_type,payload_json)VALUES($1,$2,$3)`,
			l.RunID, evType, mustJSON(map[string]any{"stage_key": l.Stage, "attempt_id": l.AttemptID, "status": status}))
		if err != nil {
			return err
		}
		for _, ev := range extra {
			if strings.TrimSpace(ev.Kind) == "" {
				continue
			}
			if _, err = tx.Exec(ctx, `INSERT INTO analysis_run_events(run_id,event_type,payload_json)VALUES($1,$2,$3)`,
				l.RunID, ev.Kind, mustJSON(ev.Payload)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) MarkNextStageReady(ctx context.Context, runID int64, completedStage string, allowedKeys []string) error {
	allowed := map[string]bool{}
	for _, k := range allowedKeys {
		allowed[k] = true
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var nextID int64
		var nextKey string
		err := tx.QueryRow(ctx, `SELECT id,stage_key FROM analysis_run_stages
			WHERE run_id=$1 AND status='PENDING' AND ordinal>(SELECT ordinal FROM analysis_run_stages WHERE run_id=$1 AND stage_key=$2)
			ORDER BY ordinal LIMIT 1`, runID, completedStage).Scan(&nextID, &nextKey)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if !allowed[nextKey] {
			return nil
		}
		if _, err = tx.Exec(ctx, `UPDATE analysis_run_stages SET status='READY' WHERE id=$1 AND status='PENDING'`, nextID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO analysis_run_events(run_id,event_type,payload_json)VALUES($1,'STAGE_READY',$2)`,
			runID, mustJSON(map[string]any{"stage_key": nextKey, "after": completedStage}))
		return err
	})
}

func (r *Repository) FinalizeRetryable(ctx context.Context, l Lease, failureClass, message string, nextRetry time.Time) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE analysis_run_stages SET status='FAILED_RETRYABLE',next_retry_at=$4,
				lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL,error_code=$5,error_message=$6
			WHERE id=$1 AND lease_token=$2::uuid AND lease_version=$3 AND status='RUNNING'`,
			l.StageID, l.Token, l.Version, nextRetry.UTC(), failureClass, message)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrLeaseLost
		}
		tag, err = tx.Exec(ctx, `UPDATE analysis_stage_attempts SET status='FAILED',failure_class=$2,failure_code=$2,
				error_summary=$3,finished_at=NOW()
			WHERE id=$1 AND lease_token=$4::uuid AND lease_version=$5 AND status='RUNNING'`,
			l.AttemptID, failureClass, message, l.Token, l.Version)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrLeaseLost
		}
		_, err = tx.Exec(ctx, `INSERT INTO analysis_run_events(run_id,event_type,payload_json)VALUES($1,'STAGE_FAILED',$2)`,
			l.RunID, mustJSON(map[string]any{"stage_key": l.Stage, "retryable": true, "failure_class": failureClass}))
		return err
	})
}

func (r *Repository) FinalizeTerminal(ctx context.Context, l Lease, failureClass, message string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE analysis_run_stages SET status='FAILED_TERMINAL',
				lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL,completed_at=NOW(),error_code=$4,error_message=$5
			WHERE id=$1 AND lease_token=$2::uuid AND lease_version=$3 AND status='RUNNING'`,
			l.StageID, l.Token, l.Version, failureClass, message)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrLeaseLost
		}
		tag, err = tx.Exec(ctx, `UPDATE analysis_stage_attempts SET status='FAILED',failure_class=$2,failure_code=$2,
				error_summary=$3,finished_at=NOW()
			WHERE id=$1 AND lease_token=$4::uuid AND lease_version=$5 AND status='RUNNING'`,
			l.AttemptID, failureClass, message, l.Token, l.Version)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrLeaseLost
		}
		if failureClass != "CANCELLED" {
			if _, err = tx.Exec(ctx, `UPDATE analysis_runs SET status='FAILED',completed_at=NOW(),error_code=$2,error_message=$3 WHERE id=$1`,
				l.RunID, failureClass, message); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO analysis_run_events(run_id,event_type,payload_json)VALUES($1,'RUN_FAILED',$2)`,
				l.RunID, mustJSON(map[string]any{"stage_key": l.Stage, "failure_class": failureClass}))
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO analysis_run_events(run_id,event_type,payload_json)VALUES($1,'STAGE_FAILED',$2)`,
			l.RunID, mustJSON(map[string]any{"stage_key": l.Stage, "retryable": false, "failure_class": failureClass}))
		return err
	})
}

func (r *Repository) Finalize(ctx context.Context, l Lease, success bool, failureClass, message string, outputArtifactID *int64) error {
	if success {
		return r.FinalizeSuccess(ctx, l, outputArtifactID, "")
	}
	return r.FinalizeTerminal(ctx, l, failureClass, message)
}

func (r *Repository) RequestCancel(ctx context.Context, runID int64, reason string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE analysis_runs SET cancel_requested_at=NOW(),cancel_reason_code=$2
		WHERE id=$1 AND status IN('PREPARING','RUNNING','READY_TO_PUBLISH') AND cancel_requested_at IS NULL`, runID, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		_, _ = r.pool.Exec(ctx, `INSERT INTO analysis_run_events(run_id,event_type,payload_json)VALUES($1,'RUN_CANCEL_REQUESTED',$2)`,
			runID, mustJSON(map[string]any{"reason": reason}))
	}
	return nil
}

func (r *Repository) ApplyCancelAtBoundary(ctx context.Context, runID int64) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var cancelAt *time.Time
		if err := tx.QueryRow(ctx, `SELECT cancel_requested_at FROM analysis_runs WHERE id=$1 FOR UPDATE`, runID).Scan(&cancelAt); err != nil {
			return err
		}
		if cancelAt == nil {
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE analysis_stage_attempts SET status='CANCELLED',finished_at=NOW(),failure_class='CANCELLED'
			WHERE stage_id IN (SELECT id FROM analysis_run_stages WHERE run_id=$1 AND status='RUNNING')
			  AND status='RUNNING'`, runID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE analysis_run_stages SET status='CANCELLED',completed_at=NOW(),lease_token=NULL,lease_expires_at=NULL
			WHERE run_id=$1 AND status IN('PENDING','READY','RUNNING','FAILED_RETRYABLE')`, runID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO analysis_run_events(run_id,event_type,payload_json)VALUES($1,'STAGE_CANCELLED','{}')`, runID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE analysis_runs SET status='CANCELLED',completed_at=NOW() WHERE id=$1`, runID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO analysis_run_events(run_id,event_type,payload_json)VALUES($1,'RUN_CANCELLED','{}')`, runID)
		return err
	})
}

func (r *Repository) ListRecoveryCandidates(ctx context.Context, now time.Time) ([]RecoveryCandidate, error) {
	rows, err := r.pool.Query(ctx, `SELECT s.run_id,s.id,r.seller_account_id,s.stage_key,s.status,
		CASE WHEN s.status='READY' THEN 'ready'
		     WHEN s.status='RUNNING' THEN 'lease_expired'
		     ELSE 'retry_due' END
		FROM analysis_run_stages s JOIN analysis_runs r ON r.id=s.run_id
		WHERE r.status IN('PREPARING','RUNNING')
		  AND(s.status='READY'
		   OR(s.status='RUNNING' AND s.lease_expires_at IS NOT NULL AND s.lease_expires_at<$1)
		   OR(s.status='FAILED_RETRYABLE' AND s.next_retry_at IS NOT NULL AND s.next_retry_at<=$1))
		ORDER BY r.requested_at,s.ordinal`, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecoveryCandidate
	for rows.Next() {
		var c RecoveryCandidate
		if err = rows.Scan(&c.RunID, &c.StageID, &c.SellerID, &c.StageKey, &c.Status, &c.Reason); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) AddEvent(ctx context.Context, runID int64, _ *int64, kind string, payload any) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO analysis_run_events(run_id,event_type,payload_json)VALUES($1,$2,$3)`, runID, kind, mustJSON(payload))
	return err
}

type ArtifactRecordInput struct {
	RunID          int64
	StageID        *int64
	StageAttemptID *int64
	StageKey       string
	Kind           string
	SchemaVersion  string
	StorageKey     string
	SHA256         string
	MediaType      string
	Compression    *string
	Bytes          int64
}

func (r *Repository) AddArtifactRecord(ctx context.Context, in ArtifactRecordInput) (int64, error) {
	if in.SchemaVersion == "" {
		in.SchemaVersion = "v1"
	}
	if in.MediaType == "" {
		in.MediaType = "application/json"
	}
	stageKey := in.StageKey
	if stageKey == "" {
		stageKey = "RUN"
	}
	var id int64
	err := r.pool.QueryRow(ctx, `INSERT INTO analysis_artifacts(run_id,stage_id,stage_attempt_id,stage_key,kind,schema_version,storage_key,sha256,media_type,compression,uncompressed_bytes)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT(sha256,storage_key) DO NOTHING RETURNING id`,
		in.RunID, in.StageID, in.StageAttemptID, stageKey, in.Kind, in.SchemaVersion, in.StorageKey, in.SHA256, in.MediaType, in.Compression, in.Bytes).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = r.pool.QueryRow(ctx, `SELECT id FROM analysis_artifacts WHERE sha256=$1 AND storage_key=$2`, in.SHA256, in.StorageKey).Scan(&id)
	}
	return id, err
}

func (r *Repository) AddArtifact(ctx context.Context, runID int64, stageID *int64, kind, key, hash string, size int64) (int64, error) {
	stageKey := "RUN"
	var attemptID *int64
	if stageID != nil {
		_ = r.pool.QueryRow(ctx, `SELECT s.stage_key,(SELECT a.id FROM analysis_stage_attempts a WHERE a.stage_id=s.id ORDER BY a.attempt_no DESC LIMIT 1)
			FROM analysis_run_stages s WHERE s.id=$1`, *stageID).Scan(&stageKey, &attemptID)
	}
	return r.AddArtifactRecord(ctx, ArtifactRecordInput{
		RunID: runID, StageID: stageID, StageAttemptID: attemptID, StageKey: stageKey,
		Kind: kind, SchemaVersion: "v1", StorageKey: key, SHA256: hash, Bytes: size,
	})
}

type AIAttemptInput struct {
	StageAttemptID    int64
	Provider, Model   string
	ProviderRequestID string
	InputTokens       *int64
	OutputTokens      *int64
	ReasoningTokens   *int64
	CachedTokens      *int64
	TotalTokens       *int64
	LatencyMS         *int64
	FinishReason      string
	IncompleteReason  string
	RequestHash       string
	ResponseHash      string
	StartedAt         *time.Time
	FinishedAt        *time.Time
	ErrorCode         string
	ErrorMessage      string
	ErrorMetadata     any
}

func (r *Repository) AddAIAttempt(ctx context.Context, in AIAttemptInput) (int64, error) {
	var id int64
	var meta []byte
	if in.ErrorMetadata != nil {
		meta = mustJSON(in.ErrorMetadata)
	}
	err := r.pool.QueryRow(ctx, `INSERT INTO analysis_ai_attempts(
			stage_attempt_id,provider,model,provider_request_id,input_tokens,output_tokens,reasoning_tokens,cached_tokens,total_tokens,
			latency_ms,finish_reason,incomplete_reason,request_hash,response_hash,started_at,finished_at,error_code,error_message,error_metadata_json)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19) RETURNING id`,
		in.StageAttemptID, nz(in.Provider, "OPENAI"), in.Model, nullIfEmpty(in.ProviderRequestID),
		in.InputTokens, in.OutputTokens, in.ReasoningTokens, in.CachedTokens, in.TotalTokens,
		in.LatencyMS, nullIfEmpty(in.FinishReason), nullIfEmpty(in.IncompleteReason),
		nullIfEmpty(in.RequestHash), nullIfEmpty(in.ResponseHash), in.StartedAt, in.FinishedAt,
		nullIfEmpty(in.ErrorCode), nullIfEmpty(in.ErrorMessage), meta).Scan(&id)
	return id, err
}

func (r *Repository) LatestArtifactByKind(ctx context.Context, runID int64, kind string) (id int64, key, hash string, attemptID *int64, err error) {
	err = r.pool.QueryRow(ctx, `SELECT id,storage_key,sha256,stage_attempt_id FROM analysis_artifacts WHERE run_id=$1 AND kind=$2 ORDER BY id DESC LIMIT 1`,
		runID, kind).Scan(&id, &key, &hash, &attemptID)
	return
}

func (r *Repository) ArtifactByStageAttemptAndKind(ctx context.Context, runID, stageAttemptID int64, kind string) (id int64, key, hash string, err error) {
	err = r.pool.QueryRow(ctx, `SELECT id,storage_key,sha256 FROM analysis_artifacts
		WHERE run_id=$1 AND stage_attempt_id=$3 AND kind=$2 ORDER BY id DESC LIMIT 1`,
		runID, kind, stageAttemptID).Scan(&id, &key, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", "", fmt.Errorf("analysis artifacts: %s not found for run %d attempt %d", kind, runID, stageAttemptID)
	}
	return
}

func (r *Repository) FindArtifactByHash(ctx context.Context, runID int64, hash string) (int64, bool, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `SELECT id FROM analysis_artifacts WHERE run_id=$1 AND sha256=$2 ORDER BY id LIMIT 1`, runID, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	return id, err == nil, err
}

func (r *Repository) ArtifactStorageKey(ctx context.Context, runID int64, hash string) (string, error) {
	var key string
	err := r.pool.QueryRow(ctx, `SELECT storage_key FROM analysis_artifacts WHERE run_id=$1 AND sha256=$2 ORDER BY id LIMIT 1`, runID, hash).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("analysis artifacts: output not found for run %d hash %s", runID, hash)
	}
	return key, err
}

func (r *Repository) PutContractBundle(ctx context.Context, hash, _ string, body []byte) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO analysis_contract_bundles(hash,schema_version,bundle_json)
		VALUES($1,'contract_bundle_v1',$2)ON CONFLICT(hash)DO NOTHING`, hash, body)
	return err
}

func (r *Repository) PutUniverse(ctx context.Context, in UniverseSealInput) (UniverseRecord, error) {
	u := in.Universe
	if u.ManifestVersion == "" {
		u.ManifestVersion = "factual_universe_manifest_v1"
	}
	if u.UniversePolicyVersion == "" {
		u.UniversePolicyVersion = "factual_universe_policy_v1"
	}
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `INSERT INTO factual_universes(seller_account_id,analysis_date,analysis_cutoff_at,
				manifest_hash,manifest_schema_version,manifest_artifact_id,temporal_policy_version,capability_policy_version,dq_policy_version,
				factual_api_version,universe_policy_version,seal_status)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'PREPARING')
			ON CONFLICT(seller_account_id,analysis_date,analysis_cutoff_at,manifest_hash)DO NOTHING
			RETURNING id::text,created_at`,
			u.SellerID, u.AnalysisDate, u.CutoffAt, u.ManifestHash, u.ManifestVersion, u.ManifestArtifactID, u.TemporalVersion, u.CapabilityVersion, u.DQVersion, u.APIVersion, u.UniversePolicyVersion).Scan(&u.ID, &u.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `SELECT id::text,created_at FROM factual_universes
				WHERE seller_account_id=$1 AND analysis_date=$2 AND analysis_cutoff_at=$3 AND manifest_hash=$4`,
				u.SellerID, u.AnalysisDate, u.CutoffAt, u.ManifestHash).Scan(&u.ID, &u.CreatedAt)
			if err != nil {
				return err
			}
			return r.loadUniverseChildrenTx(ctx, tx, &u)
		}
		if err != nil {
			return err
		}
		for _, s := range in.Sources {
			sem := nullIfEmpty(s.SemanticMappingHash)
			if _, err = tx.Exec(ctx, `INSERT INTO factual_universe_sources(
					universe_id,seller_account_id,domain,fact_family,source_binding_key,source_state,
					last_accepted_domain_run_id,coverage_from,coverage_to,snapshot_as_of,latest_observed_at,
					latest_complete_business_date,latest_mature_business_date,latest_complete_snapshot_at,
					source_contract_key,source_contract_version,source_contract_hash,
					semantic_mapping_hash,source_endpoint_family,semantic_mapping_version,source_revision_hash,
					watermark_hash,source_finalized_at,source_hash,binding_hash)
				VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25)
				ON CONFLICT DO NOTHING`,
				u.ID, u.SellerID, s.Domain, s.FactFamily, s.SourceBindingKey, s.SourceState, s.LastAcceptedDomainRunID,
				s.CoverageFrom, s.CoverageTo, s.SnapshotAsOf, s.LatestObservedAt, s.LatestCompleteBusinessDate, s.LatestMatureBusinessDate, s.LatestCompleteSnapshotAt,
				s.SourceContractKey, s.SourceContractVersion, s.SourceContractHash, sem, nullIfEmpty(s.SourceEndpointFamily), nullIfEmpty(s.SemanticMappingVersion), nullIfEmpty(s.SourceRevisionHash),
				nullIfEmpty(s.WatermarkHash), s.SourceFinalizedAt, s.SourceHash, s.BindingHash); err != nil {
				return err
			}
		}
		for _, c := range in.Capabilities {
			if _, err = tx.Exec(ctx, `INSERT INTO factual_universe_capabilities(
					universe_id,capability_key,scope,final_state,contract_state,access_state,source_state,semantic_state,
					coverage_state,temporal_state,history_state,population_state,reason_code,policy_version,assessment_id,assessment_hash,assessed_at)
				VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
				ON CONFLICT DO NOTHING`,
				u.ID, c.CapabilityKey, c.Scope, c.FinalState,
				nz(c.ContractState, "UNKNOWN"), nz(c.AccessState, "UNKNOWN"), nz(c.SourceState, "NOT_ATTEMPTED"),
				nz(c.SemanticState, "UNKNOWN_PROVIDER_SEMANTICS"), // CHECK has no bare UNKNOWN; never invent VALIDATED
				nz(c.CoverageState, "UNKNOWN"), nz(c.TemporalState, "UNKNOWN"), nz(c.HistoryState, "UNKNOWN"), nz(c.PopulationState, "UNKNOWN"),
				nullIfEmpty(c.ReasonCode), nz(c.PolicyVersion, u.CapabilityVersion), c.AssessmentID, c.AssessmentHash, c.AssessedAt); err != nil {
				return err
			}
		}
		for _, d := range in.DQ {
			sum := d.SummaryJSON
			if len(sum) == 0 {
				sum = []byte(`{}`)
			}
			if _, err = tx.Exec(ctx, `INSERT INTO factual_universe_dq_summary(
					universe_id,domain,fact_family,dq_run_id,dq_policy_version,dq_result_hash,highest_severity,
					blocking_issue_count,limitation_issue_count,warning_count,canonical_issue_set_hash,summary_json)
				VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
				ON CONFLICT DO NOTHING`,
				u.ID, d.Domain, d.FactFamily, d.DQRunID, d.DQPolicyVersion, d.DQResultHash, d.HighestSeverity,
				d.BlockingIssueCount, d.LimitationIssueCount, d.WarningCount, d.CanonicalIssueSetHash, sum); err != nil {
				return err
			}
		}
		for _, e := range in.EntitySets {
			if _, err = tx.Exec(ctx, `INSERT INTO factual_universe_entity_sets(universe_id,entity_kind,set_hash,member_count)
				VALUES($1::uuid,$2,$3,$4)ON CONFLICT DO NOTHING`, u.ID, e.EntityKind, e.SetHash, e.MemberCount); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE factual_universes SET seal_status='SEALED' WHERE id=$1::uuid AND seal_status='PREPARING'`, u.ID); err != nil {
			return err
		}
		u.Sources, u.Capabilities, u.DQ, u.EntitySets = in.Sources, in.Capabilities, in.DQ, in.EntitySets
		return nil
	})
	u.ContextHash = in.Universe.ContextHash
	u.ObjectKey = in.Universe.ObjectKey
	u.ManifestJSON = in.Universe.ManifestJSON
	u.ManifestHash = in.Universe.ManifestHash
	return u, err
}

func (r *Repository) loadUniverseChildrenTx(ctx context.Context, tx pgx.Tx, u *UniverseRecord) error {
	u.Sources, u.Capabilities, u.DQ, u.EntitySets = nil, nil, nil, nil
	rows, err := tx.Query(ctx, `SELECT domain,fact_family,source_binding_key,source_state,last_accepted_domain_run_id,
		coverage_from,coverage_to,snapshot_as_of,latest_observed_at,latest_complete_business_date,latest_mature_business_date,latest_complete_snapshot_at,
		source_contract_key,source_contract_version,source_contract_hash,COALESCE(semantic_mapping_hash,''),
		COALESCE(source_endpoint_family,''),COALESCE(semantic_mapping_version,''),COALESCE(source_revision_hash,''),
		COALESCE(watermark_hash,''),source_finalized_at,source_hash,binding_hash
		FROM factual_universe_sources WHERE universe_id=$1::uuid ORDER BY domain,fact_family,source_binding_key`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var s UniverseSource
		if err = rows.Scan(&s.Domain, &s.FactFamily, &s.SourceBindingKey, &s.SourceState, &s.LastAcceptedDomainRunID,
			&s.CoverageFrom, &s.CoverageTo, &s.SnapshotAsOf, &s.LatestObservedAt, &s.LatestCompleteBusinessDate, &s.LatestMatureBusinessDate, &s.LatestCompleteSnapshotAt,
			&s.SourceContractKey, &s.SourceContractVersion, &s.SourceContractHash, &s.SemanticMappingHash,
			&s.SourceEndpointFamily, &s.SemanticMappingVersion, &s.SourceRevisionHash, &s.WatermarkHash, &s.SourceFinalizedAt, &s.SourceHash, &s.BindingHash); err != nil {
			return err
		}
		u.Sources = append(u.Sources, s)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT capability_key,scope,final_state,contract_state,access_state,source_state,semantic_state,
		coverage_state,temporal_state,history_state,population_state,COALESCE(reason_code,''),policy_version,assessment_id,assessment_hash,assessed_at
		FROM factual_universe_capabilities WHERE universe_id=$1::uuid ORDER BY capability_key,scope`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var c UniverseCapability
		if err = rows.Scan(&c.CapabilityKey, &c.Scope, &c.FinalState, &c.ContractState, &c.AccessState, &c.SourceState, &c.SemanticState,
			&c.CoverageState, &c.TemporalState, &c.HistoryState, &c.PopulationState, &c.ReasonCode, &c.PolicyVersion, &c.AssessmentID, &c.AssessmentHash, &c.AssessedAt); err != nil {
			return err
		}
		u.Capabilities = append(u.Capabilities, c)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT domain,fact_family,dq_policy_version,dq_result_hash,highest_severity,
		blocking_issue_count,limitation_issue_count,warning_count,canonical_issue_set_hash,dq_run_id
		FROM factual_universe_dq_summary WHERE universe_id=$1::uuid ORDER BY domain,fact_family`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var d UniverseDQSummary
		if err = rows.Scan(&d.Domain, &d.FactFamily, &d.DQPolicyVersion, &d.DQResultHash, &d.HighestSeverity,
			&d.BlockingIssueCount, &d.LimitationIssueCount, &d.WarningCount, &d.CanonicalIssueSetHash, &d.DQRunID); err != nil {
			return err
		}
		u.DQ = append(u.DQ, d)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT entity_kind,set_hash,member_count FROM factual_universe_entity_sets
		WHERE universe_id=$1::uuid ORDER BY entity_kind`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var e UniverseEntitySet
		if err = rows.Scan(&e.EntityKind, &e.SetHash, &e.MemberCount); err != nil {
			return err
		}
		u.EntitySets = append(u.EntitySets, e)
	}
	return rows.Err()
}

func (r *Repository) GetUniverse(ctx context.Context, id string) (UniverseRecord, error) {
	var u UniverseRecord
	err := r.pool.QueryRow(ctx, `SELECT u.id::text,u.seller_account_id,u.analysis_date,u.analysis_cutoff_at,u.manifest_hash,
		u.manifest_schema_version,u.manifest_artifact_id,COALESCE(a.storage_key,''),u.temporal_policy_version,
		u.capability_policy_version,u.dq_policy_version,u.factual_api_version,u.universe_policy_version,u.created_at
		FROM factual_universes u LEFT JOIN analysis_artifacts a ON a.id=u.manifest_artifact_id WHERE u.id=$1::uuid`, id).
		Scan(&u.ID, &u.SellerID, &u.AnalysisDate, &u.CutoffAt, &u.ManifestHash, &u.ManifestVersion, &u.ManifestArtifactID, &u.ObjectKey, &u.TemporalVersion, &u.CapabilityVersion, &u.DQVersion, &u.APIVersion, &u.UniversePolicyVersion, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error { return r.loadUniverseChildrenTx(ctx, tx, &u) })
	return u, err
}

func (r *Repository) BindUniverse(ctx context.Context, runID int64, universeID, hash string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE analysis_runs SET factual_universe_id=$2::uuid
		WHERE id=$1 AND factual_universe_id IS NULL AND EXISTS(SELECT 1 FROM factual_universes WHERE id=$2::uuid AND manifest_hash=$3)`, runID, universeID, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("analysis postgres: universe already bound")
	}
	_, _ = r.pool.Exec(ctx, `INSERT INTO analysis_run_events(run_id,event_type,payload_json)VALUES($1,'UNIVERSE_SEALED',$2)`,
		runID, mustJSON(map[string]any{"universe_id": universeID, "manifest_hash": hash}))
	return nil
}

func (r *Repository) PriorSuccessfulProduction(ctx context.Context, seller int64, universeHash, contractHash, intentHash string) (int64, bool, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `SELECT r.id FROM analysis_runs r JOIN factual_universes u ON u.id=r.factual_universe_id
		WHERE r.seller_account_id=$1 AND r.execution_class='PRODUCTION' AND r.status IN('PUBLISHED','READY_TO_PUBLISH','NO_OP')
		  AND u.manifest_hash=$2 AND r.contract_bundle_hash=$3 AND r.run_intent_hash=$4
		ORDER BY COALESCE(r.completed_at,r.requested_at) DESC LIMIT 1`, seller, universeHash, contractHash, intentHash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	return id, err == nil, err
}

// MaterialityRunInput is the minimal SoT row for a succeeded Materiality stage.
type MaterialityRunInput struct {
	AnalysisRunID              int64
	SellerID                   int64
	UniverseID                 string
	UniverseManifestHash       string
	LandscapeCoreProfile       string
	InputLandscapeCoreHash     string
	PolicyVersion, PolicyHash  string
	BudgetVersion, BudgetHash  string
	RankingHash, SelectionHash string
	RankingArtifactID          *int64
	SelectionArtifactID        *int64
}

type MaterialitySelectionRow struct {
	EntityKind, SemanticKey, CanonicalRefID string
	SelectionOrder                          int
	SelectedAsAnchor                        bool
	AuditReasonCodes                        []string
	AuditSummaryJSON                        []byte
}

// PutMaterialityRun inserts an accepted Materiality run once.
// Same analysis_run_id + same semantic hashes → reuse. Different hashes → MATERIALITY_RESULT_CONFLICT.
func (r *Repository) PutMaterialityRun(ctx context.Context, in MaterialityRunInput) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO materiality_runs(
			analysis_run_id,seller_account_id,factual_universe_id,factual_universe_manifest_hash,
			landscape_core_profile_version,input_landscape_core_hash,
			materiality_policy_version,materiality_policy_hash,
			budget_profile_version,budget_profile_hash,status,
			ranking_hash,ranking_artifact_id,selection_hash,selection_artifact_id,completed_at)
		VALUES($1,$2,$3::uuid,$4,$5,$6,$7,$8,$9,$10,'SUCCEEDED',$11,$12,$13,$14,NOW())
		ON CONFLICT(analysis_run_id) DO NOTHING
		RETURNING id`,
		in.AnalysisRunID, in.SellerID, in.UniverseID, in.UniverseManifestHash,
		in.LandscapeCoreProfile, in.InputLandscapeCoreHash,
		in.PolicyVersion, in.PolicyHash, in.BudgetVersion, in.BudgetHash,
		in.RankingHash, in.RankingArtifactID, in.SelectionHash, in.SelectionArtifactID,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	var existing struct {
		ID                               int64
		RankingHash, SelectionHash       string
		PolicyHash, BudgetHash, CoreHash string
		PolicyVersion, BudgetVersion     string
	}
	err = r.pool.QueryRow(ctx, `
		SELECT id, COALESCE(ranking_hash,''), COALESCE(selection_hash,''),
			materiality_policy_hash, budget_profile_hash, input_landscape_core_hash,
			materiality_policy_version, budget_profile_version
		FROM materiality_runs WHERE analysis_run_id=$1`, in.AnalysisRunID).Scan(
		&existing.ID, &existing.RankingHash, &existing.SelectionHash,
		&existing.PolicyHash, &existing.BudgetHash, &existing.CoreHash,
		&existing.PolicyVersion, &existing.BudgetVersion,
	)
	if err != nil {
		return 0, err
	}
	if existing.RankingHash != in.RankingHash || existing.SelectionHash != in.SelectionHash ||
		existing.PolicyHash != in.PolicyHash || existing.BudgetHash != in.BudgetHash ||
		existing.CoreHash != in.InputLandscapeCoreHash ||
		existing.PolicyVersion != in.PolicyVersion || existing.BudgetVersion != in.BudgetVersion {
		return 0, fmt.Errorf("MATERIALITY_RESULT_CONFLICT: accepted materiality_run for analysis_run_id=%d", in.AnalysisRunID)
	}
	return existing.ID, nil
}

// PutMaterialitySelections inserts selection rows once for a materiality run.
// If rows already exist, verifies the same semantic set; never DELETE/replace.
func (r *Repository) PutMaterialitySelections(ctx context.Context, materialityRunID int64, rows []MaterialitySelectionRow) error {
	var existing int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM materiality_selections WHERE materiality_run_id=$1`, materialityRunID).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		if existing != len(rows) {
			return fmt.Errorf("MATERIALITY_SELECTION_IMMUTABLE: existing=%d new=%d run=%d", existing, len(rows), materialityRunID)
		}
		for _, row := range rows {
			var order int
			var anchor bool
			err := r.pool.QueryRow(ctx, `
				SELECT selection_order, selected_as_anchor FROM materiality_selections
				WHERE materiality_run_id=$1 AND entity_kind=$2 AND semantic_entity_key=$3`,
				materialityRunID, row.EntityKind, row.SemanticKey).Scan(&order, &anchor)
			if err != nil || order != row.SelectionOrder || anchor != row.SelectedAsAnchor {
				return fmt.Errorf("MATERIALITY_SELECTION_IMMUTABLE: mismatch for %s/%s", row.EntityKind, row.SemanticKey)
			}
		}
		return nil
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		for _, row := range rows {
			codes := row.AuditReasonCodes
			if codes == nil {
				codes = []string{}
			}
			summary := row.AuditSummaryJSON
			if len(summary) == 0 {
				summary = []byte("{}")
			}
			var ref any
			if row.CanonicalRefID != "" {
				ref = row.CanonicalRefID
			}
			if _, err := tx.Exec(ctx, `INSERT INTO materiality_selections(
				materiality_run_id,entity_kind,semantic_entity_key,canonical_ref_id,selection_order,selected_as_anchor,audit_reason_codes,audit_summary_json)
				VALUES($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb)`,
				materialityRunID, row.EntityKind, row.SemanticKey, ref, row.SelectionOrder, row.SelectedAsAnchor,
				mustJSON(codes), summary); err != nil {
				return err
			}
		}
		return nil
	})
}

// EnsureMaterialityPolicySeeds inserts policy/budget versions if absent.
// Same version + different hash → CONTRACT_VERSION_COLLISION. Never UPDATE in place.
func (r *Repository) UpsertMaterialityPolicySeeds(ctx context.Context, policyVersion, policyHash string, policyJSON []byte, budgetVersion, budgetHash string, budgetJSON []byte) error {
	if _, err := r.pool.Exec(ctx, `INSERT INTO materiality_policy_versions(version,policy_hash,policy_json)
		VALUES($1,$2,$3::jsonb) ON CONFLICT(version) DO NOTHING`,
		policyVersion, policyHash, policyJSON); err != nil {
		return err
	}
	var gotPolicyHash string
	if err := r.pool.QueryRow(ctx, `SELECT policy_hash FROM materiality_policy_versions WHERE version=$1`, policyVersion).Scan(&gotPolicyHash); err != nil {
		return err
	}
	if gotPolicyHash != policyHash {
		return fmt.Errorf("CONTRACT_VERSION_COLLISION: materiality_policy_versions.version=%s", policyVersion)
	}
	if _, err := r.pool.Exec(ctx, `INSERT INTO materiality_budget_profiles(version,budget_hash,budget_json)
		VALUES($1,$2,$3::jsonb) ON CONFLICT(version) DO NOTHING`,
		budgetVersion, budgetHash, budgetJSON); err != nil {
		return err
	}
	var gotBudgetHash string
	if err := r.pool.QueryRow(ctx, `SELECT budget_hash FROM materiality_budget_profiles WHERE version=$1`, budgetVersion).Scan(&gotBudgetHash); err != nil {
		return err
	}
	if gotBudgetHash != budgetHash {
		return fmt.Errorf("CONTRACT_VERSION_COLLISION: materiality_budget_profiles.version=%s", budgetVersion)
	}
	return nil
}

func nz(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{}`)
	}
	return b
}
