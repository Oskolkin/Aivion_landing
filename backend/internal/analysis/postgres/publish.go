package postgres

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"crypto/sha256"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actionreconcile"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/publication"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrStalePublish            = errors.New("STALE_PUBLISH")
	ErrEvaluationCannotPublish = errors.New("EVALUATION_CANNOT_PUBLISH")
	ErrPublishAlreadyDone      = errors.New("PUBLISH_IDEMPOTENT")
)

type PublishInput struct {
	Run              Run
	Lease            Lease
	Candidate        publication.CandidateV1
	Plan             actionreconcile.PlanV1
	Validated        deep.ValidatedOutputV1
	ProjectionBytes  map[string][]byte
	ProjectionHashes map[string]string
	ComponentSHA     map[string]string
	Now              time.Time
	AllowedNext      []string
}

type PublishResult struct {
	PublicationID int64
	Generation    int64
	Idempotent    bool
	Stale         bool
}

func SellerAdvisoryLockKey(sellerID int64) int64 {
	sum := sha256.Sum256([]byte("aivion.publication.seller:" + strconv.FormatInt(sellerID, 10)))
	return int64(binary.BigEndian.Uint64(sum[:8]))
}

func (r *Repository) CurrentPublicationID(ctx context.Context, sellerID int64) (*int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `SELECT publication_id FROM seller_current_publications WHERE seller_account_id=$1`, sellerID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (r *Repository) PublishAtomic(ctx context.Context, in PublishInput) (PublishResult, error) {
	if in.Run.ExecutionClass == "EVALUATION" {
		return PublishResult{}, ErrEvaluationCannotPublish
	}
	var out PublishResult
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, SellerAdvisoryLockKey(in.Run.SellerID)); err != nil {
			return err
		}
		var existingID int64
		err := tx.QueryRow(ctx, `SELECT id FROM analysis_publications WHERE analysis_run_id=$1`, in.Run.ID).Scan(&existingID)
		if err == nil {
			out = PublishResult{PublicationID: existingID, Idempotent: true}
			var stageStatus string
			if serr := tx.QueryRow(ctx, `SELECT status FROM analysis_run_stages WHERE id=$1`, in.Lease.StageID).Scan(&stageStatus); serr != nil {
				return serr
			}
			if stageStatus == "SUCCEEDED" || stageStatus == "SUCCEEDED_WITH_LIMITATIONS" {
				return nil
			}
			return finalizePublishTx(ctx, tx, in.Lease, existingID, in.Candidate.PublicationContentHash, in.AllowedNext)
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var curID *int64
		var got int64
		cerr := tx.QueryRow(ctx, `SELECT publication_id FROM seller_current_publications WHERE seller_account_id=$1`, in.Run.SellerID).Scan(&got)
		if cerr == nil {
			curID = &got
		} else if !errors.Is(cerr, pgx.ErrNoRows) {
			return cerr
		}
		if !sameBase(curID, in.Run.BasePublicationID) {
			out.Stale = true
			return ErrStalePublish
		}
		var nextGen int64 = 1
		_ = tx.QueryRow(ctx, `SELECT COALESCE(MAX(publication_generation),0)+1 FROM analysis_publications WHERE seller_account_id=$1`, in.Run.SellerID).Scan(&nextGen)
		if in.Candidate.ExpectedPublicationGeneration > 0 && in.Candidate.ExpectedPublicationGeneration != nextGen {
			out.Stale = true
			return ErrStalePublish
		}
		universeID := in.Candidate.FactualUniverseID
		if universeID == "" {
			universeID = in.Run.UniverseID
		}
		var pubID int64
		err = tx.QueryRow(ctx, `INSERT INTO analysis_publications(
			seller_account_id,publication_generation,analysis_run_id,analysis_date,analysis_cutoff_at,
			factual_universe_id,factual_universe_hash,contract_bundle_hash,publication_content_hash)
			VALUES($1,$2,$3,$4,$5,$6::uuid,$7,$8,$9) RETURNING id`,
			in.Run.SellerID, nextGen, in.Run.ID, in.Run.AnalysisDate, in.Run.CutoffAt,
			universeID, in.Candidate.FactualUniverseHash, in.Candidate.ContractBundleHash,
			in.Candidate.PublicationContentHash).Scan(&pubID)
		if err != nil {
			return err
		}
		out.PublicationID = pubID
		out.Generation = nextGen
		if err = maybeFailpoint(FailpointAfterPublicationInsert); err != nil {
			return err
		}
		for _, c := range in.Candidate.Components {
			if _, err = tx.Exec(ctx, `INSERT INTO publication_components(publication_id,component_type,schema_version,sha256) VALUES($1,$2,$3,$4)`,
				pubID, c.ComponentType, c.SchemaVersion, c.SHA256); err != nil {
				return err
			}
		}
		for i, f := range in.Validated.Output.Findings {
			ref := fmt.Sprintf("F%03d", i+1)
			if i < len(in.Validated.FindingRefs) {
				ref = in.Validated.FindingRefs[i]
			}
			if _, err = tx.Exec(ctx, `INSERT INTO publication_findings(publication_id,finding_ref,ordinal,primary_kind,primary_id,statement,display_order,grounding_refs_json)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
				pubID, ref, f.Ordinal, string(f.PrimaryEntity.Kind), f.PrimaryEntity.ID, f.FindingStatement, i+1, mustJSON(f.SupportingFactHandles)); err != nil {
				return err
			}
		}
		for i, rec := range in.Validated.Output.Recommendations {
			ref := fmt.Sprintf("R%03d", i+1)
			if i < len(in.Validated.RecommendationRefs) {
				ref = in.Validated.RecommendationRefs[i]
			}
			if _, err = tx.Exec(ctx, `INSERT INTO publication_recommendations(publication_id,recommendation_ref,ordinal,actionability,summary,finding_refs_json,display_order)
				VALUES($1,$2,$3,$4,$5,$6,$7)`,
				pubID, ref, rec.Ordinal, rec.Actionability, rec.RecommendationStatement, mustJSON(rec.SupportFindingOrdinals), i+1); err != nil {
				return err
			}
		}
		if err = applyActionPlan(ctx, tx, in, pubID); err != nil {
			return err
		}
		if err = maybeFailpoint(FailpointAfterActionMutations); err != nil {
			return err
		}
		for key, raw := range in.ProjectionBytes {
			sum := in.ProjectionHashes[key]
			if sum == "" {
				h := sha256.Sum256(raw)
				sum = fmt.Sprintf("%x", h)
			}
			switch key {
			case "STOCK_SKU_ROWS":
				if err = insertProjectionRows(ctx, tx, pubID, "publication_stock_sku_rows", "sku_id", raw); err != nil {
					return err
				}
				continue
			case "AD_CAMPAIGN_ROWS":
				if err = insertProjectionRows(ctx, tx, pubID, "publication_ad_campaign_rows", "campaign_id", raw); err != nil {
					return err
				}
				continue
			case "SKU_FOCUS_ROWS":
				if err = insertProjectionRows(ctx, tx, pubID, "publication_sku_focus_rows", "sku_id", raw); err != nil {
					return err
				}
				continue
			}
			payload := json.RawMessage(raw)
			schema := projectionSchemaVersion(raw)
			if _, err = tx.Exec(ctx, `INSERT INTO publication_projections(publication_id,projection_key,schema_version,payload_json,payload_hash)
				VALUES($1,$2,$3,$4,$5)`, pubID, key, schema, payload, sum); err != nil {
				return err
			}
		}
		if err = maybeFailpoint(FailpointAfterProjections); err != nil {
			return err
		}
		if err = maybeFailpoint(FailpointBeforeCurrentPointer); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO seller_current_publications(seller_account_id,publication_id,updated_at)
			VALUES($1,$2,NOW()) ON CONFLICT (seller_account_id) DO UPDATE SET publication_id=EXCLUDED.publication_id,updated_at=NOW()`,
			in.Run.SellerID, pubID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO publication_release_events(publication_id,seller_account_id,event_type) VALUES($1,$2,'PUBLISHED'),($1,$2,'MADE_CURRENT')`,
			pubID, in.Run.SellerID); err != nil {
			return err
		}
		oid := uuid.New()
		if _, err = tx.Exec(ctx, `INSERT INTO analysis_outbox_events(id,event_type,aggregate_type,aggregate_id,payload_json)
			VALUES($1,'publication.current_changed','publication',$2,$3)`,
			oid, strconv.FormatInt(pubID, 10), mustJSON(map[string]any{"seller_account_id": in.Run.SellerID, "publication_id": pubID})); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE analysis_runs SET status='PUBLISHED',completed_at=NOW(),current_stage='PUBLISH' WHERE id=$1`, in.Run.ID); err != nil {
			return err
		}
		return finalizePublishTx(ctx, tx, in.Lease, pubID, in.Candidate.PublicationContentHash, in.AllowedNext)
	})
	if errors.Is(err, ErrStalePublish) {
		_ = r.markRunStale(ctx, in.Lease)
		return out, err
	}
	if err == nil && TestPublishUnknownCommitAfterSuccess {
		return out, ErrPublishUnknownCommit
	}
	return out, err
}

