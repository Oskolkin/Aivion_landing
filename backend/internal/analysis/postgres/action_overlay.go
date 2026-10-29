package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actionreconcile"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ActionGroupRow struct {
	ID                    uuid.UUID
	ActionType, Family    string
	DisplayOrder          int
	DisplayTitle, Summary string
	PresentationKey       string
	ActiveMemberCount     int
	TotalMemberCount      int
}

type ActionMemberRow struct {
	GroupID           uuid.UUID
	CaseID            uuid.UUID
	ProposalID        uuid.UUID
	MemberOrder       int
	UserState         string
	SystemState       string
	TargetKind        string
	TargetID          string
	ActionType        string
	Parameters        []byte
	RecommendationRef string
}

func (r *Repository) ListActionGroupsOverlay(ctx context.Context, sellerID, publicationID int64) ([]ActionGroupRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT g.id, g.action_type, g.family, g.display_order, g.display_title, g.display_summary, g.presentation_template_key,
			count(m.id)::int AS total_members,
			count(m.id) FILTER (WHERE st.user_state='OPEN' AND st.system_state='ACTIVE')::int AS active_members
		FROM publication_action_groups g
		JOIN analysis_publications p ON p.id=g.publication_id
		LEFT JOIN publication_action_group_members m ON m.group_id=g.id
		LEFT JOIN action_case_state st ON st.case_id=m.case_id
		WHERE g.publication_id=$1 AND p.seller_account_id=$2
		GROUP BY g.id
		ORDER BY g.display_order, g.id`, publicationID, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActionGroupRow
	for rows.Next() {
		var g ActionGroupRow
		if err := rows.Scan(&g.ID, &g.ActionType, &g.Family, &g.DisplayOrder, &g.DisplayTitle, &g.Summary, &g.PresentationKey, &g.TotalMemberCount, &g.ActiveMemberCount); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *Repository) ListActionGroupMembers(ctx context.Context, sellerID int64, groupIDs []uuid.UUID) ([]ActionMemberRow, error) {
	if len(groupIDs) == 0 {
		return []ActionMemberRow{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT m.group_id, m.case_id, m.proposal_id, m.member_order,
			coalesce(st.user_state,''), coalesce(st.system_state,''),
			s.target_kind, s.target_id, p.action_type, p.parameters_json, coalesce(p.recommendation_id,'')
		FROM publication_action_group_members m
		JOIN publication_action_groups g ON g.id=m.group_id
		JOIN analysis_publications pub ON pub.id=g.publication_id
		JOIN action_cases c ON c.id=m.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		JOIN action_case_proposals p ON p.id=m.proposal_id
		LEFT JOIN action_case_state st ON st.case_id=c.id
		WHERE pub.seller_account_id=$1 AND m.group_id = ANY($2)
		ORDER BY g.display_order, m.member_order, m.id`, sellerID, groupIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActionMemberRow
	for rows.Next() {
		var m ActionMemberRow
		if err := rows.Scan(&m.GroupID, &m.CaseID, &m.ProposalID, &m.MemberOrder, &m.UserState, &m.SystemState, &m.TargetKind, &m.TargetID, &m.ActionType, &m.Parameters, &m.RecommendationRef); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repository) ListUngroupedActiveCases(ctx context.Context, sellerID, publicationID int64) ([]ActionMemberRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, c.id, p.id, 1,
			coalesce(st.user_state,''), coalesce(st.system_state,''),
			s.target_kind, s.target_id, p.action_type, p.parameters_json, coalesce(p.recommendation_id,'')
		FROM action_case_proposals p
		JOIN action_cases c ON c.id=p.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		JOIN action_case_state st ON st.case_id=c.id
		WHERE p.publication_id=$1 AND s.seller_account_id=$2
		  AND st.user_state='OPEN' AND st.system_state='ACTIVE'
		  AND NOT EXISTS (
			SELECT 1 FROM publication_action_group_members m
			JOIN publication_action_groups g ON g.id=m.group_id
			WHERE m.case_id=c.id AND g.publication_id=$1
		  )
		ORDER BY p.id`, publicationID, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActionMemberRow
	for rows.Next() {
		var m ActionMemberRow
		if err := rows.Scan(&m.GroupID, &m.CaseID, &m.ProposalID, &m.MemberOrder, &m.UserState, &m.SystemState, &m.TargetKind, &m.TargetID, &m.ActionType, &m.Parameters, &m.RecommendationRef); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repository) ApplyUserEvents(ctx context.Context, sellerID int64, caseIDs []string, userID *int64, eventType, reason, comment string, now time.Time) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, SellerAdvisoryLockKey(sellerID)); err != nil {
			return err
		}
		for _, caseID := range caseIDs {
			if err := applyUserEventTx(ctx, tx, sellerID, caseID, userID, eventType, reason, comment, now); err != nil {
				return err
			}
		}
		return nil
	})
}

func applyUserEventTx(ctx context.Context, tx pgx.Tx, sellerID int64, caseID string, userID *int64, eventType, reason, comment string, now time.Time) error {
	var gotSeller int64
	var userState string
	err := tx.QueryRow(ctx, `SELECT s.seller_account_id, coalesce(st.user_state,'')
		FROM action_cases c
		JOIN action_signatures s ON s.id=c.signature_id
		LEFT JOIN action_case_state st ON st.case_id=c.id
		WHERE c.id=$1::uuid`, caseID).Scan(&gotSeller, &userState)
	if err != nil {
		return err
	}
	if gotSeller != sellerID {
		return fmt.Errorf("seller isolation")
	}
	want := "OPEN"
	switch eventType {
	case "DONE":
		want = "DONE"
	case "DISMISSED":
		want = "DISMISSED"
	case "REOPENED":
		want = "OPEN"
	}
	if userState == want {
		return nil
	}
	id := uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO action_user_events(id,case_id,seller_account_id,user_id,event_type,reason_code,comment) VALUES($1,$2,$3,$4,$5,$6,$7)`,
		id, caseID, sellerID, userID, eventType, reason, comment); err != nil {
		return err
	}
	if eventType == "DISMISSED" {
		until := now.UTC().AddDate(0, 0, actionreconcile.UserDismissCalendarDays)
		if _, err = tx.Exec(ctx, `UPDATE action_signature_state SET suppressed_until=$2,suppression_source='USER_DISMISSED',updated_at=NOW()
			WHERE signature_id=(SELECT signature_id FROM action_cases WHERE id=$1::uuid)`, caseID, until); err != nil {
			return err
		}
	}
	if eventType == "REOPENED" {
		if _, err = tx.Exec(ctx, `UPDATE action_signature_state SET suppressed_until=NULL,suppression_source=NULL,updated_at=NOW()
			WHERE signature_id=(SELECT signature_id FROM action_cases WHERE id=$1::uuid)`, caseID); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE action_case_state SET user_state=$2,updated_at=NOW() WHERE case_id=$1::uuid`, caseID, want); err != nil {
		return err
	}
	oid := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO analysis_outbox_events(id,event_type,aggregate_type,aggregate_id,payload_json)
		VALUES($1,'action.user_state_changed','action_case',$2,$3)`,
		oid, caseID, mustJSON(map[string]any{"seller_account_id": sellerID, "case_id": caseID, "event": eventType}))
	return err
}
