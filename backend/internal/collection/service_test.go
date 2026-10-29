package collection

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeServiceRepository struct {
	run             CollectionRun
	domainRun       DomainRun
	payloads        []SourcePayload
	beforeRunCAS    func()
	beforeDomainCAS func()
}

func (f *fakeServiceRepository) GetRun(_ context.Context, sellerID, id int64) (CollectionRun, error) {
	if f.run.SellerAccountID != sellerID || f.run.ID != id {
		return CollectionRun{}, errors.New("run not found")
	}
	return f.run, nil
}

func (f *fakeServiceRepository) TransitionRun(_ context.Context, _ int64, id int64, expected, target CollectionRunStatus) (CollectionRun, error) {
	if f.beforeRunCAS != nil {
		hook := f.beforeRunCAS
		f.beforeRunCAS = nil
		hook()
	}
	if f.run.Status != expected {
		return CollectionRun{}, &StaleTransitionError{EntityKind: "collection run", EntityID: id, Expected: string(expected), Requested: string(target)}
	}
	f.run.Status = target
	return f.run, nil
}

func (f *fakeServiceRepository) GetDomainRun(_ context.Context, sellerID, id int64) (DomainRun, error) {
	if f.domainRun.SellerAccountID != sellerID || f.domainRun.ID != id {
		return DomainRun{}, errors.New("domain run not found")
	}
	return f.domainRun, nil
}

func (f *fakeServiceRepository) TransitionDomainRun(_ context.Context, _ int64, id int64, expected, target DomainRunStatus) (DomainRun, error) {
	if f.beforeDomainCAS != nil {
		hook := f.beforeDomainCAS
		f.beforeDomainCAS = nil
		hook()
	}
	if f.domainRun.Status != expected {
		return DomainRun{}, &StaleTransitionError{EntityKind: "domain run", EntityID: id, Expected: string(expected), Requested: string(target)}
	}
	f.domainRun.Status = target
	return f.domainRun, nil
}

func (f *fakeServiceRepository) UpsertCursor(_ context.Context, sellerID int64, domain CollectionDomain, key string, value []byte) (Cursor, error) {
	return Cursor{SellerAccountID: sellerID, Domain: domain, Key: key, Value: value}, nil
}

func (f *fakeServiceRepository) CreatePayload(_ context.Context, payload SourcePayload) (SourcePayload, error) {
	f.payloads = append(f.payloads, payload)
	return payload, nil
}

func TestTransitionRunRejectsStaleCompetingTerminalWrite(t *testing.T) {
	repo := &fakeServiceRepository{run: CollectionRun{ID: 10, SellerAccountID: 7, Status: CollectionRunStatusRunning}}
	repo.beforeRunCAS = func() { repo.run.Status = CollectionRunStatusCompleted }

	_, err := NewService(repo).TransitionRun(context.Background(), 7, 10, CollectionRunStatusFailed)
	var stale *StaleTransitionError
	if !errors.As(err, &stale) {
		t.Fatalf("error=%v, want typed stale transition", err)
	}
	if repo.run.Status != CollectionRunStatusCompleted {
		t.Fatalf("stale actor overwrote final status: %s", repo.run.Status)
	}
}

func TestTransitionDomainRunRejectsStaleCompetingTerminalWrite(t *testing.T) {
	repo := &fakeServiceRepository{domainRun: DomainRun{ID: 11, SellerAccountID: 7, Status: DomainRunStatusRunning}}
	repo.beforeDomainCAS = func() { repo.domainRun.Status = DomainRunStatusCompleted }

	_, err := NewService(repo).TransitionDomainRun(context.Background(), 7, 11, DomainRunStatusFailed)
	var stale *StaleTransitionError
	if !errors.As(err, &stale) {
		t.Fatalf("error=%v, want typed stale transition", err)
	}
	if repo.domainRun.Status != DomainRunStatusCompleted {
		t.Fatalf("stale actor overwrote final status: %s", repo.domainRun.Status)
	}
}

func TestLifecycleNormalIdempotentInvalidAndSellerIsolation(t *testing.T) {
	repo := &fakeServiceRepository{
		run:       CollectionRun{ID: 10, SellerAccountID: 7, Status: CollectionRunStatusPending},
		domainRun: DomainRun{ID: 11, SellerAccountID: 7, Status: DomainRunStatusPending},
	}
	service := NewService(repo)
	if _, err := service.TransitionRun(context.Background(), 7, 10, CollectionRunStatusRunning); err != nil {
		t.Fatal(err)
	}
	if _, err := service.TransitionRun(context.Background(), 7, 10, CollectionRunStatusRunning); err != nil {
		t.Fatalf("same-target transition is not idempotent: %v", err)
	}
	if _, err := service.TransitionRun(context.Background(), 7, 10, CollectionRunStatusPending); err == nil {
		t.Fatal("invalid transition succeeded")
	}
	if _, err := service.TransitionRun(context.Background(), 8, 10, CollectionRunStatusCompleted); err == nil {
		t.Fatal("cross-seller transition succeeded")
	}
	if _, err := service.TransitionDomainRun(context.Background(), 7, 11, DomainRunStatusRunning); err != nil {
		t.Fatal(err)
	}
}

