package eval

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	UpsertSuite(ctx context.Context, s Suite) (int64, error)
	InsertCase(ctx context.Context, c Case) (int64, error)
	AttachCase(ctx context.Context, sc SuiteCase) error
	InsertRun(ctx context.Context, r Run) (int64, error)
	UpdateRun(ctx context.Context, r Run) error
	InsertCaseRun(ctx context.Context, cr CaseRun) (int64, error)
	InsertGraderResult(ctx context.Context, g GraderResult) error
	InsertHumanReview(ctx context.Context, h HumanReview) error
	GetRun(ctx context.Context, id int64) (Run, error)
	GetCaseRun(ctx context.Context, id int64) (CaseRun, error)
	ListCaseRuns(ctx context.Context, runID int64) ([]CaseRun, error)
	PromoteBaseline(ctx context.Context, suiteID int64, split string, runID int64, by string) error
	CurrentBaseline(ctx context.Context, suiteID int64, split string) (int64, bool, error)
	ListHumanReviews(ctx context.Context, runID int64) ([]HumanReview, error)
}

type Repository struct {
	Pool *pgxpool.Pool
}

func (r *Repository) UpsertSuite(ctx context.Context, s Suite) (int64, error) {
	var id int64
	err := r.Pool.QueryRow(ctx, `INSERT INTO eval_suites(suite_key,version,dataset_kind,description,rubric_version,manifest_artifact_key,manifest_artifact_hash)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT(suite_key,version) DO UPDATE SET description=EXCLUDED.description
		RETURNING id`, s.SuiteKey, s.Version, s.DatasetKind, s.Description, s.RubricVersion, null(s.ManifestArtifactKey), null(s.ManifestArtifactHash)).Scan(&id)
	return id, err
}

func (r *Repository) InsertCase(ctx context.Context, c Case) (int64, error) {
	exp, _ := json.Marshal(c.Expected)
	tags, _ := json.Marshal(c.Tags)
	var id int64
	err := r.Pool.QueryRow(ctx, `INSERT INTO eval_cases(case_key,version,fixture_artifact_key,fixture_artifact_hash,analysis_date,analysis_cutoff_at,expected_json,tags_json)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT(case_key,version) DO UPDATE SET expected_json=EXCLUDED.expected_json
		RETURNING id`, c.CaseKey, c.Version, null(c.FixtureArtifactKey), null(c.FixtureArtifactHash), c.AnalysisDate, c.AnalysisCutoffAt, exp, tags).Scan(&id)
	return id, err
}

