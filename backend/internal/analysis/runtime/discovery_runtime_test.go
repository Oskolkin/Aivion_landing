package runtime

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/policy"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

func TestAttemptTimeoutUsesAIDefault(t *testing.T) {
	p := policy.V3(nil, nil)
	p.Timeouts["stage_default"] = 20 * time.Millisecond
	p.Timeouts["ai_default"] = 200 * time.Millisecond
	e := &Executor{Policy: p, Registry: stages.ProductionRegistry}
	ai := e.attemptTimeout(stages.DiscoveryAI)
	st := e.attemptTimeout(stages.BuildDiscoveryPack)
	if ai != 200*time.Millisecond {
		t.Fatalf("ai timeout %s", ai)
	}
	if ai <= st {
		t.Fatal("ai_default must exceed stage_default")
	}
	stageCtx, cancelStage := context.WithTimeout(context.Background(), st)
	defer cancelStage()
	select {
	case <-time.After(80 * time.Millisecond):
		if stageCtx.Err() == nil {
			t.Fatal("stage_default window should expire before 80ms simulated AI work")
		}
	case <-stageCtx.Done():
	}
	aiCtx, cancelAI := context.WithTimeout(context.Background(), ai)
	defer cancelAI()
	select {
	case <-time.After(80 * time.Millisecond):
		if aiCtx.Err() != nil {
			t.Fatal("ai_default must allow work longer than stage_default")
		}
	case <-aiCtx.Done():
		t.Fatal("ai_default cancelled too early")
	}
}

func TestHashDiscoveryStages(t *testing.T) {
	h1 := HashBuildDiscoveryPack(BuildDiscoveryPackInput{
		UniverseManifestHash: strings64("a"), MaterialitySelectionHash: strings64("b"), LandscapeDetailHash: strings64("c"),
		DiscoveryPackSeal: "p", FactHandleRegistry: "f", EvidenceCatalogSeal: "e", PackBudgetVersion: "bud",
		ContractBundleHash: strings64("k"),
	})
	h2 := HashBuildDiscoveryPack(BuildDiscoveryPackInput{
		UniverseManifestHash: strings64("a"), MaterialitySelectionHash: strings64("b"), LandscapeDetailHash: strings64("d"),
		DiscoveryPackSeal: "p", FactHandleRegistry: "f", EvidenceCatalogSeal: "e", PackBudgetVersion: "bud",
		ContractBundleHash: strings64("k"),
	})
	if RequireInputHash(h1) != nil || h1 == h2 {
		t.Fatal("detail hash must change pack input hash")
	}
	payload := strings64("p")
	mp := strings64("m")
	man := strings64("n")
	ai := HashDiscoveryAI(DiscoveryAIInput{
		DiscoveryPackHash: h1, PayloadHash: payload, FactHandleMapHash: mp, RequestManifestHash: man,
		PromptPolicyVersion: "pv", OutputSchemaVersion: "ov", ModelPolicyVersion: "mv", OutputBudgetVersion: "bv", ContractBundleHash: strings64("k"),
	})
	if RequireInputHash(ai) != nil {
		t.Fatal(ai)
	}
	ai2 := HashDiscoveryAI(DiscoveryAIInput{
		DiscoveryPackHash: h1, PayloadHash: strings64("q"), FactHandleMapHash: mp, RequestManifestHash: man,
		PromptPolicyVersion: "pv", OutputSchemaVersion: "ov", ModelPolicyVersion: "mv", OutputBudgetVersion: "bv", ContractBundleHash: strings64("k"),
	})
	if ai == ai2 {
		t.Fatal("payload hash must change Stage 8 hash")
	}
	menu := strings64("e")
	v := HashValidateDiscovery(ValidateDiscoveryInput{
		AcceptedDiscoveryOutputHash: strings64("o"), DiscoveryPackHash: h1, FactHandleMapHash: mp, EvidenceMenuHash: menu,
		ValidationPolicyVersion: "v", OutputBudgetVersion: "b", ContractBundleHash: strings64("k"),
	})
	if RequireInputHash(v) != nil {
		t.Fatal(v)
	}
	v2 := HashValidateDiscovery(ValidateDiscoveryInput{
		AcceptedDiscoveryOutputHash: strings64("o"), DiscoveryPackHash: h1, FactHandleMapHash: mp, EvidenceMenuHash: strings64("f"),
		ValidationPolicyVersion: "v", OutputBudgetVersion: "b", ContractBundleHash: strings64("k"),
	})
	if v == v2 {
		t.Fatal("EvidenceMenuHash must change Stage 9 hash")
	}
	if HashDiscoveryAI(DiscoveryAIInput{DiscoveryPackHash: h1, PromptPolicyVersion: "pv", OutputSchemaVersion: "ov", ModelPolicyVersion: "mv", OutputBudgetVersion: "bv", ContractBundleHash: strings64("k")}) != "" {
		t.Fatal("missing supporting hashes must not produce a Stage 8 input hash")
	}
}

