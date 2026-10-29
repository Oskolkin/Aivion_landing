package collection

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type memoryObjects struct {
	body map[string][]byte
	err  error
}

func (m *memoryObjects) Put(_ context.Context, b, k string, body []byte, _ string) error {
	if m.err != nil {
		return m.err
	}
	if m.body == nil {
		m.body = map[string][]byte{}
	}
	m.body[b+"/"+k] = append([]byte(nil), body...)
	return nil
}
func (m *memoryObjects) Get(_ context.Context, b, k string) ([]byte, error) {
	v, ok := m.body[b+"/"+k]
	if !ok {
		return nil, errors.New("not found")
	}
	return v, nil
}
func TestSourcePayloadStorePutAndVerify(t *testing.T) {
	objects := &memoryObjects{}
	store := NewSourcePayloadStoreWithObjects(objects, "raw")
	stored, err := store.Put(context.Background(), 7, DomainCatalog, []byte(`{"ok":true}`), "application/json")
	if err != nil {
		t.Fatal(err)
	}
	if stored.SHA256 == "" || !strings.HasPrefix(stored.ObjectKey, "collection/ozon/seller-7/CATALOG/") {
		t.Fatalf("unexpected stored payload: %#v", stored)
	}
	ok, err := store.Verify(context.Background(), SourcePayload{Bucket: stored.Bucket, ObjectKey: stored.ObjectKey, SHA256: stored.SHA256})
	if err != nil || !ok {
		t.Fatalf("verify = %v, %v", ok, err)
	}
}

func TestSourcePayloadStoreUsesUniqueImmutableObjectKeys(t *testing.T) {
	objects := &memoryObjects{}
	store := NewSourcePayloadStoreWithObjects(objects, "raw")
	first, err := store.Put(context.Background(), 7, DomainCatalog, []byte(`{"same":true}`), "application/json")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Put(context.Background(), 7, DomainCatalog, []byte(`{"same":true}`), "application/json")
	if err != nil {
		t.Fatal(err)
	}
	if first.ObjectKey == second.ObjectKey {
		t.Fatalf("payload object key was reused: %s", first.ObjectKey)
	}
	if first.SHA256 != second.SHA256 {
		t.Fatal("equal immutable payload bytes must have equal content hashes")
	}
}
func TestSourcePayloadStoreDoesNotReportSuccessOnWriteFailure(t *testing.T) {
	store := NewSourcePayloadStoreWithObjects(&memoryObjects{err: errors.New("offline")}, "raw")
	if _, err := store.Put(context.Background(), 1, DomainCatalog, []byte("body"), ""); err == nil {
		t.Fatal("expected error")
	}
}
