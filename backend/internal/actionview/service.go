package actionview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo *analysispg.Repository
}

func NewService(repo *analysispg.Repository) *Service { return &Service{repo: repo} }

func (s *Service) List(ctx context.Context, sellerID int64, f ListFilter) (ListResponse, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	f.Status = normalizeStatus(f.Status)
	f.Query = strings.TrimSpace(f.Query)
	pub, err := s.repo.GetCurrentPublication(ctx, sellerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ListResponse{State: "PRE_ANALYSIS", Groups: []GroupView{}, History: []CaseHistoryView{}}, nil
	}
	if err != nil {
		return ListResponse{}, err
	}
	if f.Cursor != "" {
		cur, err := decodeCursor(f.Cursor)
		if err != nil {
			return ListResponse{}, err
		}
		if !cur.matches(sellerID, f) {
			return ListResponse{}, fmt.Errorf("%w: binding", ErrCursorMismatch)
		}
	}
	pid := strconv.FormatInt(pub.ID, 10)
	if f.Status == "active" {
		groups, err := s.listActive(ctx, sellerID, pub.ID, f)
		if err != nil {
			return ListResponse{}, err
		}
		sortGroupViews(groups)
		page, next, more, err := pageGroups(groups, sellerID, f)
		if err != nil {
			return ListResponse{}, err
		}
		return ListResponse{PublicationID: pid, State: "READY", Groups: page, NextCursor: next, HasMore: more}, nil
	}
	hist, err := s.listHistory(ctx, sellerID, f.Status, f)
	if err != nil {
		return ListResponse{}, err
	}
	sortHistoryViews(hist)
	page, next, more, err := pageHistory(hist, sellerID, f)
	if err != nil {
		return ListResponse{}, err
	}
	return ListResponse{PublicationID: pid, State: "READY", History: page, NextCursor: next, HasMore: more}, nil
}

func (s *Service) listActive(ctx context.Context, sellerID, pubID int64, f ListFilter) ([]GroupView, error) {
	rows, err := s.repo.ListActionGroupsOverlay(ctx, sellerID, pubID)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	var selected []analysispg.ActionGroupRow
	q := strings.ToLower(strings.TrimSpace(f.Query))
	for _, g := range rows {
		if g.ActiveMemberCount == 0 {
			continue
		}
		if f.Family != "" && g.Family != f.Family {
			continue
		}
		selected = append(selected, g)
		ids = append(ids, g.ID)
	}
	members, err := s.repo.ListActionGroupMembers(ctx, sellerID, ids)
	if err != nil {
		return nil, err
	}
	by := map[uuid.UUID][]MemberView{}
	for _, m := range members {
		by[m.GroupID] = append(by[m.GroupID], memberView(m))
	}
	out := make([]GroupView, 0, len(selected))
	for _, g := range selected {
		ms := by[g.ID]
		if ms == nil {
			ms = []MemberView{}
		}
		if q != "" && !groupMatchesQuery(g, ms, q) {
			continue
		}
		out = append(out, GroupView{
			ActionViewID: EncodeGroup(g.ID.String()), DisplayOrder: g.DisplayOrder,
			DisplayTitle: g.DisplayTitle, DisplaySummary: g.Summary, ActionType: g.ActionType,
			Family: g.Family, PresentationTemplateKey: g.PresentationKey,
			ActiveMemberCount: g.ActiveMemberCount, Members: ms, UserStateSummary: "ACTIVE",
		})
	}
	ungrouped, err := s.repo.ListUngroupedActiveCases(ctx, sellerID, pubID)
	if err != nil {
		return nil, err
	}
	fam := familyByType()
	for i, m := range ungrouped {
		mv := memberView(m)
		if q != "" && !groupMatchesQuery(analysispg.ActionGroupRow{DisplayTitle: m.ActionType, ActionType: m.ActionType, Family: fam[m.ActionType]}, []MemberView{mv}, q) {
			continue
		}
		if f.Family != "" && fam[m.ActionType] != f.Family {
			continue
		}
		out = append(out, GroupView{
			ActionViewID: EncodeCase(m.CaseID.String()), DisplayOrder: 1000 + i,
			DisplayTitle: m.ActionType, DisplaySummary: m.TargetKind + " " + m.TargetID,
			ActionType: m.ActionType, Family: fam[m.ActionType], PresentationTemplateKey: m.ActionType,
			ActiveMemberCount: 1, Members: []MemberView{mv}, UserStateSummary: "ACTIVE",
		})
	}
	return out, nil
}

