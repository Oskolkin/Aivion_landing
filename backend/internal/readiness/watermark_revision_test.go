package readiness

import "testing"

func TestShouldAppendWatermarkRevisionABA(t *testing.T) {
	hashA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hashB := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if !shouldAppendWatermarkRevision("", false, hashA) {
		t.Fatal("first insert must append")
	}
	if shouldAppendWatermarkRevision(hashA, true, hashA) {
		t.Fatal("A→A must not append")
	}
	if !shouldAppendWatermarkRevision(hashA, true, hashB) {
		t.Fatal("A→B must append")
	}
	if !shouldAppendWatermarkRevision(hashB, true, hashA) {
		t.Fatal("A→B→A must append when returning to A")
	}
}