func (r *Repository) AttachCase(ctx context.Context, sc SuiteCase) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO eval_suite_cases(suite_id,case_id,split,ordinal,critical) VALUES($1,$2,$3,$4,$5)
		ON CONFLICT(suite_id,case_id) DO UPDATE SET split=EXCLUDED.split,ordinal=EXCLUDED.ordinal,critical=EXCLUDED.critical`,
		sc.SuiteID, sc.CaseID, sc.Split, sc.Ordinal, sc.Critical)
	return err
}

func (r *Repository) InsertRun(ctx context.Context, run Run) (int64, error) {
	models, _ := json.Marshal(run.ResolvedModels)
	agg, _ := json.Marshal(run.Aggregate)
	reason := run.ReasoningConfig
	if len(reason) == 0 {
		reason = []byte("{}")
	}
	graders := run.GraderVersions
	if len(graders) == 0 {
		graders = []byte("{}")
	}
	var id int64
	err := r.Pool.QueryRow(ctx, `INSERT INTO eval_runs(
		suite_id,split,purpose,status,verdict,code_sha,ucdm_version,factual_api_version,temporal_policy_version,
		capability_policy_version,materiality_policy_version,discovery_prompt_version,discovery_schema_version,
		discovery_model_policy_version,evidence_catalog_version,deep_prompt_version,deep_schema_version,
		deep_model_policy_version,action_catalog_version,action_continuity_policy_version,projection_version,
		chat_prompt_version,chat_tool_policy_version,chat_model_policy_version,contract_bundle_hash,
		resolved_models_json,reasoning_config_json,grader_versions_json,critical_failures_count,aggregate_json,
		report_artifact_key,report_artifact_hash)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32)
		RETURNING id`,
		run.SuiteID, run.Split, run.Purpose, run.Status, run.Verdict, run.CodeSHA, run.UCDMVersion, run.FactualAPIVersion,
		run.TemporalPolicyVersion, run.CapabilityPolicyVersion, run.MaterialityPolicyVersion, run.DiscoveryPromptVersion,
		run.DiscoverySchemaVersion, run.DiscoveryModelPolicy, run.EvidenceCatalogVersion, run.DeepPromptVersion, run.DeepSchemaVersion,
		run.DeepModelPolicy, run.ActionCatalogVersion, run.ActionContinuityPolicy, run.ProjectionVersion,
		run.ChatPromptVersion, run.ChatToolPolicyVersion, run.ChatModelPolicyVersion, run.ContractBundleHash,
		models, reason, graders, run.CriticalFailuresCount, agg, null(run.ReportArtifactKey), null(run.ReportArtifactHash),
	).Scan(&id)
	return id, err
}

func (r *Repository) UpdateRun(ctx context.Context, run Run) error {
	agg, _ := json.Marshal(run.Aggregate)
	_, err := r.Pool.Exec(ctx, `UPDATE eval_runs SET status=$2,verdict=$3,critical_failures_count=$4,aggregate_json=$5,
		completed_at=$6,report_artifact_key=$7,report_artifact_hash=$8 WHERE id=$1`,
		run.ID, run.Status, run.Verdict, run.CriticalFailuresCount, agg, run.CompletedAt, null(run.ReportArtifactKey), null(run.ReportArtifactHash))
	return err
}

func (r *Repository) InsertCaseRun(ctx context.Context, cr CaseRun) (int64, error) {
	var id int64
	err := r.Pool.QueryRow(ctx, `INSERT INTO eval_case_runs(eval_run_id,case_id,split,status,verdict,analysis_run_id,input_tokens,output_tokens,cached_tokens,reasoning_tokens,estimated_cost_usd,pricing_version,latency_ms,artifact_key,artifact_hash,error_code)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) RETURNING id`,
		cr.EvalRunID, cr.CaseID, cr.Split, cr.Status, cr.Verdict, cr.AnalysisRunID, cr.InputTokens, cr.OutputTokens, cr.CachedTokens, cr.ReasoningTokens, cr.EstimatedCostUSD, null(cr.PricingVersion), cr.LatencyMS, null(cr.ArtifactKey), null(cr.ArtifactHash), null(cr.ErrorCode)).Scan(&id)
	return id, err
}

func (r *Repository) InsertGraderResult(ctx context.Context, g GraderResult) error {
	score, _ := json.Marshal(g.Score)
	_, err := r.Pool.Exec(ctx, `INSERT INTO eval_grader_results(case_run_id,grader_key,grader_version,grader_prompt_hash,grader_model,rubric_version,score_json,critical_fail,artifact_key,artifact_hash)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, g.CaseRunID, g.GraderKey, g.GraderVersion, null(g.GraderPromptHash), null(g.GraderModel), null(g.RubricVersion), score, g.CriticalFail, null(g.ArtifactKey), null(g.ArtifactHash))
	return err
}

