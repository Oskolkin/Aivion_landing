package eval

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type MemoryStore struct {
	mu         sync.Mutex
	next       int64
	Suites     map[string]Suite
	Cases      map[string]Case
	SuiteCases []SuiteCase
	Runs       map[int64]Run
	CaseRuns   map[int64]CaseRun
	Graders    []GraderResult
	Reviews    []HumanReview
	Baselines  map[string]int64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		Suites: map[string]Suite{}, Cases: map[string]Case{}, Runs: map[int64]Run{},
		CaseRuns: map[int64]CaseRun{}, Baselines: map[string]int64{},
	}
}

func (m *MemoryStore) alloc() int64 {
	m.next++
	return m.next
}

func (m *MemoryStore) UpsertSuite(ctx context.Context, s Suite) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := s.SuiteKey + "/" + s.Version
	if old, ok := m.Suites[key]; ok {
		s.ID = old.ID
	} else {
		s.ID = m.alloc()
	}
	m.Suites[key] = s
	return s.ID, nil
}

func (m *MemoryStore) InsertCase(ctx context.Context, c Case) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := c.CaseKey + "/" + c.Version
	if old, ok := m.Cases[key]; ok {
		c.ID = old.ID
	} else {
		c.ID = m.alloc()
	}
	m.Cases[key] = c
	return c.ID, nil
}

func (m *MemoryStore) AttachCase(ctx context.Context, sc SuiteCase) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SuiteCases = append(m.SuiteCases, sc)
	return nil
}

func (m *MemoryStore) InsertRun(ctx context.Context, r Run) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if r.ID == 0 {
		r.ID = m.alloc()
	}
	m.Runs[r.ID] = r
	return r.ID, nil
}

func (m *MemoryStore) UpdateRun(ctx context.Context, r Run) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Runs[r.ID] = r
	return nil
}

func (m *MemoryStore) InsertCaseRun(ctx context.Context, cr CaseRun) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cr.ID == 0 {
		cr.ID = m.alloc()
	}
	m.CaseRuns[cr.ID] = cr
	return cr.ID, nil
}

func (m *MemoryStore) InsertGraderResult(ctx context.Context, g GraderResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Graders = append(m.Graders, g)
	return nil
}

func (m *MemoryStore) InsertHumanReview(ctx context.Context, h HumanReview) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if h.ID == 0 {
		h.ID = m.alloc()
	}
	m.Reviews = append(m.Reviews, h)
	return nil
}

func (m *MemoryStore) GetRun(ctx context.Context, id int64) (Run, error) {
	if err := ctx.Err(); err != nil {
		return Run{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.Runs[id]
	if !ok {
		return Run{}, fmt.Errorf("eval run %d not found", id)
	}
	return r, nil
}

func (m *MemoryStore) GetCaseRun(ctx context.Context, id int64) (CaseRun, error) {
	if err := ctx.Err(); err != nil {
		return CaseRun{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cr, ok := m.CaseRuns[id]
	if !ok {
		return CaseRun{}, fmt.Errorf("eval case run %d not found", id)
	}
	return cr, nil
}

func (m *MemoryStore) ListCaseRuns(ctx context.Context, runID int64) ([]CaseRun, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []CaseRun
	for _, cr := range m.CaseRuns {
		if cr.EvalRunID == runID {
			out = append(out, cr)
		}
	}
	return out, nil
}

func (m *MemoryStore) PromoteBaseline(ctx context.Context, suiteID int64, split string, runID int64, by string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if by == "" {
		return fmt.Errorf("promoted_by required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Baselines[fmt.Sprintf("%d/%s", suiteID, split)] = runID
	return nil
}

func (m *MemoryStore) CurrentBaseline(ctx context.Context, suiteID int64, split string) (int64, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.Baselines[fmt.Sprintf("%d/%s", suiteID, split)]
	return id, ok, nil
}

func (m *MemoryStore) ListHumanReviews(ctx context.Context, runID int64) ([]HumanReview, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []HumanReview
	for _, h := range m.Reviews {
		if cr, ok := m.CaseRuns[h.CaseRunID]; ok && cr.EvalRunID == runID {
			out = append(out, h)
		}
	}
	return out, nil
}

func (m *MemoryStore) CountGraders() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Graders)
}

var _ Store = (*MemoryStore)(nil)

func RecordHumanReview(ctx context.Context, st Store, h HumanReview) (HumanReview, error) {
	if h.CaseRunID == 0 {
		return HumanReview{}, fmt.Errorf("case_run_id required")
	}
	if h.Reviewer == "" {
		return HumanReview{}, fmt.Errorf("reviewer required")
	}
	if err := RejectAgentHumanPass(h.Reviewer, h.Status); err != nil {
		return HumanReview{}, err
	}
	if getter, ok := st.(interface {
		GetCaseRun(context.Context, int64) (CaseRun, error)
	}); ok {
		if _, err := getter.GetCaseRun(ctx, h.CaseRunID); err != nil {
			return HumanReview{}, err
		}
	}
	if err := st.InsertHumanReview(ctx, h); err != nil {
		return HumanReview{}, err
	}
	return h, nil
}

func RejectAgentHumanPass(reviewer, status string) error {
	if (strings.EqualFold(reviewer, "codex") || strings.EqualFold(reviewer, "cursor")) && strings.EqualFold(status, "PASS") {
		return fmt.Errorf("automated agents cannot record human_review=PASS")
	}
	return nil
}

func PromoteIfGuarded(ctx context.Context, st Store, run Run, by string) error {
	reviews, err := st.ListHumanReviews(ctx, run.ID)
	if err != nil {
		return err
	}
	if err := CanPromote(run, reviews); err != nil {
		return err
	}
	return st.PromoteBaseline(ctx, run.SuiteID, run.Split, run.ID, by)
}
