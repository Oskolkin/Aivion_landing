package actions

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type mockRepo struct {
	latestRun     *ActionGenerationRunRecord
	latestMetric  *time.Time
	items         []ActionItemRecord
	getByKey      func(sellerAccountID int64, actionKey string) (*ActionItemRecord, error)
	checkedKeys   []string
	uncheckedKeys []string
	dismissedKeys []string
	setCheckedErr error
	setOpenErr    error
	setDismissErr error
	listCalls     int
}

func (m *mockRepo) GetLatestRun(ctx context.Context, sellerAccountID int64) (*ActionGenerationRunRecord, error) {
	return m.latestRun, nil
}

func (m *mockRepo) GetLatestPublishableRun(ctx context.Context, sellerAccountID int64) (*ActionGenerationRunRecord, error) {
	if m.latestRun != nil && IsPublishableActionFinalStatus(m.latestRun.FinalStatus) {
		return m.latestRun, nil
	}
	if len(m.items) == 0 {
		return nil, nil
	}
	runID := int64(1)
	if m.latestRun != nil && m.latestRun.ID > 0 {
		runID = m.latestRun.ID
	}
	for i := range m.items {
		if m.items[i].RunID == nil {
			id := runID
			m.items[i].RunID = &id
		}
	}
	metric := m.latestMetric
	if metric == nil && m.items[0].MetricDate != nil {
		metric = m.items[0].MetricDate
	}
	return &ActionGenerationRunRecord{
		ID:              runID,
		SellerAccountID: sellerAccountID,
		MetricDate:      metric,
		Status:          "completed",
		FinalStatus:     GenerationFinalStatusCompleted,
	}, nil
}

func (m *mockRepo) GetPublishableRunByPipelineID(ctx context.Context, sellerAccountID, pipelineRunID int64) (*ActionGenerationRunRecord, error) {
	run, err := m.GetLatestPublishableRun(ctx, sellerAccountID)
	if err != nil || run == nil {
		return run, err
	}
	if pipelineRunID > 0 && run.AiPipelineRunID > 0 && run.AiPipelineRunID != pipelineRunID {
		return nil, nil
	}
	if pipelineRunID > 0 {
		run.AiPipelineRunID = pipelineRunID
	}
	return run, nil
}

func (m *mockRepo) ListActionsByRunID(ctx context.Context, runID int64, params ListActionsParams) ([]ActionItemRecord, error) {
	out := make([]ActionItemRecord, 0)
	for _, item := range m.items {
		if item.RunID == nil || *item.RunID != runID {
			continue
		}
		if params.Category != nil && item.Category != *params.Category {
			continue
		}
		if params.Priority != nil && item.Priority != *params.Priority {
			continue
		}
		if params.Status != "" && params.Status != "all" && string(item.Status) != params.Status {
			continue
		}
		out = append(out, item)
	}
	if params.Offset >= len(out) {
		return []ActionItemRecord{}, nil
	}
	end := len(out)
	if params.Limit > 0 && params.Offset+params.Limit < end {
		end = params.Offset + params.Limit
	}
	return out[params.Offset:end], nil
}

func (m *mockRepo) GetLatestMetricDate(ctx context.Context, sellerAccountID int64) (*time.Time, error) {
	return m.latestMetric, nil
}

func (m *mockRepo) ListActions(ctx context.Context, params ListActionsParams) ([]ActionItemRecord, error) {
	m.listCalls++
	out := make([]ActionItemRecord, 0)
	for _, item := range m.items {
		if item.SellerAccountID != params.SellerAccountID {
			continue
		}
		if params.MetricDate != nil && item.MetricDate != nil {
			if !sameDate(*params.MetricDate, *item.MetricDate) {
				continue
			}
		}
		if params.Category != nil && item.Category != *params.Category {
			continue
		}
		if params.Priority != nil && item.Priority != *params.Priority {
			continue
		}
		if params.Status != "" && params.Status != "all" && string(item.Status) != params.Status {
			continue
		}
		out = append(out, item)
	}
	if params.Offset >= len(out) {
		return []ActionItemRecord{}, nil
	}
	end := len(out)
	if params.Limit > 0 && params.Offset+params.Limit < end {
		end = params.Offset + params.Limit
	}
	return out[params.Offset:end], nil
}