func familyByType() map[string]string {
	out := map[string]string{}
	for _, d := range actioncatalog.MustHave19() {
		out[d.ActionTypeKey] = d.ActionFamily
	}
	return out
}

func groupMatchesQuery(g analysispg.ActionGroupRow, members []MemberView, q string) bool {
	hay := strings.ToLower(g.DisplayTitle + " " + g.Summary + " " + g.ActionType + " " + g.Family)
	if strings.Contains(hay, q) {
		return true
	}
	for _, m := range members {
		if strings.Contains(strings.ToLower(m.TargetID+" "+m.TargetKind+" "+m.ActionType), q) {
			return true
		}
	}
	return false
}

func memberView(m analysispg.ActionMemberRow) MemberView {
	var params map[string]any
	_ = json.Unmarshal(m.Parameters, &params)
	return MemberView{
		CaseID: m.CaseID.String(), MemberOrder: m.MemberOrder, ActionType: m.ActionType,
		TargetKind: m.TargetKind, TargetID: m.TargetID, UserState: m.UserState, SystemState: m.SystemState,
		Active: m.UserState == "OPEN" && m.SystemState == "ACTIVE", Parameters: params,
		RecommendationRef: m.RecommendationRef,
	}
}

func (s *Service) listHistory(ctx context.Context, sellerID int64, status string, f ListFilter) ([]CaseHistoryView, error) {
	mode := "DONE"
	if status == "dismissed" || status == "irrelevant" {
		mode = "IRRELEVANT"
	}
	rows, err := s.repo.Pool().Query(ctx, historySQL(mode), sellerID, f.Family, "%"+strings.ToLower(f.Query)+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CaseHistoryView
	for rows.Next() {
		var caseID, actionType, family, targetKind, targetID, userState, sysState, title, summary, tmpl, rec string
		var params []byte
		if err := rows.Scan(&caseID, &actionType, &family, &targetKind, &targetID, &userState, &sysState, &title, &summary, &tmpl, &params, &rec); err != nil {
			return nil, err
		}
		var p map[string]any
		_ = json.Unmarshal(params, &p)
		out = append(out, CaseHistoryView{
			ActionViewID: EncodeCase(caseID), CaseID: caseID, DisplayTitle: title, DisplaySummary: summary,
			ActionType: actionType, Family: family, PresentationTemplateKey: tmpl,
			UserFacingMode: userFacingMode(userState, sysState), TargetKind: targetKind, TargetID: targetID,
			Parameters: p, RecommendationRef: rec,
		})
	}
	if out == nil {
		out = []CaseHistoryView{}
	}
	return out, rows.Err()
}

func historySQL(mode string) string {
	where := `st.user_state='DONE'`
	if mode == "IRRELEVANT" {
		where = `(st.user_state='DISMISSED' OR (st.system_state <> 'ACTIVE' AND st.user_state <> 'DONE' AND st.user_state <> 'OPEN'))`
	}
	return `
		SELECT c.id::text, p.action_type, coalesce(g.family, ''), s.target_kind, s.target_id,
			coalesce(st.user_state,''), coalesce(st.system_state,''),
			coalesce(g.display_title, p.action_type), coalesce(g.display_summary,''), coalesce(g.presentation_template_key, p.action_type),
			p.parameters_json, coalesce(p.recommendation_id,'')
		FROM action_cases c
		JOIN action_signatures s ON s.id=c.signature_id
		JOIN action_case_state st ON st.case_id=c.id
		JOIN action_case_proposals p ON p.id=st.current_proposal_id
		LEFT JOIN LATERAL (
			SELECT g.display_title, g.display_summary, g.presentation_template_key, g.family
			FROM publication_action_group_members m
			JOIN publication_action_groups g ON g.id=m.group_id
			WHERE m.case_id=c.id
			ORDER BY g.display_order LIMIT 1
		) g ON true
		WHERE s.seller_account_id=$1 AND ` + where + `
		  AND ($2='' OR coalesce(g.family, '')=$2)
		  AND ($3='%%' OR lower(coalesce(g.display_title,'')||' '||s.target_id||' '||p.action_type) LIKE $3)
		ORDER BY c.id
		LIMIT 2000`
}

func userFacingMode(user, sys string) string {
	if user == "DONE" {
		return "DONE"
	}
	if user == "DISMISSED" {
		return "IRRELEVANT"
	}
	if sys != "ACTIVE" {
		return "IRRELEVANT"
	}
	return "ACTIVE"
}

func (s *Service) Get(ctx context.Context, sellerID int64, viewID string) (DetailResponse, error) {
	ref, err := Decode(viewID)
	if err != nil {
		return DetailResponse{}, err
	}
	if ref.Kind == "g" {
		gid, err := uuid.Parse(ref.ID)
		if err != nil {
			return DetailResponse{}, err
		}
		pub, err := s.repo.GetCurrentPublication(ctx, sellerID)
		if err != nil {
			return DetailResponse{}, err
		}
		groups, err := s.listActive(ctx, sellerID, pub.ID, ListFilter{})
		if err != nil {
			return DetailResponse{}, err
		}
		enc := EncodeGroup(gid.String())
		for i := range groups {
			if groups[i].ActionViewID == enc {
				return DetailResponse{Kind: "group", Group: &groups[i]}, nil
			}
		}
		return DetailResponse{}, pgx.ErrNoRows
	}
	enc := EncodeCase(ref.ID)
	pub, err := s.repo.GetCurrentPublication(ctx, sellerID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return DetailResponse{}, err
	}
	if err == nil {
		groups, err := s.listActive(ctx, sellerID, pub.ID, ListFilter{})
		if err != nil {
			return DetailResponse{}, err
		}
		for i := range groups {
			if groups[i].ActionViewID == enc {
				return DetailResponse{Kind: "group", Group: &groups[i]}, nil
			}
			for _, m := range groups[i].Members {
				if m.CaseID == ref.ID {
					return DetailResponse{Kind: "group", Group: &groups[i]}, nil
				}
			}
		}
	}
	hist, err := s.listHistory(ctx, sellerID, "done", ListFilter{Query: "", Limit: 500})
	if err != nil {
		return DetailResponse{}, err
	}
	irr, err := s.listHistory(ctx, sellerID, "irrelevant", ListFilter{Query: "", Limit: 500})
	if err != nil {
		return DetailResponse{}, err
	}
	for i := range hist {
		if hist[i].ActionViewID == enc {
			return DetailResponse{Kind: "history", History: &hist[i]}, nil
		}
	}
	for i := range irr {
		if irr[i].ActionViewID == enc {
			return DetailResponse{Kind: "history", History: &irr[i]}, nil
		}
	}
	return DetailResponse{}, pgx.ErrNoRows
}

func (s *Service) OriginPublication(ctx context.Context, sellerID int64, viewID string) (int64, error) {
	ref, err := Decode(viewID)
	if err != nil {
		return 0, err
	}
	if ref.Kind == "g" {
		gid, err := uuid.Parse(ref.ID)
		if err != nil {
			return 0, err
		}
		var pubID int64
		err = s.repo.Pool().QueryRow(ctx, `
			SELECT g.publication_id FROM publication_action_groups g
			JOIN analysis_publications p ON p.id=g.publication_id
			WHERE g.id=$1 AND p.seller_account_id=$2`, gid, sellerID).Scan(&pubID)
		return pubID, err
	}
	cid, err := uuid.Parse(ref.ID)
	if err != nil {
		return 0, err
	}
	var pubID int64
	err = s.repo.Pool().QueryRow(ctx, `
		SELECT p.publication_id
		FROM action_cases c
		JOIN action_signatures s ON s.id=c.signature_id
		JOIN action_case_state st ON st.case_id=c.id
		JOIN action_case_proposals p ON p.id=st.current_proposal_id
		WHERE c.id=$1 AND s.seller_account_id=$2`, cid, sellerID).Scan(&pubID)
	return pubID, err
}

func (s *Service) GetBoundCases(ctx context.Context, sellerID, originPublicationID int64, caseIDs []string) ([]BoundCaseContext, error) {
	if len(caseIDs) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, 0, len(caseIDs))
	for _, raw := range caseIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.repo.Pool().Query(ctx, `
		SELECT c.id::text, s.target_kind, s.target_id,
			coalesce(cp.action_type, op.action_type, ''),
			coalesce(st.user_state,''), coalesce(st.system_state,''),
			op.parameters_json, coalesce(op.recommendation_id,''), coalesce(op.publication_id,0),
			cp.parameters_json, coalesce(cp.recommendation_id,''), coalesce(cp.publication_id,0)
		FROM action_cases c
		JOIN action_signatures s ON s.id=c.signature_id
		LEFT JOIN action_case_state st ON st.case_id=c.id
		LEFT JOIN action_case_proposals op ON op.case_id=c.id AND op.publication_id=$3
		LEFT JOIN action_case_proposals cp ON cp.id=st.current_proposal_id
		WHERE s.seller_account_id=$1 AND c.id = ANY($2)`, sellerID, ids, originPublicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BoundCaseContext
	for rows.Next() {
		var b BoundCaseContext
		var originJSON, currentJSON []byte
		if err := rows.Scan(&b.CaseID, &b.TargetKind, &b.TargetID, &b.ActionType, &b.UserState, &b.SystemState,
			&originJSON, &b.OriginRecommendationRef, &b.OriginPublicationID,
			&currentJSON, &b.CurrentRecommendationRef, &b.CurrentPublicationID); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(originJSON, &b.OriginParameters)
		_ = json.Unmarshal(currentJSON, &b.CurrentParameters)
		b.Active = b.UserState == "OPEN" && b.SystemState == "ACTIVE"
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Service) Mutate(ctx context.Context, sellerID int64, userID *int64, caseID, eventType string) error {
	return s.repo.ApplyUserEvent(ctx, sellerID, caseID, userID, eventType, "ui", "", time.Now().UTC())
}

func (s *Service) Bulk(ctx context.Context, sellerID int64, userID *int64, caseIDs []string, eventType string) error {
	return s.repo.ApplyUserEvents(ctx, sellerID, caseIDs, userID, eventType, "ui", "", time.Now().UTC())
}

func CanonicalActionTypes() []string {
	out := make([]string, 0, 19)
	for _, d := range actioncatalog.MustHave19() {
		out = append(out, d.ActionTypeKey)
	}
	return out
}

func pageGroups(all []GroupView, sellerID int64, f ListFilter) ([]GroupView, string, bool, error) {
	start := 0
	if f.Cursor != "" {
		cur, err := decodeCursor(f.Cursor)
		if err != nil {
			return nil, "", false, err
		}
		if !cur.matches(sellerID, f) {
			return nil, "", false, fmt.Errorf("%w: binding", ErrCursorMismatch)
		}
		keys := make([]orderedView, len(all))
		for i, g := range all {
			keys[i] = sortKeys(g.DisplayOrder, g.ActionViewID)
		}
		start = afterCursor(keys, cur)
	}
	end := start + f.Limit
	more := end < len(all)
	if end > len(all) {
		end = len(all)
	}
	page := all[start:end]
	next := ""
	if more && len(page) > 0 {
		last := page[len(page)-1]
		next = encodeCursor(pageCursor{
			Seller: sellerID, Status: normalizeStatus(f.Status), Family: f.Family, Query: f.Query,
			Sort: sortAivion, Order: last.DisplayOrder, ID: last.ActionViewID,
		})
	}
	if page == nil {
		page = []GroupView{}
	}
	return page, next, more, nil
}

func pageHistory(all []CaseHistoryView, sellerID int64, f ListFilter) ([]CaseHistoryView, string, bool, error) {
	start := 0
	if f.Cursor != "" {
		cur, err := decodeCursor(f.Cursor)
		if err != nil {
			return nil, "", false, err
		}
		if !cur.matches(sellerID, f) {
			return nil, "", false, fmt.Errorf("%w: binding", ErrCursorMismatch)
		}
		keys := make([]orderedView, len(all))
		for i, h := range all {
			keys[i] = sortKeys(0, h.ActionViewID)
		}
		start = afterCursor(keys, cur)
	}
	end := start + f.Limit
	more := end < len(all)
	if end > len(all) {
		end = len(all)
	}
	page := all[start:end]
	next := ""
	if more && len(page) > 0 {
		last := page[len(page)-1]
		next = encodeCursor(pageCursor{
			Seller: sellerID, Status: normalizeStatus(f.Status), Family: f.Family, Query: f.Query,
			Sort: sortAivion, Order: 0, ID: last.ActionViewID,
		})
	}
	if page == nil {
		page = []CaseHistoryView{}
	}
	return page, next, more, nil
}
