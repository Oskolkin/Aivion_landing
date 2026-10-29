package adapter

import "testing"

func TestProjectionHash_StableAcrossKeyOrder(t *testing.T) {
	type payload map[string]any

	a := payload{"b": 1, "a": 2, "c": []string{"x", "y"}}
	b := payload{"c": []string{"x", "y"}, "a": 2, "b": 1}

	hashA, err := ProjectionHash(a)
	if err != nil {
		t.Fatalf("hash a: %v", err)
	}
	hashB, err := ProjectionHash(b)
	if err != nil {
		t.Fatalf("hash b: %v", err)
	}
	if hashA != hashB {
		t.Fatalf("expected key-order-independent hashes to match, got %q vs %q", hashA, hashB)
	}
	if len(hashA) != 64 {
		t.Fatalf("expected 64-char hex sha256 digest, got %d chars", len(hashA))
	}
}

func TestProjectionHash_DiffersOnContentChange(t *testing.T) {
	hash1, err := ProjectionHash(map[string]any{"value": 1})
	if err != nil {
		t.Fatalf("hash1: %v", err)
	}
	hash2, err := ProjectionHash(map[string]any{"value": 2})
	if err != nil {
		t.Fatalf("hash2: %v", err)
	}
	if hash1 == hash2 {
		t.Fatal("expected different content to produce different hashes")
	}
}

func TestBundleHash_StableAndOrderSensitive(t *testing.T) {
	h1 := BundleHash("aaa", "bbb", "ccc")
	h2 := BundleHash("aaa", "bbb", "ccc")
	if h1 != h2 {
		t.Fatalf("expected BundleHash to be deterministic, got %q vs %q", h1, h2)
	}
	h3 := BundleHash("bbb", "aaa", "ccc")
	if h1 == h3 {
		t.Fatal("expected BundleHash to be sensitive to part order")
	}
}

func TestHash_AdapterVersionHashIsStable(t *testing.T) {
	if Hash() != Hash() {
		t.Fatal("expected adapter Hash() to be deterministic")
	}
	if len(Hash()) != 64 {
		t.Fatalf("expected 64-char hex sha256 digest, got %d chars", len(Hash()))
	}
}