func (m *mockRepo) GetActionByKey(ctx context.Context, sellerAccountID int64, actionKey string) (*ActionItemRecord, error) {
	if m.getByKey != nil {
		return m.getByKey(sellerAccountID, actionKey)
	}
	for i := range m.items {
		if m.items[i].SellerAccountID == sellerAccountID && m.items[i].ActionKey == actionKey {
			item := m.items[i]
			return &item, nil
		}
	}
	return nil, ErrActionNotFound
}

func (m *mockRepo) GetActionByID(ctx context.Context, sellerAccountID int64, actionID int64) (*ActionItemRecord, error) {
	for i := range m.items {
		if m.items[i].SellerAccountID == sellerAccountID && m.items[i].ID == actionID {
			item := m.items[i]
			return &item, nil
		}
	}
	return nil, ErrActionNotFound
}

func (m *mockRepo) GetHistoricalActionByID(ctx context.Context, sellerAccountID int64, actionID int64) (*ActionItemRecord, error) {
	return m.GetActionByID(ctx, sellerAccountID, actionID)
}

func (m *mockRepo) GetActionEntityGroup(ctx context.Context, sellerAccountID, actionsRunID int64, groupKey string) (*ActionEntityGroup, error) {
	return nil, ErrEntityGroupNotFound
}

func (m *mockRepo) GetGenerationRunByID(ctx context.Context, runID int64) (*ActionGenerationRunRecord, error) {
	return nil, ErrActionNotFound
}

func (m *mockRepo) SetActionChecked(ctx context.Context, sellerAccountID int64, actionKey string) error {
	if m.setCheckedErr != nil {
		return m.setCheckedErr
	}
	if _, err := m.GetActionByKey(ctx, sellerAccountID, actionKey); err != nil {
		return err
	}
	m.checkedKeys = append(m.checkedKeys, actionKey)
	for i := range m.items {
		if m.items[i].SellerAccountID == sellerAccountID && m.items[i].ActionKey == actionKey {
			now := time.Now().UTC()
			m.items[i].Status = ActionStatusChecked
			m.items[i].CheckedAt = &now
		}
	}
	return nil
}

func (m *mockRepo) SetActionOpen(ctx context.Context, sellerAccountID int64, actionKey string) error {
	if m.setOpenErr != nil {
		return m.setOpenErr
	}
	if _, err := m.GetActionByKey(ctx, sellerAccountID, actionKey); err != nil {
		return err
	}
	m.uncheckedKeys = append(m.uncheckedKeys, actionKey)
	for i := range m.items {
		if m.items[i].SellerAccountID == sellerAccountID && m.items[i].ActionKey == actionKey {
			m.items[i].Status = ActionStatusOpen
			m.items[i].CheckedAt = nil
		}
	}
	return nil
}

func (m *mockRepo) SetActionDismissed(ctx context.Context, sellerAccountID int64, actionKey string) error {
	if m.setDismissErr != nil {
		return m.setDismissErr
	}
	if _, err := m.GetActionByKey(ctx, sellerAccountID, actionKey); err != nil {
		return err
	}
	m.dismissedKeys = append(m.dismissedKeys, actionKey)
	for i := range m.items {
		if m.items[i].SellerAccountID == sellerAccountID && m.items[i].ActionKey == actionKey {
			m.items[i].Status = ActionStatusDismissed
			now := time.Now().UTC()
			m.items[i].CheckedAt = &now
		}
	}
	return nil
}

func sameDate(a, b time.Time) bool {
	return a.Format("2006-01-02") == b.Format("2006-01-02")
}