func TestFrontierEventsOnlyForPostPublish(t *testing.T) {
	if extraSuccessEvents(string(stages.ValidateDeep), string(CompletionSucceeded)) != nil {
		t.Fatal("frontier must not emit before POST_PUBLISH")
	}
	ev := extraSuccessEvents(string(stages.PostPublish), string(CompletionSucceeded))
	if len(ev) != 1 || ev[0].Kind != "IMPLEMENTED_FRONTIER_REACHED" {
		t.Fatalf("%+v", ev)
	}
	ev2 := extraSuccessEvents(string(stages.PostPublish), string(CompletionSucceededWithLimitations))
	if len(ev2) != 1 {
		t.Fatal("limitations completion must still emit frontier")
	}
}

func TestLeaseHeartbeatKeepsLeaseAndCancelsOnLoss(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	hb := &fakeHeartbeater{}
	lease := analysispg.Lease{StageID: 1, Token: "t", Version: 1}
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	handlerCtx, cancelHandler := context.WithCancel(parent)
	ctl := startLeaseHeartbeat(parent, hb, lease, 50*time.Millisecond, clock, cancelHandler)
	time.Sleep(80 * time.Millisecond)
	ctl.Stop()
	if hb.n == 0 {
		t.Fatal("expected heartbeats during slow handler")
	}
	if ctl.Lost() {
		t.Fatal("healthy heartbeat must not lose lease")
	}
	if handlerCtx.Err() != nil {
		t.Fatal("handler must stay alive while heartbeat succeeds")
	}
	cancelHandler()

	hb2 := &fakeHeartbeater{failAfter: 1}
	handlerCtx2, cancelHandler2 := context.WithCancel(context.Background())
	ctl2 := startLeaseHeartbeat(context.Background(), hb2, lease, 30*time.Millisecond, clock, cancelHandler2)
	deadline := time.Now().Add(400 * time.Millisecond)
	for handlerCtx2.Err() == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	ctl2.Stop()
	if !ctl2.Lost() {
		t.Fatal("lost fencing token must mark heartbeat lost")
	}
	if handlerCtx2.Err() == nil {
		t.Fatal("lease loss must cancel handler context")
	}
}

func TestDiscoveryProviderFailClosed(t *testing.T) {
	if _, err := NewDiscoveryProvider(DiscoveryProviderConfig{Mode: "fake", Environment: "production"}); err == nil {
		t.Fatal("prod fake")
	}
	if _, err := NewDiscoveryProvider(DiscoveryProviderConfig{Mode: "fake", Environment: "staging"}); err == nil {
		t.Fatal("staging fake")
	}
	if _, err := NewDiscoveryProvider(DiscoveryProviderConfig{Mode: "real"}); err == nil {
		t.Fatal("real missing key")
	}
	if _, err := NewDiscoveryProvider(DiscoveryProviderConfig{
		Mode: "real", APIKey: "sk-test",
		BuildHTTPClient: func(int, string) (*http.Client, error) { return nil, errors.New("proxy") },
	}); err == nil {
		t.Fatal("http client failure")
	}
	p, err := NewDiscoveryProvider(DiscoveryProviderConfig{
		Mode: "openai", APIKey: "sk-test",
		BuildHTTPClient: func(int, string) (*http.Client, error) { return &http.Client{}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.(*discovery.OpenAIProvider); !ok {
		t.Fatalf("openai alias got %T", p)
	}
	if _, err := NewDiscoveryProvider(DiscoveryProviderConfig{Mode: "offline"}); err == nil {
		t.Fatal("unknown mode")
	}
	p, err = NewDiscoveryProvider(DiscoveryProviderConfig{Mode: "fake", Environment: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.(*discovery.FakeProvider); !ok {
		t.Fatalf("got %T", p)
	}
}

type fakeHeartbeater struct {
	n         int
	failAfter int
}

func (f *fakeHeartbeater) Heartbeat(context.Context, analysispg.Lease, time.Time) error {
	f.n++
	if f.failAfter > 0 && f.n >= f.failAfter {
		return analysispg.ErrLeaseLost
	}
	return nil
}

func strings64(c string) string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = c[0]
	}
	return string(b)
}