func TestStoreAndRegisterPayloadDerivesDomainFromSellerScopedDomainRun(t *testing.T) {
	repo := &fakeServiceRepository{domainRun: DomainRun{ID: 11, SellerAccountID: 7, Domain: DomainCatalog}}
	objects := &memoryObjects{}
	store := NewSourcePayloadStoreWithObjects(objects, "raw")

	payload, err := NewService(repo).StoreAndRegisterPayload(context.Background(), store, RegisterPayloadInput{
		SellerAccountID: 7,
		DomainRunID:     11,
		Provider:        ProviderOzon,
		EndpointKey:     "ozon.product.list",
		RequestIdentity: map[string]any{"visibility": "ALL"},
		Metadata:        map[string]any{"stream": "ALL"},
		Body:            []byte(`{"result":{}}`),
		ContentType:     "application/json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload.ObjectKey, "/CATALOG/") || strings.Contains(payload.ObjectKey, "/STOCK/") {
		t.Fatalf("object key did not use persisted domain: %s", payload.ObjectKey)
	}
	var metadata map[string]any
	if err := json.Unmarshal(payload.Metadata, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["collection_domain"] != "CATALOG" {
		t.Fatalf("metadata domain=%v", metadata["collection_domain"])
	}
}

func TestStoreAndRegisterPayloadUsesPersistedDomainInsteadOfCallerInput(t *testing.T) {
	repo := &fakeServiceRepository{domainRun: DomainRun{ID: 11, SellerAccountID: 7, Domain: DomainCatalog}}
	objects := &memoryObjects{}
	store := NewSourcePayloadStoreWithObjects(objects, "raw")
	payload, err := NewService(repo).StoreAndRegisterPayload(context.Background(), store, RegisterPayloadInput{
		SellerAccountID: 7,
		DomainRunID:     11,
		Provider:        ProviderOzon,
		EndpointKey:     "ozon.product.list",
		Body:            []byte(`{"body":true}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload.ObjectKey, "/CATALOG/") || len(repo.payloads) != 1 {
		t.Fatalf("payload=%+v registered=%d", payload, len(repo.payloads))
	}
}

func TestStoreAndRegisterPayloadRejectsWrongSellerBeforeObjectWrite(t *testing.T) {
	repo := &fakeServiceRepository{domainRun: DomainRun{ID: 11, SellerAccountID: 7, Domain: DomainCatalog}}
	objects := &memoryObjects{}
	store := NewSourcePayloadStoreWithObjects(objects, "raw")
	_, err := NewService(repo).StoreAndRegisterPayload(context.Background(), store, RegisterPayloadInput{
		SellerAccountID: 8,
		DomainRunID:     11,
		Provider:        ProviderOzon,
		EndpointKey:     "ozon.product.list",
		Body:            []byte("body"),
	})
	if err == nil {
		t.Fatal("cross-seller payload registration succeeded")
	}
	if len(objects.body) != 0 || len(repo.payloads) != 0 {
		t.Fatal("payload side effect occurred before seller/domain validation")
	}
}

func TestCatalogPayloadIsStructurallyRedactedBeforeObjectWrite(t *testing.T) {
	repo := &fakeServiceRepository{domainRun: DomainRun{
		ID: 11, SellerAccountID: 7, Domain: DomainCatalog,
		SourceContractKey: "OZON_CATALOG", SourceContractVersion: "1.0.0",
		SemanticMappingVersion: "1.0.0", SourceRevisionHash: "semantic-hash",
	}}
	objects := &memoryObjects{}
	store := NewSourcePayloadStoreWithObjects(objects, "raw")
	payload, err := NewService(repo).StoreAndRegisterPayload(context.Background(), store, RegisterPayloadInput{
		SellerAccountID: 7, DomainRunID: 11, Provider: ProviderOzon, EndpointKey: "ozon.product.list",
		Body:        []byte(`{"result":{"api_key":"secret","nested":{"access_token":"token","name":"safe"}},"authorization":"Bearer secret"}`),
		ContentType: "application/json",
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := objects.Get(context.Background(), payload.Bucket, payload.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	text := string(stored)
	if strings.Contains(text, "secret") || strings.Contains(text, "token") || strings.Contains(text, "authorization") || !strings.Contains(text, `"name":"safe"`) {
		t.Fatalf("unsafe redacted payload: %s", text)
	}
	if payload.RedactionProfile != "OZON_CATALOG_REDACTION_V1" {
		t.Fatalf("redaction profile=%q", payload.RedactionProfile)
	}
}