func sampleActionRecord(id int64, sellerID int64, key string, category ActionCategory, priority ActionPriority, status ActionStatus) ActionItemRecord {
	now := time.Now().UTC()
	metricDate := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	evidence, _ := json.Marshal([]ActionEvidence{{Kind: "metric", Label: "stock", Value: "3"}})
	ctas, _ := json.Marshal([]ActionCTA{{Type: "link", Label: "Open"}})
	return ActionItemRecord{
		ID:              id,
		SellerAccountID: sellerID,
		ActionKey:       key,
		Type:            ActionTypeStockReplenishment,
		Category:        category,
		Priority:        priority,
		Status:          status,
		Title:           "Test action",
		Reason:          "Because",
		NextStep:        "Do it",
		EvidenceJSON:    evidence,
		CTAJSON:         ctas,
		MetricDate:      &metricDate,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func TestServiceListActions_EmptyResponse(t *testing.T) {
	svc := NewService(&mockRepo{})
	resp, err := svc.ListActions(context.Background(), 1, ListActionsFilters{
		Status:     string(ActionStatusOpen),
		MetricDate: "latest",
		Limit:      50,
	})
	if err != nil {
		t.Fatalf("ListActions: %v", err)
	}
	if resp.Status != string(ActionResponseStatusNotGenerated) {
		t.Fatalf("status=%q want not_generated", resp.Status)
	}
	if len(resp.Actions) != 0 {
		t.Fatalf("expected empty actions")
	}
	if resp.Counters.TotalOpen != 0 || resp.Counters.Checked != 0 {
		t.Fatalf("expected zero counters, got %+v", resp.Counters)
	}
	if len(resp.DataQualityNotes) != 1 || resp.DataQualityNotes[0] != notGeneratedActionsNote {
		t.Fatalf("unexpected data_quality_notes: %#v", resp.DataQualityNotes)
	}
}

func TestServiceListActions_Counters(t *testing.T) {
	metricDate := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	repo := &mockRepo{
		latestMetric: &metricDate,
		items: []ActionItemRecord{
			sampleActionRecord(1, 1, "a1", ActionCategoryStock, ActionPriorityCritical, ActionStatusOpen),
			sampleActionRecord(2, 1, "a2", ActionCategoryAdvertising, ActionPriorityImportant, ActionStatusOpen),
			sampleActionRecord(3, 1, "a3", ActionCategoryStock, ActionPriorityWatch, ActionStatusChecked),
		},
	}
	svc := NewService(repo)
	resp, err := svc.ListActions(context.Background(), 1, ListActionsFilters{
		Status:     string(ActionStatusOpen),
		MetricDate: "latest",
		Limit:      50,
	})
	if err != nil {
		t.Fatalf("ListActions: %v", err)
	}
	if resp.Counters.TotalOpen != 2 {
		t.Fatalf("total_open=%d want 2", resp.Counters.TotalOpen)
	}
	if resp.Counters.Critical != 1 || resp.Counters.Important != 1 || resp.Counters.Watch != 0 {
		t.Fatalf("priority counters mismatch: %+v", resp.Counters)
	}
	if resp.Counters.Stock != 1 || resp.Counters.Advertising != 1 {
		t.Fatalf("category counters mismatch: %+v", resp.Counters)
	}
	if resp.Counters.Checked != 1 {
		t.Fatalf("checked=%d want 1", resp.Counters.Checked)
	}
	if len(resp.Actions) != 2 {
		t.Fatalf("expected 2 open actions in list, got %d", len(resp.Actions))
	}
}

func TestServiceListActions_CountersGlobalWithPriorityFilter(t *testing.T) {
	metricDate := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	critical := ActionPriorityCritical
	repo := &mockRepo{
		latestMetric: &metricDate,
		items: []ActionItemRecord{
			sampleActionRecord(1, 1, "a1", ActionCategoryStock, ActionPriorityCritical, ActionStatusOpen),
			sampleActionRecord(2, 1, "a2", ActionCategoryAdvertising, ActionPriorityImportant, ActionStatusOpen),
			sampleActionRecord(3, 1, "a3", ActionCategoryStock, ActionPriorityWatch, ActionStatusOpen),
		},
	}
	svc := NewService(repo)
	resp, err := svc.ListActions(context.Background(), 1, ListActionsFilters{
		Status:     string(ActionStatusOpen),
		Priority:   &critical,
		MetricDate: "latest",
		Limit:      50,
	})
	if err != nil {
		t.Fatalf("ListActions: %v", err)
	}
	if resp.Counters.Critical != 1 || resp.Counters.Important != 1 || resp.Counters.Watch != 1 {
		t.Fatalf("expected global priority counters, got %+v", resp.Counters)
	}
	if len(resp.Actions) != 1 {
		t.Fatalf("expected filtered list size 1, got %d", len(resp.Actions))
	}
}

func TestServiceGetAction_FoundAndSellerIsolation(t *testing.T) {
	repo := &mockRepo{
		items: []ActionItemRecord{
			sampleActionRecord(1, 1, "seller1.action", ActionCategoryStock, ActionPriorityCritical, ActionStatusOpen),
			sampleActionRecord(2, 2, "seller2.action", ActionCategoryStock, ActionPriorityCritical, ActionStatusOpen),
		},
	}
	svc := NewService(repo)

	item, err := svc.GetAction(context.Background(), 1, "seller1.action")
	if err != nil {
		t.Fatalf("GetAction: %v", err)
	}
	if item.ActionKey != "seller1.action" {
		t.Fatalf("unexpected action key %q", item.ActionKey)
	}

	_, err = svc.GetAction(context.Background(), 1, "seller2.action")
	if err != ErrActionNotFound {
		t.Fatalf("expected ErrActionNotFound, got %v", err)
	}
}

func TestServiceDismissAction_ItemVisibleInDismissedFilter(t *testing.T) {
	metricDate := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	repo := &mockRepo{
		latestMetric: &metricDate,
		items: []ActionItemRecord{
			sampleActionRecord(1, 1, "a1", ActionCategoryStock, ActionPriorityCritical, ActionStatusOpen),
		},
	}
	svc := NewService(repo)
	if err := svc.DismissAction(context.Background(), 1, "a1"); err != nil {
		t.Fatalf("DismissAction: %v", err)
	}
	resp, err := svc.ListActions(context.Background(), 1, ListActionsFilters{
		Status:     string(ActionStatusDismissed),
		MetricDate: "latest",
		Limit:      50,
	})
	if err != nil {
		t.Fatalf("ListActions: %v", err)
	}
	if len(resp.Actions) != 1 || resp.Actions[0].ActionKey != "a1" {
		t.Fatalf("dismissed filter must include action a1, got %+v", resp.Actions)
	}
	if resp.Counters.Dismissed != 1 {
		t.Fatalf("dismissed counter=%d want 1", resp.Counters.Dismissed)
	}
}

func TestServiceCheckAndUncheckAction(t *testing.T) {
	repo := &mockRepo{
		items: []ActionItemRecord{
			sampleActionRecord(1, 1, "check.me", ActionCategoryStock, ActionPriorityCritical, ActionStatusOpen),
		},
	}
	svc := NewService(repo)

	if err := svc.CheckAction(context.Background(), 1, "check.me"); err != nil {
		t.Fatalf("CheckAction: %v", err)
	}
	item, err := svc.GetAction(context.Background(), 1, "check.me")
	if err != nil {
		t.Fatalf("GetAction after check: %v", err)
	}
	if item.Status != ActionStatusChecked {
		t.Fatalf("status=%q want checked", item.Status)
	}

	if err := svc.UncheckAction(context.Background(), 1, "check.me"); err != nil {
		t.Fatalf("UncheckAction: %v", err)
	}
	item, err = svc.GetAction(context.Background(), 1, "check.me")
	if err != nil {
		t.Fatalf("GetAction after uncheck: %v", err)
	}
	if item.Status != ActionStatusOpen {
		t.Fatalf("status=%q want open", item.Status)
	}
}

func TestServiceCheckAction_NotFoundDoesNotMutate(t *testing.T) {
	repo := &mockRepo{
		items: []ActionItemRecord{
			sampleActionRecord(1, 1, "exists", ActionCategoryStock, ActionPriorityCritical, ActionStatusOpen),
		},
	}
	svc := NewService(repo)

	err := svc.CheckAction(context.Background(), 1, "missing")
	if err != ErrActionNotFound {
		t.Fatalf("expected ErrActionNotFound, got %v", err)
	}
	if len(repo.checkedKeys) != 0 {
		t.Fatalf("unexpected checked keys: %v", repo.checkedKeys)
	}
}

func TestValidateActionKey(t *testing.T) {
	cases := []struct {
		key   string
		valid bool
	}{
		{"", false},
		{"valid.key-1:abc", true},
		{"has/slash", false},
		{string(make([]byte, 201)), false},
		{"bad space", false},
	}
	for _, tc := range cases {
		err := ValidateActionKey(tc.key)
		if tc.valid && err != nil {
			t.Fatalf("key %q should be valid: %v", tc.key, err)
		}
		if !tc.valid && err == nil {
			t.Fatalf("key %q should be invalid", tc.key)
		}
	}
}

func TestListTopOpenActionsForDashboard_ExcludesCheckedAndSortsPriority(t *testing.T) {
	repo := &mockRepo{
		items: []ActionItemRecord{
			sampleActionRecord(1, 1, "watch-1", ActionCategoryStock, ActionPriorityWatch, ActionStatusOpen),
			sampleActionRecord(2, 1, "important-1", ActionCategoryStock, ActionPriorityImportant, ActionStatusOpen),
			sampleActionRecord(3, 1, "critical-1", ActionCategoryStock, ActionPriorityCritical, ActionStatusOpen),
			sampleActionRecord(4, 1, "checked-critical", ActionCategoryStock, ActionPriorityCritical, ActionStatusChecked),
		},
	}
	svc := NewService(repo)
	out, err := svc.ListTopOpenActionsForDashboard(context.Background(), 1, nil, 3)
	if err != nil {
		t.Fatalf("ListTopOpenActionsForDashboard: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(out))
	}
	if out[0].Priority != ActionPriorityCritical || out[1].Priority != ActionPriorityImportant || out[2].Priority != ActionPriorityWatch {
		t.Fatalf("unexpected priority order: %+v", []ActionPriority{out[0].Priority, out[1].Priority, out[2].Priority})
	}
	for _, item := range out {
		if item.ActionKey == "checked-critical" {
			t.Fatal("checked action must be excluded")
		}
	}
}

func TestListTopOpenActionsForDashboard_SellerIsolationAndLimitAndTieBreak(t *testing.T) {
	repo := &mockRepo{
		items: []ActionItemRecord{
			sampleActionRecord(1, 1, "b-key", ActionCategoryStock, ActionPriorityImportant, ActionStatusOpen),
			sampleActionRecord(2, 1, "a-key", ActionCategoryStock, ActionPriorityImportant, ActionStatusOpen),
			sampleActionRecord(3, 1, "c-key", ActionCategoryStock, ActionPriorityImportant, ActionStatusOpen),
			sampleActionRecord(4, 2, "foreign", ActionCategoryStock, ActionPriorityCritical, ActionStatusOpen),
		},
	}
	svc := NewService(repo)
	out, err := svc.ListTopOpenActionsForDashboard(context.Background(), 1, nil, 3)
	if err != nil {
		t.Fatalf("ListTopOpenActionsForDashboard: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected max 3 actions, got %d", len(out))
	}
	for _, item := range out {
		if item.ActionKey == "foreign" {
			t.Fatal("foreign seller action leaked")
		}
	}
	if out[0].ActionKey != "a-key" || out[1].ActionKey != "b-key" || out[2].ActionKey != "c-key" {
		t.Fatalf("expected deterministic tie-break by action_key, got %q %q %q", out[0].ActionKey, out[1].ActionKey, out[2].ActionKey)
	}
}

func TestListTopOpenActionsForDashboard_LatestMetricDateWhenNil(t *testing.T) {
	latest := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	old := latest.AddDate(0, 0, -1)
	oldItem := sampleActionRecord(1, 1, "old", ActionCategoryStock, ActionPriorityCritical, ActionStatusOpen)
	oldItem.MetricDate = &old
	latestItem := sampleActionRecord(2, 1, "latest", ActionCategoryStock, ActionPriorityCritical, ActionStatusOpen)
	latestItem.MetricDate = &latest
	repo := &mockRepo{
		latestMetric: &latest,
		items:        []ActionItemRecord{oldItem, latestItem},
	}
	svc := NewService(repo)
	out, err := svc.ListTopOpenActionsForDashboard(context.Background(), 1, nil, 3)
	if err != nil {
		t.Fatalf("ListTopOpenActionsForDashboard: %v", err)
	}
	if len(out) != 1 || out[0].ActionKey != "latest" {
		t.Fatalf("expected latest metric date action only, got %+v", out)
	}
}

var _ Repository = (*mockRepo)(nil)
