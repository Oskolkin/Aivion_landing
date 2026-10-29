package collection

import (
	"context"
	"fmt"
	"sort"
)

type ReplayRepository interface {
	ListPayloads(context.Context, int64, int64) ([]SourcePayload, error)
}

type ReplayArtifact struct {
	Payload SourcePayload
	Body    []byte
}

// LoadReplayArtifacts returns only immutable, hash-verified artifacts from the
// same seller/domain run. Callers still validate endpoint-specific sequence,
// contract metadata, and request identity before normalization.
func LoadReplayArtifacts(ctx context.Context, repo ReplayRepository, store *SourcePayloadStore, seller, domainRun int64, endpoint, profile string) ([]ReplayArtifact, error) {
	if repo == nil {
		return nil, nil
	}
	if store == nil {
		return nil, fmt.Errorf("resume requires payload store")
	}
	rows, err := repo.ListPayloads(ctx, seller, domainRun)
	if err != nil {
		return nil, fmt.Errorf("list replay payloads: %w", err)
	}
	filtered := make([]SourcePayload, 0)
	for _, p := range rows {
		if p.EndpointKey == endpoint {
			filtered = append(filtered, p)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].PageSeq == nil {
			return false
		}
		if filtered[j].PageSeq == nil {
			return true
		}
		if *filtered[i].PageSeq == *filtered[j].PageSeq {
			return filtered[i].ID < filtered[j].ID
		}
		return *filtered[i].PageSeq < *filtered[j].PageSeq
	})
	out := make([]ReplayArtifact, 0, len(filtered))
	for _, p := range filtered {
		if p.SellerAccountID != seller || p.DomainRunID != domainRun || p.RedactionProfile != profile || p.PageSeq == nil {
			return nil, fmt.Errorf("resume artifact scope/sequence/redaction mismatch")
		}
		valid, verifyErr := store.Verify(ctx, p)
		if verifyErr != nil {
			return nil, fmt.Errorf("resume artifact integrity failure: %w", verifyErr)
		}
		if !valid {
			return nil, fmt.Errorf("resume artifact integrity failure: hash mismatch")
		}
		body, readErr := store.Get(ctx, p.Bucket, p.ObjectKey)
		if readErr != nil {
			return nil, fmt.Errorf("read replay artifact: %w", readErr)
		}
		out = append(out, ReplayArtifact{Payload: p, Body: body})
	}
	return out, nil
}