func insertProjectionRows(ctx context.Context, tx pgx.Tx, pubID int64, table, idCol string, raw []byte) error {
	var rows []map[string]any
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return fmt.Errorf("decode %s: %w", table, err)
	}
	idKey := idCol
	for _, row := range rows {
		id, _ := row[idKey].(string)
		if id == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO `+table+`(publication_id,`+idCol+`,row_json) VALUES($1,$2,$3)`,
			pubID, id, mustJSON(row)); err != nil {
			return err
		}
	}
	return nil
}

func sameBase(cur, expected *int64) bool {
	var cz, ez int64
	if cur != nil {
		cz = *cur
	}
	if expected != nil {
		ez = *expected
	}
	return cz == ez
}

func finalizePublishTx(ctx context.Context, tx pgx.Tx, l Lease, pubID int64, hash string, allowed []string) error {
	tag, err := tx.Exec(ctx, `UPDATE analysis_run_stages SET status='SUCCEEDED',accepted_attempt_id=$4,
			output_hash=NULLIF($5,''),lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL,completed_at=NOW()
		WHERE id=$1 AND lease_token=$2::uuid AND lease_version=$3 AND status='RUNNING'`,
		l.StageID, l.Token, l.Version, l.AttemptID, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	tag, err = tx.Exec(ctx, `UPDATE analysis_stage_attempts SET status='SUCCEEDED',output_hash=NULLIF($2,''),finished_at=NOW()
		WHERE id=$1 AND lease_token=$3::uuid AND lease_version=$4 AND status='RUNNING'`,
		l.AttemptID, hash, l.Token, l.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	if _, err = tx.Exec(ctx, `INSERT INTO analysis_run_events(run_id,event_type,payload_json)VALUES($1,'STAGE_SUCCEEDED',$2)`,
		l.RunID, mustJSON(map[string]any{"stage_key": l.Stage, "publication_id": pubID})); err != nil {
		return err
	}
	allowedM := map[string]bool{}
	for _, k := range allowed {
		allowedM[k] = true
	}
	var nextID int64
	var nextKey string
	err = tx.QueryRow(ctx, `SELECT id,stage_key FROM analysis_run_stages
		WHERE run_id=$1 AND status='PENDING' AND ordinal>(SELECT ordinal FROM analysis_run_stages WHERE run_id=$1 AND stage_key=$2)
		ORDER BY ordinal LIMIT 1`, l.RunID, l.Stage).Scan(&nextID, &nextKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if allowedM[nextKey] {
		if _, err = tx.Exec(ctx, `UPDATE analysis_run_stages SET status='READY' WHERE id=$1 AND status='PENDING'`, nextID); err != nil {
			return err
		}
	}
	_ = canonicaljson.Hash
	_ = time.Now
	return nil
}

func (r *Repository) markRunStale(ctx context.Context, l Lease) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		_, _ = tx.Exec(ctx, `UPDATE analysis_run_stages SET status='FAILED_TERMINAL',error_code='STALE',lease_owner=NULL,lease_token=NULL,completed_at=NOW()
			WHERE id=$1 AND lease_token=$2::uuid AND lease_version=$3`, l.StageID, l.Token, l.Version)
		_, _ = tx.Exec(ctx, `UPDATE analysis_runs SET status='STALE',error_code='STALE_PUBLISH',completed_at=NOW() WHERE id=$1`, l.RunID)
		return nil
	})
}

func applyActionPlan(ctx context.Context, tx pgx.Tx, in PublishInput, pubID int64) error {
	caseBySig := map[string]uuid.UUID{}
	for _, np := range in.Plan.NewProposalIntents {
		var sigID int64
		err := tx.QueryRow(ctx, `INSERT INTO action_signatures(seller_account_id,signature_hash,action_type,target_kind,target_id,normalized_context_hash,normalized_context_json,intent_key)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (seller_account_id,signature_hash) DO UPDATE SET action_type=EXCLUDED.action_type
			RETURNING id`,
			in.Run.SellerID, np.SignatureHash, np.ActionType, string(np.Target.Kind), np.Target.ID,
			actionreconcile.ContextHash(np.NormalizedContext), mustJSON(np.NormalizedContext), np.IntentSignature).Scan(&sigID)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO action_signature_state(signature_id,last_occurrence_no,updated_at) VALUES($1,0,NOW()) ON CONFLICT (signature_id) DO NOTHING`, sigID); err != nil {
			return err
		}
		var occ int
		var active uuid.UUID
		_ = tx.QueryRow(ctx, `SELECT last_occurrence_no,COALESCE(active_case_id,'00000000-0000-0000-0000-000000000000') FROM action_signature_state WHERE signature_id=$1`, sigID).Scan(&occ, &active)
		caseID := active
		if np.OpenNewOccurrence || caseID == uuid.Nil {
			occ++
			caseID = uuid.New()
			if _, err = tx.Exec(ctx, `INSERT INTO action_cases(id,signature_id,occurrence_no,opened_by_publication_id) VALUES($1,$2,$3,$4)`,
				caseID, sigID, occ, pubID); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO action_case_state(case_id,system_state,user_state,updated_at) VALUES($1,'ACTIVE','OPEN',NOW())`, caseID); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE action_signature_state SET active_case_id=$2,last_occurrence_no=$3,updated_at=NOW() WHERE signature_id=$1`, sigID, caseID, occ); err != nil {
				return err
			}
		}
		propID := uuid.New()
		if _, err = tx.Exec(ctx, `INSERT INTO action_case_proposals(id,case_id,publication_id,recommendation_id,action_type,parameters_json,display_order)
			VALUES($1,$2,$3,$4,$5,$6,$7)`,
			propID, caseID, pubID, np.RecommendationRef, np.ActionType, mustJSON(np.Parameters), np.DisplayOrder); err != nil {
			return err
		}
		for _, sp := range np.Supports {
			if _, err = tx.Exec(ctx, `INSERT INTO action_proposal_supports(proposal_id,support_key,support_role,claim,fact_refs_json,finding_refs_json,revalidation_json)
				VALUES($1,$2,$3,$4,$5,$6,$7)`,
				propID, sp.SupportKey, sp.Role, sp.Claim, mustJSON(sp.FactHandles), mustJSON(sp.FindingOrdinals), mustJSON(sp.Revalidation)); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE action_case_state SET current_proposal_id=$2,updated_at=NOW() WHERE case_id=$1`, caseID, propID); err != nil {
			return err
		}
		if np.Disposition == "SUPPRESSED_USER_DISMISSED" {
			if _, err = tx.Exec(ctx, `UPDATE action_case_state SET user_state='DISMISSED',updated_at=NOW() WHERE case_id=$1`, caseID); err != nil {
				return err
			}
		}
		caseBySig[np.SignatureHash] = caseID
		_ = np.Visible
	}
	for _, po := range in.Plan.PreviousOutcomes {
		sys := "ACTIVE"
		switch po.SystemTransition {
		case "RESOLVED":
			sys = "RESOLVED"
		case "SUSPENDED":
			sys = "SUSPENDED"
		case "SUPERSEDED", "BASIS_CHANGED":
			sys = "SUPERSEDED"
			if po.SystemTransition == "BASIS_CHANGED" {
				sys = "RESOLVED"
			}
		case "REAFFIRM", "KEEP_DONE_OPEN_NEW":
			sys = "ACTIVE"
		}
		if po.CaseRef != "" {
			cid, err := uuid.Parse(po.CaseRef)
			if err == nil {
				if po.ProposalRef != "" && po.DeepAssessment != "" {
					prevPropID, perr := uuid.Parse(po.ProposalRef)
					if perr == nil {
						assessID := uuid.New()
						if err = tx.QueryRow(ctx, `INSERT INTO action_case_assessments(id,previous_proposal_id,analysis_run_id,new_publication_id,outcome)
							VALUES($1,$2,$3,$4,$5)
							ON CONFLICT (analysis_run_id, previous_proposal_id) DO UPDATE SET outcome=EXCLUDED.outcome
							RETURNING id`, assessID, prevPropID, in.Run.ID, pubID, po.DeepAssessment).Scan(&assessID); err != nil {
							return err
						}
						for _, sa := range po.SupportAssessments {
							if _, err = tx.Exec(ctx, `INSERT INTO action_support_assessments(assessment_id,support_key,status,current_fact_refs_json)
								VALUES($1,$2,$3,$4)
								ON CONFLICT (assessment_id, support_key) DO UPDATE SET status=EXCLUDED.status, current_fact_refs_json=EXCLUDED.current_fact_refs_json`,
								assessID, sa.SupportKey, sa.Status, mustJSON(sa.CurrentFactHandles)); err != nil {
								return err
							}
						}
					}
				}
				if _, err = tx.Exec(ctx, `UPDATE action_case_state SET system_state=$2,updated_at=NOW() WHERE case_id=$1`, cid, sys); err != nil {
					return err
				}
				if po.SystemEvent != "" {
					if _, err = tx.Exec(ctx, `INSERT INTO action_system_events(id,case_id,signature_id,event_type,publication_id,payload_json)
						SELECT $1,$2,signature_id,$3,$4,'{}'::jsonb FROM action_cases WHERE id=$2`,
						uuid.New(), cid, po.SystemEvent, pubID); err != nil {
						return err
					}
				}
			}
		}
	}
	for i, g := range in.Plan.Groups {
		gid := uuid.New()
		if _, err := tx.Exec(ctx, `INSERT INTO publication_action_groups(id,publication_id,recommendation_id,action_type,target_kind,family,display_order,display_title,display_summary,presentation_template_key)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			gid, pubID, g.RecommendationRef, g.ActionType, g.TargetKind, g.Family, g.DisplayOrder, g.DisplayTitle, g.DisplaySummary, g.PresentationKey); err != nil {
			return err
		}
		_ = i
		for j, ref := range g.MemberLocalRefs {
			for _, np := range in.Plan.NewProposalIntents {
				if np.LocalRef == ref {
					cid := caseBySig[np.SignatureHash]
					var pid uuid.UUID
					_ = tx.QueryRow(ctx, `SELECT id FROM action_case_proposals WHERE publication_id=$1 AND case_id=$2`, pubID, cid).Scan(&pid)
					if pid != uuid.Nil {
						if _, err := tx.Exec(ctx, `INSERT INTO publication_action_group_members(group_id,case_id,proposal_id,member_order) VALUES($1,$2,$3,$4)`,
							gid, cid, pid, j+1); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func (r *Repository) LoadPreviousActions(sellerID int64, publicationID int64) ([]continuity.PreviousAtomicAction, error) {
	rows, err := r.pool.Query(context.Background(), `
		SELECT c.id::text, p.id::text, s.signature_hash, p.action_type, s.target_kind, s.target_id,
			s.normalized_context_json, s.intent_key, p.parameters_json,
			COALESCE(st.system_state,''), COALESCE(st.user_state,''),
			COALESCE(ss.suppressed_until::text,''), s.seller_account_id
		FROM action_case_proposals p
		JOIN action_cases c ON c.id=p.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		JOIN action_case_state st ON st.case_id=c.id
		JOIN action_signature_state ss ON ss.signature_id=s.id
		WHERE s.seller_account_id=$2
		  AND (
			p.publication_id=$1
			OR (st.system_state='SUSPENDED' AND st.current_proposal_id=p.id)
		  )
		ORDER BY p.display_order`, publicationID, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []continuity.PreviousAtomicAction
	for rows.Next() {
		var a continuity.PreviousAtomicAction
		var ctxRaw, paramRaw []byte
		var seller int64
		if err = rows.Scan(&a.CaseRef, &a.ProposalRef, &a.SignatureHash, &a.ActionType, &a.Target.Kind, &a.Target.ID,
			&ctxRaw, &a.IntentSignature, &paramRaw, &a.SystemState, &a.UserState, &a.SuppressedUntil, &seller); err != nil {
			return nil, err
		}
		if seller != sellerID {
			return nil, fmt.Errorf("continuity: seller mismatch")
		}
		_ = json.Unmarshal(ctxRaw, &a.NormalizedContext)
		_ = json.Unmarshal(paramRaw, &a.Parameters)
		a.PreviousActionRef = a.ProposalRef
		a.PublicationID = publicationID
		a.SelectionReason = "ACTION_CONTINUITY_RECHECK"
		srows, err := r.pool.Query(context.Background(), `SELECT support_key,support_role,claim,fact_refs_json,finding_refs_json,revalidation_json
			FROM action_proposal_supports WHERE proposal_id=$1::uuid`, a.ProposalRef)
		if err != nil {
			return nil, err
		}
		for srows.Next() {
			var sp continuity.PackedSupport
			var facts, finds, revRaw []byte
			if err = srows.Scan(&sp.SupportKey, &sp.Role, &sp.Claim, &facts, &finds, &revRaw); err != nil {
				srows.Close()
				return nil, err
			}
			_ = json.Unmarshal(facts, &sp.FactHandles)
			_ = json.Unmarshal(finds, &sp.FindingRefs)
			_ = json.Unmarshal(revRaw, &sp.Revalidation)
			if sp.Role == "REQUIRED_SUPPORT" {
				a.RequiredSupports = append(a.RequiredSupports, sp)
			} else if sp.Role == "CONTEXT" {
				a.ContextSupports = append(a.ContextSupports, sp)
			} else {
				a.CounterSupports = append(a.CounterSupports, sp)
			}
		}
		srows.Close()
		if a.RequiredSupports == nil {
			a.RequiredSupports = []continuity.PackedSupport{}
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repository) ProcessOutbox(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `SELECT id, COALESCE(payload_json->>'seller_account_id',''), COALESCE(aggregate_id,'') FROM analysis_outbox_events WHERE processed_at IS NULL ORDER BY created_at LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	type item struct {
		id     uuid.UUID
		seller string
		agg    string
	}
	var ids []item
	for rows.Next() {
		var it item
		if err = rows.Scan(&it.id, &it.seller, &it.agg); err != nil {
			return n, err
		}
		ids = append(ids, it)
	}
	rows.Close()
	for _, it := range ids {
		if TestOutboxSellerFilter != 0 && it.seller != strconv.FormatInt(TestOutboxSellerFilter, 10) {
			continue
		}
		if TestOutboxFailNext {
			if TestOutboxFailSellerID != 0 && it.seller != strconv.FormatInt(TestOutboxFailSellerID, 10) {
				// Skip fail injection for other sellers; process normally below.
			} else {
				TestOutboxFailNext = false
				tag, uerr := r.pool.Exec(ctx, `UPDATE analysis_outbox_events SET attempts=attempts+1,last_error=$2 WHERE id=$1 AND processed_at IS NULL`,
					it.id, ErrOutboxConsumer.Error())
				if uerr != nil {
					return n, uerr
				}
				if tag.RowsAffected() == 0 {
					return n, fmt.Errorf("%w: outbox row not pending", ErrOutboxConsumer)
				}
				return n, ErrOutboxConsumer
			}
		}
		tag, err := r.pool.Exec(ctx, `UPDATE analysis_outbox_events SET processed_at=NOW(),attempts=attempts+1,last_error=NULL WHERE id=$1 AND processed_at IS NULL`, it.id)
		if err != nil {
			_, _ = r.pool.Exec(ctx, `UPDATE analysis_outbox_events SET attempts=attempts+1,last_error=$2 WHERE id=$1`, it.id, err.Error())
			return n, err
		}
		if tag.RowsAffected() == 1 {
			TestOutboxDeliveryAttemptCount++
			if TestOutboxProcessedAggregates == nil {
				TestOutboxProcessedAggregates = make(map[string]bool)
			}
			if it.agg != "" && !TestOutboxProcessedAggregates[it.agg] {
				TestOutboxProcessedAggregates[it.agg] = true
				TestOutboxSideEffectCount++
			}
			n++
		}
	}
	return n, nil
}

func (r *Repository) ApplyUserEvent(ctx context.Context, sellerID int64, caseID string, userID *int64, eventType, reason, comment string, now time.Time) error {
	return r.ApplyUserEvents(ctx, sellerID, []string{caseID}, userID, eventType, reason, comment, now)
}

func (r *Repository) CaptureBasePublication(ctx context.Context, runID, sellerID int64) error {
	cur, err := r.CurrentPublicationID(ctx, sellerID)
	if err != nil {
		return err
	}
	if cur == nil {
		return nil
	}
	_, err = r.pool.Exec(ctx, `UPDATE analysis_runs SET base_publication_id=$2 WHERE id=$1 AND base_publication_id IS NULL`, runID, *cur)
	return err
}

func projectionSchemaVersion(raw []byte) string {
	var env struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.SchemaVersion != "" {
		return env.SchemaVersion
	}
	return "v1"
}
