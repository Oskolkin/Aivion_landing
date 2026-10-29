package pack

import (
	"testing"
	"time"
)

func TestBuildIsDeterministicForSameSnapshot(t *testing.T) {
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	snap1 := populatedSnapshot(t, metricDate)
	snap2 := populatedSnapshot(t, metricDate)

	b := NewPackBuilder(DefaultBuildOptions())
	r1, err := b.Build(snap1)
	if err != nil {
		t.Fatalf("build 1: %v", err)
	}
	r2, err := b.Build(snap2)
	if err != nil {
		t.Fatalf("build 2: %v", err)
	}

	if r1.PackHash != r2.PackHash {
		t.Fatalf("pack_hash mismatch: %s vs %s", r1.PackHash, r2.PackHash)
	}
	if r1.PackID != r2.PackID {
		t.Fatalf("pack_id mismatch: %s vs %s", r1.PackID, r2.PackID)
	}
	if len(r1.PackHash) != 64 {
		t.Fatalf("pack_hash length=%d want=64 (sha256 hex)", len(r1.PackHash))
	}

	for _, f1 := range r1.Files {
		var f2 *GeneratedFile
		for i := range r2.Files {
			if r2.Files[i].Name == f1.Name {
				f2 = &r2.Files[i]
				break
			}
		}
		if f2 == nil {
			t.Fatalf("file %s missing from second build", f1.Name)
		}
		if f1.Name == "manifest.json" {
			// manifest embeds pack_id derived from the content hash; still deterministic
			// across two builds of the same snapshot, but skip byte comparison here.
			continue
		}
		if f1.SHA256 != f2.SHA256 {
			t.Fatalf("file %s sha mismatch: %s vs %s", f1.Name, f1.SHA256, f2.SHA256)
		}
	}

	if len(r1.ZipBytes) == 0 || len(r2.ZipBytes) == 0 {
		t.Fatal("expected non-empty zip bytes")
	}
}

func TestBuildIsDeterministicAcrossSourceOrder(t *testing.T) {
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	snap := populatedSnapshot(t, metricDate)

	reversed := *snap
	reversed.AccountMetrics = reverseAccountMetrics(snap.AccountMetrics)
	reversed.SKUMetrics = reverseSKUMetrics(snap.SKUMetrics)
	reversed.AdMetrics = reverseAdMetrics(snap.AdMetrics)

	b := NewPackBuilder(DefaultBuildOptions())
	r1, err := b.Build(snap)
	if err != nil {
		t.Fatalf("build original: %v", err)
	}
	r2, err := b.Build(&reversed)
	if err != nil {
		t.Fatalf("build reversed: %v", err)
	}
	if r1.PackHash != r2.PackHash {
		t.Fatalf("pack_hash changed when input row order was reversed: %s vs %s", r1.PackHash, r2.PackHash)
	}
}
