package chatv2

import (
	"fmt"
	"sync"
)

type GroundingRegistry struct {
	mu       sync.Mutex
	byHandle map[string]GroundingEntry
	facts    int
	pubs     int
	acts     int
	atts     int
	order    []GroundingEntry
}

func NewGroundingRegistry() *GroundingRegistry {
	return &GroundingRegistry{byHandle: map[string]GroundingEntry{}}
}

func (g *GroundingRegistry) Register(t RefType, refID, fingerprint, summary string) GroundingEntry {
	g.mu.Lock()
	defer g.mu.Unlock()
	var handle string
	switch t {
	case RefFact:
		g.facts++
		handle = fmt.Sprintf("cf_%03d", g.facts)
	case RefFinding:
		g.pubs++
		handle = fmt.Sprintf("pf_%03d", g.pubs)
	case RefRecommendation:
		g.pubs++
		handle = fmt.Sprintf("pr_%03d", g.pubs)
	case RefAction, RefActionCase:
		g.acts++
		handle = fmt.Sprintf("pa_%03d", g.acts)
	case RefAttachment:
		g.atts++
		handle = fmt.Sprintf("att_%03d", g.atts)
	case RefPublication:
		g.pubs++
		handle = fmt.Sprintf("pp_%03d", g.pubs)
	default:
		g.facts++
		handle = fmt.Sprintf("cf_%03d", g.facts)
	}
	e := GroundingEntry{Handle: handle, Type: t, RefID: refID, Fingerprint: fingerprint, Summary: summary}
	g.byHandle[handle] = e
	g.order = append(g.order, e)
	return e
}

func (g *GroundingRegistry) Get(handle string) (GroundingEntry, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.byHandle[handle]
	return e, ok
}

func (g *GroundingRegistry) All() []GroundingEntry {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]GroundingEntry, len(g.order))
	copy(out, g.order)
	return out
}

func (g *GroundingRegistry) ModelView() []map[string]string {
	all := g.All()
	out := make([]map[string]string, 0, len(all))
	for _, e := range all {
		out = append(out, map[string]string{"handle": e.Handle, "type": string(e.Type), "summary": e.Summary})
	}
	return out
}
