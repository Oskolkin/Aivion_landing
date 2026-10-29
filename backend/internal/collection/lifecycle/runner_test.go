package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/jackc/pgx/v5"
)

type memoryObjects struct{ bodies map[string][]byte }

func (o *memoryObjects) Put(_ context.Context, bucket, key string, body []byte, _ string) error {
	if o.bodies == nil {
		o.bodies = map[string][]byte{}
	}
	o.bodies[bucket+"/"+key] = append([]byte(nil), body...)
	return nil
}
func (o *memoryObjects) Get(_ context.Context, bucket, key string) ([]byte, error) {
	v, ok := o.bodies[bucket+"/"+key]
	if !ok {
		return nil, errors.New("missing")
	}
	return append([]byte(nil), v...), nil
}

type runnerHarness struct {
	store    *collection.SourcePayloadStore
	objects  *memoryObjects
	payloads []collection.SourcePayload
	cursors  map[string]collection.Cursor
	next     int64
}

func newRunnerHarness() *runnerHarness {
	o := &memoryObjects{}
	return &runnerHarness{store: collection.NewSourcePayloadStoreWithObjects(o, "raw"), objects: o, cursors: map[string]collection.Cursor{}}
}
func (h *runnerHarness) StoreAndRegisterPayload(ctx context.Context, store *collection.SourcePayloadStore, in collection.RegisterPayloadInput) (collection.SourcePayload, error) {
	domain := collection.DomainPostings
	if in.EndpointKey == ReturnsEndpoint {
		domain = collection.DomainReturns
	}
	stored, e := store.Put(ctx, in.SellerAccountID, domain, in.Body, in.ContentType)
	if e != nil {
		return collection.SourcePayload{}, e
	}
	h.next++
	metadata, _ := json.Marshal(in.Metadata)
	fp, _ := collection.RequestFingerprint(in.Provider, in.EndpointKey, withDomain(in.RequestIdentity, domain))
	profile := map[string]string{FBOListEndpoint: "OZON_POSTINGS_FBO_REDACTION_V1", FBODetailEndpoint: "OZON_POSTINGS_FBO_REDACTION_V1", FBSListEndpoint: "OZON_POSTINGS_FBS_REDACTION_V1", FBSDetailEndpoint: "OZON_POSTINGS_FBS_REDACTION_V1", ReturnsEndpoint: "OZON_RETURNS_REDACTION_V1"}[in.EndpointKey]
	p := collection.SourcePayload{ID: h.next, SellerAccountID: in.SellerAccountID, DomainRunID: in.DomainRunID, Provider: in.Provider, EndpointKey: in.EndpointKey, RequestFingerprint: fp, Bucket: stored.Bucket, ObjectKey: stored.ObjectKey, SHA256: stored.SHA256, RedactionProfile: profile, ReceivedAt: time.Date(2026, 8, 20, 10, 0, int(h.next), 0, time.UTC), Metadata: metadata, PageSeq: in.PageSeq, HTTPStatus: in.HTTPStatus}
	h.payloads = append(h.payloads, p)
	return p, nil
}
func (h *runnerHarness) UpsertCursor(_ context.Context, seller int64, domain collection.CollectionDomain, key string, value []byte) (collection.Cursor, error) {
	c := collection.Cursor{ID: int64(len(h.cursors) + 1), SellerAccountID: seller, Domain: domain, Key: key, Value: append([]byte(nil), value...)}
	h.cursors[key] = c
	return c, nil
}
func (h *runnerHarness) GetCursor(_ context.Context, seller int64, domain collection.CollectionDomain, key string) (collection.Cursor, error) {
	c, ok := h.cursors[key]
	if !ok || c.SellerAccountID != seller || c.Domain != domain {
		return collection.Cursor{}, pgx.ErrNoRows
	}
	return c, nil
}
func (h *runnerHarness) ListPayloads(_ context.Context, seller, run int64) ([]collection.SourcePayload, error) {
	var out []collection.SourcePayload
	for _, p := range h.payloads {
		if p.SellerAccountID == seller && p.DomainRunID == run {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

type fakeLifecycleAPI struct {
	fbo, fbs             func(ozon.PostingListRequest) (ozon.PostingListResult, error)
	returns              func(ozon.ReturnsListRequest) (ozon.ReturnsListResult, error)
	fboDetail, fbsDetail func(ozon.PostingDetailRequest) (ozon.CanonicalPosting, error)
	fboReq, fbsReq       []ozon.PostingListRequest
	returnReq            []ozon.ReturnsListRequest
}

func (f *fakeLifecycleAPI) GetCanonicalFBOPosting(_ context.Context, _ int64, r ozon.PostingDetailRequest) (*ozon.TypedResponse[ozon.CanonicalPosting], error) {
	if f.fboDetail == nil {
		return nil, fmt.Errorf("unexpected FBO detail %s", r.PostingNumber)
	}
	v, e := f.fboDetail(r)
	if e != nil {
		return nil, e
	}
	raw, _ := json.Marshal(map[string]any{"result": v})
	return &ozon.TypedResponse[ozon.CanonicalPosting]{Raw: raw, Data: v, Meta: ozon.ResponseMeta{StatusCode: 200}}, nil
}
func (f *fakeLifecycleAPI) GetCanonicalFBSPosting(_ context.Context, _ int64, r ozon.PostingDetailRequest) (*ozon.TypedResponse[ozon.CanonicalPosting], error) {
	if f.fbsDetail == nil {
		return nil, fmt.Errorf("unexpected FBS detail %s", r.PostingNumber)
	}
	v, e := f.fbsDetail(r)
	if e != nil {
		return nil, e
	}
	raw, _ := json.Marshal(map[string]any{"result": v})
	return &ozon.TypedResponse[ozon.CanonicalPosting]{Raw: raw, Data: v, Meta: ozon.ResponseMeta{StatusCode: 200}}, nil
}

func postingResponse(data ozon.PostingListResult) (*ozon.TypedResponse[ozon.PostingListResult], error) {
	raw, _ := json.Marshal(map[string]any{"result": data})
	return &ozon.TypedResponse[ozon.PostingListResult]{Raw: raw, Data: data, Meta: ozon.ResponseMeta{StatusCode: 200}}, nil
}
func returnResponse(data ozon.ReturnsListResult) (*ozon.TypedResponse[ozon.ReturnsListResult], error) {
	raw, _ := json.Marshal(data)
	return &ozon.TypedResponse[ozon.ReturnsListResult]{Raw: raw, Data: data, Meta: ozon.ResponseMeta{StatusCode: 200}}, nil
}
func (f *fakeLifecycleAPI) ListCanonicalFBOPostings(_ context.Context, _ int64, r ozon.PostingListRequest) (*ozon.TypedResponse[ozon.PostingListResult], error) {
	f.fboReq = append(f.fboReq, r)
	v, e := f.fbo(r)
	if e != nil {
		return nil, e
	}
	return postingResponse(v)
}
func (f *fakeLifecycleAPI) ListCanonicalFBSPostings(_ context.Context, _ int64, r ozon.PostingListRequest) (*ozon.TypedResponse[ozon.PostingListResult], error) {
	f.fbsReq = append(f.fbsReq, r)
	v, e := f.fbs(r)
	if e != nil {
		return nil, e
	}
	return postingResponse(v)
}
func (f *fakeLifecycleAPI) ListCanonicalReturns(_ context.Context, _ int64, r ozon.ReturnsListRequest) (*ozon.TypedResponse[ozon.ReturnsListResult], error) {
	f.returnReq = append(f.returnReq, r)
	v, e := f.returns(r)
	if e != nil {
		return nil, e
	}
	return returnResponse(v)
}
func postingN(n int, prefix string) []ozon.CanonicalPosting {
	out := make([]ozon.CanonicalPosting, n)
	for i := range out {
		p := postingFixture("awaiting_deliver", "1")
		p.OrderID = ozon.FlexibleInt64(i + 1)
		p.OrderNumber = fmt.Sprintf("O-%s-%d", prefix, i)
		p.PostingNumber = fmt.Sprintf("P-%s-%d", prefix, i)
		out[i] = p
	}
	return out
}
func returnN(start, n int) []ozon.CanonicalReturn {
	out := make([]ozon.CanonicalReturn, n)
	for i := range out {
		out[i].ID = ozon.FlexibleInt64(start + i + 1)
		out[i].Quantity = "1"
		out[i].Price = "1"
		out[i].CurrencyCode = "RUB"
	}
	return out
}
func runner(h *runnerHarness, api *fakeLifecycleAPI) *Runner {
	return &Runner{API: api, Collection: h, Payloads: h.store, Cursors: h, Recovery: h}
}

func TestPostingPaginationUsesIndependentAdvancingCursors(t *testing.T) {
	h := newRunnerHarness()
	api := &fakeLifecycleAPI{}
	api.fbo = func(r ozon.PostingListRequest) (ozon.PostingListResult, error) {
		if r.Cursor == "" {
			return ozon.PostingListResult{Postings: postingN(1, "a"), HasNext: true, Cursor: "fbo-2"}, nil
		}
		return ozon.PostingListResult{Postings: postingN(1, "b")}, nil
	}
	api.fbs = func(r ozon.PostingListRequest) (ozon.PostingListResult, error) { return ozon.PostingListResult{}, nil }
	d := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	result, e := runner(h, api).CollectPostings(context.Background(), 1, 10, d, d)
	if e != nil {
		t.Fatal(e)
	}
	if result.FBO.Pages != 2 || result.FBS.Pages != 1 || result.Postings != 2 || api.fboReq[0].Cursor != "" || api.fboReq[1].Cursor != "fbo-2" || api.fbsReq[0].Cursor != "" {
		t.Fatalf("result=%+v fbo=%+v fbs=%+v", result, api.fboReq, api.fbsReq)
	}
}

func TestFBSIncrementalUsesChangedStatusWindow(t *testing.T) {
	h := newRunnerHarness()
	api := &fakeLifecycleAPI{fbo: func(ozon.PostingListRequest) (ozon.PostingListResult, error) { return ozon.PostingListResult{}, nil }, fbs: func(r ozon.PostingListRequest) (ozon.PostingListResult, error) {
		if r.Filter.LastChangedStatusDate == nil || r.Filter.Since != "" || r.Filter.To != "" {
			t.Fatalf("not incremental: %+v", r.Filter)
		}
		return ozon.PostingListResult{}, nil
	}}
	d := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	result, e := runner(h, api).CollectPostingsMode(context.Background(), 1, 80, PostingModeIncrementalChangedStatus, d, d)
	if e != nil {
		t.Fatal(e)
	}
	if result.FBO.Status != "EMPTY_VALID" || result.FBS.Status != "EMPTY_VALID" {
		t.Fatalf("result=%+v", result)
	}
}

func TestPermanentStreamFailureKeepsSuccessfulOtherStream(t *testing.T) {
	h := newRunnerHarness()
	api := &fakeLifecycleAPI{fbo: func(ozon.PostingListRequest) (ozon.PostingListResult, error) {
		return ozon.PostingListResult{Postings: postingN(1, "ok")}, nil
	}, fbs: func(ozon.PostingListRequest) (ozon.PostingListResult, error) {
		return ozon.PostingListResult{}, &ozon.ProviderError{StatusCode: 403, Body: "forbidden"}
	}}
	d := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	result, e := runner(h, api).CollectPostings(context.Background(), 1, 81, d, d)
	if e != nil {
		t.Fatal(e)
	}
	if result.Postings != 1 || result.FBO.Status != "COMPLETE" || result.FBS.Status != "UNAVAILABLE" {
		t.Fatalf("result=%+v", result)
	}
}

func TestTransientStreamInterruptionNeverReturnsFinalizablePartial(t *testing.T) {
	h := newRunnerHarness()
	api := &fakeLifecycleAPI{fbo: func(ozon.PostingListRequest) (ozon.PostingListResult, error) {
		return ozon.PostingListResult{Postings: postingN(1, "ok")}, nil
	}, fbs: func(ozon.PostingListRequest) (ozon.PostingListResult, error) {
		return ozon.PostingListResult{}, context.DeadlineExceeded
	}}
	d := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	result, e := runner(h, api).CollectPostings(context.Background(), 1, 82, d, d)
	if !errors.Is(e, context.DeadlineExceeded) || result.Postings != 0 || result.FBS.Status != "INTERRUPTED" {
		t.Fatalf("result=%+v err=%v", result, e)
	}
}

func TestRequiredDetailIsPersistedAndReplayedWithoutRefetch(t *testing.T) {
	h := newRunnerHarness()
	summary := postingFixture("new", "1")
	summary.Products = nil
	detailCalls := 0
	api := &fakeLifecycleAPI{fbo: func(ozon.PostingListRequest) (ozon.PostingListResult, error) {
		return ozon.PostingListResult{Postings: []ozon.CanonicalPosting{summary}}, nil
	}, fbs: func(ozon.PostingListRequest) (ozon.PostingListResult, error) { return ozon.PostingListResult{}, nil }, fboDetail: func(r ozon.PostingDetailRequest) (ozon.CanonicalPosting, error) {
		detailCalls++
		if r.PostingNumber != "POST-1" {
			t.Fatalf("detail=%s", r.PostingNumber)
		}
		return postingFixture("new", "1"), nil
	}}
	d := time.Now()
	result, e := runner(h, api).CollectPostings(context.Background(), 1, 9, d, d)
	if e != nil {
		t.Fatal(e)
	}
	if detailCalls != 1 || result.FBO.Details != 1 || len(h.payloads) != 3 {
		t.Fatalf("details=%d result=%+v payloads=%d", detailCalls, result, len(h.payloads))
	}
	blocked := &fakeLifecycleAPI{fbo: func(ozon.PostingListRequest) (ozon.PostingListResult, error) {
		t.Fatal("list refetched")
		return ozon.PostingListResult{}, nil
	}, fbs: func(ozon.PostingListRequest) (ozon.PostingListResult, error) {
		t.Fatal("list refetched")
		return ozon.PostingListResult{}, nil
	}}
	again, e := runner(h, blocked).CollectPostings(context.Background(), 1, 9, d, d)
	if e != nil || again.Postings != 1 {
		t.Fatalf("replay=%+v err=%v", again, e)
	}
}
func TestPostingPaginationRejectsNonAdvancingCursor(t *testing.T) {
	h := newRunnerHarness()
	api := &fakeLifecycleAPI{}
	api.fbo = func(r ozon.PostingListRequest) (ozon.PostingListResult, error) {
		return ozon.PostingListResult{HasNext: true, Cursor: r.Cursor}, nil
	}
	api.fbs = func(ozon.PostingListRequest) (ozon.PostingListResult, error) { return ozon.PostingListResult{}, nil }
	d := time.Now()
	_, e := runner(h, api).CollectPostings(context.Background(), 1, 11, d, d)
	if !errors.Is(e, ErrPaginationNonAdvancing) {
		t.Fatalf("err=%v", e)
	}
}

func TestPostingUniverseRejectsOrderAndSchemeIdentityConflicts(t *testing.T) {
	payload := collection.SourcePayload{ID: 1, ReceivedAt: time.Now()}
	fbo, fbs := postingFixture("new", "1"), postingFixture("new", "1")
	seen := map[string]PostingCandidate{}
	if e := acceptPostingRows([]ozon.CanonicalPosting{fbo}, postingStream{name: "FBO"}, payload, seen); e != nil {
		t.Fatal(e)
	}
	fbs.OrderNumber = "DIFFERENT"
	if e := acceptPostingRows([]ozon.CanonicalPosting{fbs}, postingStream{name: "FBS"}, payload, seen); !errors.Is(e, ErrOrderIdentityConflict) {
		t.Fatalf("order conflict=%v", e)
	}
	fbs.OrderNumber = fbo.OrderNumber
	fbs.DeliveryMethod.Schema = "FBS"
	if e := acceptPostingRows([]ozon.CanonicalPosting{fbs}, postingStream{name: "FBS"}, payload, seen); !errors.Is(e, ErrPostingSchemeConflict) {
		t.Fatalf("scheme conflict=%v", e)
	}
}
func TestReturnsPaginationLimitLastIDAndHasNext(t *testing.T) {
	h := newRunnerHarness()
	api := &fakeLifecycleAPI{returns: func(r ozon.ReturnsListRequest) (ozon.ReturnsListResult, error) {
		if r.LastID == 0 {
			return ozon.ReturnsListResult{Returns: returnN(0, 2), HasNext: true, LastID: 2}, nil
		}
		return ozon.ReturnsListResult{Returns: returnN(2, 1), LastID: 3}, nil
	}}
	d := time.Now()
	result, e := runner(h, api).CollectReturns(context.Background(), 1, 12, d, d)
	if e != nil {
		t.Fatal(e)
	}
	if result.Stream.Pages != 2 || result.Returns != 3 || api.returnReq[0].Limit != 500 || api.returnReq[0].LastID != 0 || api.returnReq[1].LastID != 2 {
		t.Fatalf("result=%+v req=%+v", result, api.returnReq)
	}
}
func TestSameRunPostingResumeReplaysPrefixAndNewRunStartsEmpty(t *testing.T) {
	for _, stream := range []string{"FBO", "FBS"} {
		t.Run(stream, func(t *testing.T) {
			h := newRunnerHarness()
			first := &fakeLifecycleAPI{}
			calls := 0
			fn := func(r ozon.PostingListRequest) (ozon.PostingListResult, error) {
				calls++
				if calls == 1 {
					return ozon.PostingListResult{Postings: postingN(1, stream+"a"), HasNext: true, Cursor: "next"}, nil
				}
				return ozon.PostingListResult{}, ozon.ErrNetworkError
			}
			empty := func(ozon.PostingListRequest) (ozon.PostingListResult, error) { return ozon.PostingListResult{}, nil }
			if stream == "FBO" {
				first.fbo = fn
				first.fbs = empty
			} else {
				first.fbo = empty
				first.fbs = fn
			}
			d := time.Now()
			_, e := runner(h, first).CollectPostings(context.Background(), 1, 20, d, d)
			if !errors.Is(e, ozon.ErrNetworkError) {
				t.Fatalf("first err=%v", e)
			}
			resume := &fakeLifecycleAPI{}
			resumeFn := func(r ozon.PostingListRequest) (ozon.PostingListResult, error) {
				if r.Cursor != "next" {
					t.Fatalf("resume cursor=%q", r.Cursor)
				}
				return ozon.PostingListResult{Postings: postingN(1, stream+"b")}, nil
			}
			if stream == "FBO" {
				resume.fbo = resumeFn
				resume.fbs = empty
			} else {
				resume.fbo = empty
				resume.fbs = resumeFn
			}
			result, e := runner(h, resume).CollectPostings(context.Background(), 1, 20, d, d)
			if e != nil {
				t.Fatal(e)
			}
			if result.Postings != 2 {
				t.Fatalf("result=%+v", result)
			}
			fresh := &fakeLifecycleAPI{fbo: empty, fbs: empty}
			_, e = runner(h, fresh).CollectPostings(context.Background(), 1, 21, d, d)
			if e != nil {
				t.Fatal(e)
			}
			if fresh.fboReq[0].Cursor != "" || fresh.fbsReq[0].Cursor != "" {
				t.Fatal("new run inherited cursor")
			}
		})
	}
}
func TestSameRunReturnsResumeAndIntegrityFailure(t *testing.T) {
	h := newRunnerHarness()
	calls := 0
	first := &fakeLifecycleAPI{returns: func(r ozon.ReturnsListRequest) (ozon.ReturnsListResult, error) {
		calls++
		if calls == 1 {
			return ozon.ReturnsListResult{Returns: returnN(0, 1), HasNext: true, LastID: 1}, nil
		}
		return ozon.ReturnsListResult{}, ozon.ErrNetworkError
	}}
	d := time.Now()
	_, e := runner(h, first).CollectReturns(context.Background(), 1, 30, d, d)
	if !errors.Is(e, ozon.ErrNetworkError) {
		t.Fatal(e)
	}
	resume := &fakeLifecycleAPI{returns: func(r ozon.ReturnsListRequest) (ozon.ReturnsListResult, error) {
		if r.LastID != 1 {
			t.Fatalf("last_id=%d", r.LastID)
		}
		return ozon.ReturnsListResult{Returns: returnN(1, 1), LastID: 2}, nil
	}}
	result, e := runner(h, resume).CollectReturns(context.Background(), 1, 30, d, d)
	if e != nil || result.Returns != 2 {
		t.Fatalf("result=%+v err=%v", result, e)
	}
	h2 := newRunnerHarness()
	calls = 0
	_, _ = runner(h2, first).CollectReturns(context.Background(), 1, 31, d, d)
	for k := range h2.objects.bodies {
		delete(h2.objects.bodies, k)
		break
	}
	noCall := &fakeLifecycleAPI{returns: func(ozon.ReturnsListRequest) (ozon.ReturnsListResult, error) {
		t.Fatal("provider called")
		return ozon.ReturnsListResult{}, nil
	}}
	_, e = runner(h2, noCall).CollectReturns(context.Background(), 1, 31, d, d)
	if !errors.Is(e, ErrResumeArtifactIntegrity) {
		t.Fatalf("integrity err=%v", e)
	}
}
func TestResumedPostingsUniverseMatchesUninterrupted(t *testing.T) {
	h := newRunnerHarness()
	first := &fakeLifecycleAPI{}
	calls := 0
	first.fbo = func(r ozon.PostingListRequest) (ozon.PostingListResult, error) {
		calls++
		if calls == 1 {
			return ozon.PostingListResult{Postings: postingN(1, "a"), HasNext: true, Cursor: "c"}, nil
		}
		return ozon.PostingListResult{}, ozon.ErrNetworkError
	}
	first.fbs = func(ozon.PostingListRequest) (ozon.PostingListResult, error) { return ozon.PostingListResult{}, nil }
	d := time.Now()
	_, _ = runner(h, first).CollectPostings(context.Background(), 1, 40, d, d)
	cont := &fakeLifecycleAPI{fbo: func(ozon.PostingListRequest) (ozon.PostingListResult, error) {
		return ozon.PostingListResult{Postings: postingN(1, "b")}, nil
	}, fbs: first.fbs}
	resumed, e := runner(h, cont).CollectPostings(context.Background(), 1, 40, d, d)
	if e != nil {
		t.Fatal(e)
	}
	freshH := newRunnerHarness()
	freshCalls := 0
	fresh := &fakeLifecycleAPI{fbo: func(ozon.PostingListRequest) (ozon.PostingListResult, error) {
		freshCalls++
		if freshCalls == 1 {
			return ozon.PostingListResult{Postings: postingN(1, "a"), HasNext: true, Cursor: "c"}, nil
		}
		return ozon.PostingListResult{Postings: postingN(1, "b")}, nil
	}, fbs: first.fbs}
	uninterrupted, e := runner(freshH, fresh).CollectPostings(context.Background(), 1, 41, d, d)
	if e != nil {
		t.Fatal(e)
	}
	var a, b []string
	for _, p := range resumed.postings {
		a = append(a, p.PostingNumber)
	}
	for _, p := range uninterrupted.postings {
		b = append(b, p.PostingNumber)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("resumed=%v fresh=%v", a, b)
	}
}

func TestPostingReplayIntegrityFailuresAreFailClosed(t *testing.T) {
	mutations := map[string]func(*runnerHarness){
		"corrupt-sha": func(h *runnerHarness) { h.payloads[0].SHA256 = fmt.Sprintf("%064d", 9) },
		"wrong-contract": func(h *runnerHarness) {
			var m map[string]any
			_ = json.Unmarshal(h.payloads[0].Metadata, &m)
			m["ContractKey"] = "wrong"
			h.payloads[0].Metadata, _ = json.Marshal(m)
		},
		"page-hole": func(h *runnerHarness) { v := int64(2); h.payloads[0].PageSeq = &v },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			h := newRunnerHarness()
			calls := 0
			api := &fakeLifecycleAPI{}
			api.fbo = func(ozon.PostingListRequest) (ozon.PostingListResult, error) {
				calls++
				if calls == 1 {
					return ozon.PostingListResult{Postings: postingN(1, "x"), HasNext: true, Cursor: "next"}, nil
				}
				return ozon.PostingListResult{}, ozon.ErrNetworkError
			}
			api.fbs = func(ozon.PostingListRequest) (ozon.PostingListResult, error) { return ozon.PostingListResult{}, nil }
			d := time.Now()
			_, _ = runner(h, api).CollectPostings(context.Background(), 1, 50, d, d)
			mutate(h)
			providerCalled := false
			blocked := &fakeLifecycleAPI{fbo: func(ozon.PostingListRequest) (ozon.PostingListResult, error) {
				providerCalled = true
				return ozon.PostingListResult{}, nil
			}, fbs: api.fbs}
			_, e := runner(h, blocked).CollectPostings(context.Background(), 1, 50, d, d)
			if !errors.Is(e, ErrResumeArtifactIntegrity) || providerCalled {
				t.Fatalf("err=%v called=%v", e, providerCalled)
			}
		})
	}
}