func (r *Repository) InsertHumanReview(ctx context.Context, h HumanReview) error {
	ratings, _ := json.Marshal(h.Ratings)
	return r.Pool.QueryRow(ctx, `INSERT INTO eval_human_reviews(case_run_id,reviewer,status,ratings_json,notes,packet_artifact_key,packet_artifact_hash)
		VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, h.CaseRunID, h.Reviewer, h.Status, ratings, h.Notes, null(h.PacketArtifactKey), null(h.PacketArtifactHash)).Scan(&h.ID)
}

func (r *Repository) GetCaseRun(ctx context.Context, id int64) (CaseRun, error) {
	var cr CaseRun
	err := r.Pool.QueryRow(ctx, `SELECT id,eval_run_id,case_id,split,status,verdict,analysis_run_id,input_tokens,output_tokens,cached_tokens,reasoning_tokens,latency_ms,COALESCE(artifact_key,''),COALESCE(artifact_hash,''),COALESCE(error_code,'')
		FROM eval_case_runs WHERE id=$1`, id).Scan(&cr.ID, &cr.EvalRunID, &cr.CaseID, &cr.Split, &cr.Status, &cr.Verdict, &cr.AnalysisRunID, &cr.InputTokens, &cr.OutputTokens, &cr.CachedTokens, &cr.ReasoningTokens, &cr.LatencyMS, &cr.ArtifactKey, &cr.ArtifactHash, &cr.ErrorCode)
	return cr, err
}

func (r *Repository) ListCaseRuns(ctx context.Context, runID int64) ([]CaseRun, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,eval_run_id,case_id,split,status,verdict,analysis_run_id,input_tokens,output_tokens,cached_tokens,reasoning_tokens,latency_ms,COALESCE(artifact_key,''),COALESCE(artifact_hash,''),COALESCE(error_code,'')
		FROM eval_case_runs WHERE eval_run_id=$1 ORDER BY id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CaseRun
	for rows.Next() {
		var cr CaseRun
		if err := rows.Scan(&cr.ID, &cr.EvalRunID, &cr.CaseID, &cr.Split, &cr.Status, &cr.Verdict, &cr.AnalysisRunID, &cr.InputTokens, &cr.OutputTokens, &cr.CachedTokens, &cr.ReasoningTokens, &cr.LatencyMS, &cr.ArtifactKey, &cr.ArtifactHash, &cr.ErrorCode); err != nil {
			return nil, err
		}
		out = append(out, cr)
	}
	return out, rows.Err()
}

func (r *Repository) GetRun(ctx context.Context, id int64) (Run, error) {
	var run Run
	var models, agg []byte
	err := r.Pool.QueryRow(ctx, `SELECT id,suite_id,split,purpose,status,verdict,code_sha,critical_failures_count,aggregate_json,resolved_models_json,started_at,completed_at,COALESCE(report_artifact_key,''),COALESCE(report_artifact_hash,'')
		FROM eval_runs WHERE id=$1`, id).Scan(&run.ID, &run.SuiteID, &run.Split, &run.Purpose, &run.Status, &run.Verdict, &run.CodeSHA, &run.CriticalFailuresCount, &agg, &models, &run.StartedAt, &run.CompletedAt, &run.ReportArtifactKey, &run.ReportArtifactHash)
	if err != nil {
		return Run{}, err
	}
	_ = json.Unmarshal(agg, &run.Aggregate)
	_ = json.Unmarshal(models, &run.ResolvedModels)
	return run, nil
}

func (r *Repository) PromoteBaseline(ctx context.Context, suiteID int64, split string, runID int64, by string) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO eval_baselines(suite_id,split,run_id,promoted_by) VALUES($1,$2,$3,$4)
		ON CONFLICT(suite_id,split) DO UPDATE SET run_id=EXCLUDED.run_id,promoted_at=NOW(),promoted_by=EXCLUDED.promoted_by`,
		suiteID, split, runID, by)
	return err
}

func (r *Repository) CurrentBaseline(ctx context.Context, suiteID int64, split string) (int64, bool, error) {
	var id int64
	err := r.Pool.QueryRow(ctx, `SELECT run_id FROM eval_baselines WHERE suite_id=$1 AND split=$2`, suiteID, split).Scan(&id)
	if err != nil {
		return 0, false, nil
	}
	return id, true, nil
}

func (r *Repository) ListHumanReviews(ctx context.Context, runID int64) ([]HumanReview, error) {
	rows, err := r.Pool.Query(ctx, `SELECT r.id,r.case_run_id,r.reviewer,r.status,r.ratings_json,r.notes,COALESCE(r.packet_artifact_key,''),COALESCE(r.packet_artifact_hash,'')
		FROM eval_human_reviews r JOIN eval_case_runs c ON c.id=r.case_run_id WHERE c.eval_run_id=$1`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HumanReview
	for rows.Next() {
		var h HumanReview
		var ratings []byte
		if err := rows.Scan(&h.ID, &h.CaseRunID, &h.Reviewer, &h.Status, &ratings, &h.Notes, &h.PacketArtifactKey, &h.PacketArtifactHash); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(ratings, &h.Ratings)
		out = append(out, h)
	}
	return out, rows.Err()
}

func null(s string) any {
	if s == "" {
		return nil
	}
	return s
}
