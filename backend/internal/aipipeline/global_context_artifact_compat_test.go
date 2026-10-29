package aipipeline_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

type gcArtifactTamper func(t *testing.T, store *persistingArtifactStore, art *dbgen.AiPipelineArtifact)

func runSharedGlobalContextFirstPass(t *testing.T) (
	*persistingArtifactStore,
	*aipipeline.SellerAIPipelineService,
	*countingRetryGlobalContextClient,
	int64,
) {
	t.Helper()
	sharedChunkPipelineEnv(t)
	gc := &countingRetryGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	store := &persistingArtifactStore{}
	svc := newSharedChunkPipelineService(t, store, aipipeline.NewDeterministicChunkAnalysisRunner())
	aipipeline.SetGlobalContextClientForTest(svc, gc)
	first, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	return store, svc, gc, first.PipelineRunID
}

func globalContextArtifactIndices(store *persistingArtifactStore, runID int64) []int {
	var out []int
	for i, art := range store.artifacts {
		if art.PipelineRunID == runID && art.Kind == aipipeline.ArtifactKindGlobalContextFinal {
			out = append(out, i)
		}
	}
	return out
}

func globalContextArtifactCount(store *persistingArtifactStore, runID int64) int {
	return len(globalContextArtifactIndices(store, runID))
}

func tamperGlobalContextArtifactPayload(art *dbgen.AiPipelineArtifact, mutate func(payload map[string]any)) {
	var doc map[string]any
	if err := json.Unmarshal(art.PayloadJson, &doc); err != nil {
		panic(err)
	}
	payload, _ := doc["payload"].(map[string]any)
	if payload == nil {
		panic("missing artifact payload")
	}
	mutate(payload)
	doc["payload"] = payload
	raw, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	art.PayloadJson = raw
}

func resumeSharedGlobalContextRun(t *testing.T, svc *aipipeline.SellerAIPipelineService, runID int64) {
	t.Helper()
	req := sharedChunkPipelineRequest()
	req.ExistingPipelineRunID = runID
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	if _, err := svc.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalContextArtifactCompatibilityMismatches(t *testing.T) {
	cases := []struct {
		name   string
		tamper gcArtifactTamper
	}{
		{
			name: "wrong_artifact_schema",
			tamper: func(t *testing.T, store *persistingArtifactStore, art *dbgen.AiPipelineArtifact) {
				tamperGlobalContextArtifactPayload(art, func(payload map[string]any) {
					payload["schema_version"] = "global_context_artifact_v0"
				})
			},
		},
		{
			name: "wrong_input_schema",
			tamper: func(t *testing.T, store *persistingArtifactStore, art *dbgen.AiPipelineArtifact) {
				tamperGlobalContextArtifactPayload(art, func(payload map[string]any) {
					payload["input_schema_version"] = aipipeline.GlobalContextInputSchemaVersionV1
				})
			},
		},
		{
			name: "wrong_seller_account_id",
			tamper: func(t *testing.T, store *persistingArtifactStore, art *dbgen.AiPipelineArtifact) {
				tamperGlobalContextArtifactPayload(art, func(payload map[string]any) {
					payload["seller_account_id"] = float64(999)
				})
			},
		},
		{
			name: "wrong_metric_date",
			tamper: func(t *testing.T, store *persistingArtifactStore, art *dbgen.AiPipelineArtifact) {
				tamperGlobalContextArtifactPayload(art, func(payload map[string]any) {
					payload["metric_date"] = "2099-01-01"
				})
			},
		},
		{
			name: "wrong_data_pack_hash",
			tamper: func(t *testing.T, store *persistingArtifactStore, art *dbgen.AiPipelineArtifact) {
				tamperGlobalContextArtifactPayload(art, func(payload map[string]any) {
					payload["data_pack_hash"] = "tampered-hash"
				})
			},
		},
		{
			name: "wrong_global_context_input_hash",
			tamper: func(t *testing.T, store *persistingArtifactStore, art *dbgen.AiPipelineArtifact) {
				tamperGlobalContextArtifactPayload(art, func(payload map[string]any) {
					payload["global_context_input_hash"] = "tampered-input-hash"
				})
			},
		},
		{
			name: "wrong_chunk_set_schema_version",
			tamper: func(t *testing.T, store *persistingArtifactStore, art *dbgen.AiPipelineArtifact) {
				tamperGlobalContextArtifactPayload(art, func(payload map[string]any) {
					payload["chunk_set_schema_version"] = "chunk_set_v0"
				})
			},
		},
		{
			name: "wrong_chunk_hash",
			tamper: func(t *testing.T, store *persistingArtifactStore, art *dbgen.AiPipelineArtifact) {
				tamperGlobalContextArtifactPayload(art, func(payload map[string]any) {
					hashes, _ := payload["chunk_hashes"].([]any)
					if len(hashes) == 0 {
						t.Fatal("expected chunk_hashes in artifact")
					}
					hashes[0] = "tampered-chunk-hash"
					payload["chunk_hashes"] = hashes
				})
			},
		},
		{
			name: "chunk_hashes_wrong_order",
			tamper: func(t *testing.T, store *persistingArtifactStore, art *dbgen.AiPipelineArtifact) {
				tamperGlobalContextArtifactPayload(art, func(payload map[string]any) {
					hashes, _ := payload["chunk_hashes"].([]any)
					if len(hashes) < 2 {
						t.Fatalf("expected >=2 chunk hashes, got %d", len(hashes))
					}
					payload["chunk_hashes"] = slices.Clone(hashes)
					slices.Reverse(payload["chunk_hashes"].([]any))
				})
			},
		},
		{
			name: "wrong_output_schema",
			tamper: func(t *testing.T, store *persistingArtifactStore, art *dbgen.AiPipelineArtifact) {
				tamperGlobalContextArtifactPayload(art, func(payload map[string]any) {
					gc, _ := payload["global_context"].(map[string]any)
					if gc == nil {
						t.Fatal("missing global_context payload")
					}
					gc["schema_version"] = "global_semantic_context_v0"
					payload["global_context"] = gc
				})
			},
		},
		{
			name: "corrupted_json",
			tamper: func(t *testing.T, store *persistingArtifactStore, art *dbgen.AiPipelineArtifact) {
				art.PayloadJson = []byte(`{"payload":`)
			},
		},
		{
			name: "restored_context_invalid",
			tamper: func(t *testing.T, store *persistingArtifactStore, art *dbgen.AiPipelineArtifact) {
				tamperGlobalContextArtifactPayload(art, func(payload map[string]any) {
					gc, _ := payload["global_context"].(map[string]any)
					if gc == nil {
						t.Fatal("missing global_context payload")
					}
					gc["source_fact_ids"] = []any{"fact.not.in.catalog"}
					payload["global_context"] = gc
				})
			},
		},
		{
			name: "artifact_document_hash_mismatch",
			tamper: func(t *testing.T, store *persistingArtifactStore, art *dbgen.AiPipelineArtifact) {
				art.Hash = pgtype.Text{String: "tampered-document-hash", Valid: true}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, svc, gc, runID := runSharedGlobalContextFirstPass(t)
			before := globalContextArtifactCount(store, runID)
			if before != 1 {
				t.Fatalf("expected 1 gc artifact before tamper, got %d", before)
			}
			idx := globalContextArtifactIndices(store, runID)[0]
			tc.tamper(t, store, &store.artifacts[idx])

			gc.generateCalls = 0
			gc.retryCalls = 0
			resumeSharedGlobalContextRun(t, svc, runID)

			if gc.generateCalls != 1 {
				t.Fatalf("generate=%d want 1", gc.generateCalls)
			}
			if gc.retryCalls != 0 {
				t.Fatalf("retry=%d want 0", gc.retryCalls)
			}
			after := globalContextArtifactCount(store, runID)
			if after != before+1 {
				t.Fatalf("gc artifacts before=%d after=%d want +1", before, after)
			}
			_ = store
		})
	}
}

func TestGlobalContextArtifactCrossRunIsolation(t *testing.T) {
	store, svc, gc, runID := runSharedGlobalContextFirstPass(t)
	if globalContextArtifactCount(store, runID) != 1 {
		t.Fatal("expected one gc artifact on first run")
	}

	// Copy a valid artifact to another pipeline run with matching seller/date/hash metadata.
	foreignRunID := runID + 1000
	srcIdx := globalContextArtifactIndices(store, runID)[0]
	foreign := store.artifacts[srcIdx]
	foreign.ID = int64(len(store.artifacts) + 1)
	foreign.PipelineRunID = foreignRunID
	store.artifacts = append(store.artifacts, foreign)

	gc.generateCalls = 0
	gc.retryCalls = 0

	// New run (not resume) must not reuse foreign-run artifact.
	if _, err := svc.Run(context.Background(), sharedChunkPipelineRequest()); err != nil {
		t.Fatal(err)
	}
	if gc.generateCalls != 1 {
		t.Fatalf("generate=%d want 1 for new run", gc.generateCalls)
	}
	if gc.retryCalls != 0 {
		t.Fatalf("retry=%d want 0", gc.retryCalls)
	}
}

func TestGlobalContextArtifactOrderingNewestCompatibleWins(t *testing.T) {
	store, svc, gc, runID := runSharedGlobalContextFirstPass(t)
	indices := globalContextArtifactIndices(store, runID)
	if len(indices) != 1 {
		t.Fatalf("expected 1 gc artifact, got %d", len(indices))
	}
	validArt := store.artifacts[indices[0]]

	buildStoredArtifact := func(id int64, mutate func(payload map[string]any)) dbgen.AiPipelineArtifact {
		art := validArt
		art.ID = id
		art.PayloadJson = slices.Clone(validArt.PayloadJson)
		tamperGlobalContextArtifactPayload(&art, mutate)
		return art
	}

	t.Run("old_incompatible_new_valid", func(t *testing.T) {
		localStore := &persistingArtifactStore{}
		localStore.artifacts = append([]dbgen.AiPipelineArtifact(nil), store.artifacts...)
		old := buildStoredArtifact(9001, func(payload map[string]any) {
			payload["global_context_input_hash"] = "stale"
		})
		old.CreatedAt = pgtype.Timestamptz{Time: time.Date(2026, 6, 25, 11, 0, 0, 0, time.UTC), Valid: true}
		newValid := validArt
		newValid.ID = 9002
		newValid.CreatedAt = pgtype.Timestamptz{Time: time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC), Valid: true}
		finalizeStoredArtifactHash(t, &newValid)
		localStore.artifacts = append(localStore.artifacts, old, newValid)

		localGC := &countingRetryGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
		localSvc := newSharedChunkPipelineService(t, localStore, aipipeline.NewDeterministicChunkAnalysisRunner())
		aipipeline.SetGlobalContextClientForTest(localSvc, localGC)

		resumeSharedGlobalContextRun(t, localSvc, runID)
		if localGC.generateCalls != 0 {
			t.Fatalf("generate=%d want 0", localGC.generateCalls)
		}
	})

	t.Run("old_valid_new_incompatible", func(t *testing.T) {
		localStore := &persistingArtifactStore{}
		localStore.artifacts = append([]dbgen.AiPipelineArtifact(nil), store.artifacts...)
		oldValid := validArt
		oldValid.ID = 9003
		oldValid.CreatedAt = pgtype.Timestamptz{Time: time.Date(2026, 6, 25, 11, 0, 0, 0, time.UTC), Valid: true}
		finalizeStoredArtifactHash(t, &oldValid)
		newIncompatible := buildStoredArtifact(9004, func(payload map[string]any) {
			payload["global_context_input_hash"] = "stale"
		})
		newIncompatible.CreatedAt = pgtype.Timestamptz{Time: time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC), Valid: true}
		localStore.artifacts = append(localStore.artifacts, oldValid, newIncompatible)

		localGC := &countingRetryGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
		localSvc := newSharedChunkPipelineService(t, localStore, aipipeline.NewDeterministicChunkAnalysisRunner())
		aipipeline.SetGlobalContextClientForTest(localSvc, localGC)

		resumeSharedGlobalContextRun(t, localSvc, runID)
		if localGC.generateCalls != 0 {
			t.Fatalf("generate=%d want 0", localGC.generateCalls)
		}
	})

	_ = svc
	_ = gc
}

type shuffledArtifactStore struct {
	persistingArtifactStore
	shuffle func([]dbgen.AiPipelineArtifact) []dbgen.AiPipelineArtifact
}

func (s *shuffledArtifactStore) ListArtifacts(ctx context.Context, pipelineRunID int64) ([]dbgen.AiPipelineArtifact, error) {
	arts, err := s.persistingArtifactStore.ListArtifacts(ctx, pipelineRunID)
	if err != nil {
		return nil, err
	}
	if s.shuffle != nil {
		arts = s.shuffle(arts)
	}
	return arts, nil
}

func replaceGlobalContextArtifacts(store *persistingArtifactStore, runID int64, arts ...dbgen.AiPipelineArtifact) {
	filtered := store.artifacts[:0]
	for _, art := range store.artifacts {
		if art.PipelineRunID == runID && art.Kind == aipipeline.ArtifactKindGlobalContextFinal {
			continue
		}
		filtered = append(filtered, art)
	}
	store.artifacts = append(filtered, arts...)
}

func artifactDocumentHash(t *testing.T, art dbgen.AiPipelineArtifact) string {
	t.Helper()
	hash, err := aipipeline.PipelineArtifactDocumentHashForTest(art.PayloadJson)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func finalizeStoredArtifactHash(t *testing.T, art *dbgen.AiPipelineArtifact) {
	t.Helper()
	hash, err := aipipeline.PipelineArtifactDocumentHashForTest(art.PayloadJson)
	if err != nil {
		t.Fatal(err)
	}
	art.Hash = pgtype.Text{String: hash, Valid: true}
}

func withArtifactIdentity(art dbgen.AiPipelineArtifact, id int64, createdAt time.Time) dbgen.AiPipelineArtifact {
	art.ID = id
	art.CreatedAt = pgtype.Timestamptz{Time: createdAt.UTC(), Valid: true}
	return art
}

func TestGlobalContextArtifactOrderingShuffledStore(t *testing.T) {
	baseTime := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	store, _, _, runID := runSharedGlobalContextFirstPass(t)
	indices := globalContextArtifactIndices(store, runID)
	if len(indices) != 1 {
		t.Fatalf("expected 1 gc artifact, got %d", len(indices))
	}
	template := store.artifacts[indices[0]]

	buildValidArtifact := func(id int64, createdAt time.Time, story string) dbgen.AiPipelineArtifact {
		art := template
		art.PayloadJson = slices.Clone(template.PayloadJson)
		tamperGlobalContextArtifactPayload(&art, func(payload map[string]any) {
			gc, _ := payload["global_context"].(map[string]any)
			if gc == nil {
				t.Fatal("missing global_context")
			}
			gc["overall_story"] = story
			payload["global_context"] = gc
		})
		art = withArtifactIdentity(art, id, createdAt)
		finalizeStoredArtifactHash(t, &art)
		return art
	}

	shuffleOrders := []func([]dbgen.AiPipelineArtifact) []dbgen.AiPipelineArtifact{
		func(in []dbgen.AiPipelineArtifact) []dbgen.AiPipelineArtifact {
			return slices.Clone(in)
		},
		func(in []dbgen.AiPipelineArtifact) []dbgen.AiPipelineArtifact {
			out := slices.Clone(in)
			slices.Reverse(out)
			return out
		},
		func(in []dbgen.AiPipelineArtifact) []dbgen.AiPipelineArtifact {
			if len(in) < 2 {
				return in
			}
			out := slices.Clone(in)
			out[0], out[len(out)-1] = out[len(out)-1], out[0]
			return out
		},
	}

	t.Run("old_valid_new_valid_shuffled", func(t *testing.T) {
		oldArt := buildValidArtifact(100, baseTime, "OLD_STORY")
		newArt := buildValidArtifact(200, baseTime.Add(time.Minute), "NEW_STORY")
		newHash := artifactDocumentHash(t, newArt)

		for i, shuffle := range shuffleOrders {
			t.Run(shuffleOrderName(i), func(t *testing.T) {
				localStore := &shuffledArtifactStore{
					persistingArtifactStore: persistingArtifactStore{artifacts: slices.Clone(store.artifacts)},
					shuffle:                 shuffle,
				}
				replaceGlobalContextArtifacts(&localStore.persistingArtifactStore, runID, oldArt, newArt)

				localGC := &countingRetryGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
				localSvc := newSharedChunkPipelineService(t, localStore, aipipeline.NewDeterministicChunkAnalysisRunner())
				aipipeline.SetGlobalContextClientForTest(localSvc, localGC)

				req := sharedChunkPipelineRequest()
				req.ExistingPipelineRunID = runID
				req.DispatchID = 1
				req.LeaseGuard = noopLeaseGuard{}
				res, err := localSvc.Run(context.Background(), req)
				if err != nil {
					t.Fatal(err)
				}
				if localGC.generateCalls != 0 {
					t.Fatalf("shuffle=%d generate=%d want 0", i, localGC.generateCalls)
				}
				if res.Diagnostics == nil || res.Diagnostics.GlobalContext.ArtifactHash != newHash {
					t.Fatalf("shuffle=%d artifact_hash=%q want %q", i, res.Diagnostics.GlobalContext.ArtifactHash, newHash)
				}
			})
		}
	})

	t.Run("old_valid_new_incompatible_shuffled", func(t *testing.T) {
		oldArt := buildValidArtifact(300, baseTime, "OLD_STORY_KEEP")
		newIncompatible := withArtifactIdentity(template, 400, baseTime.Add(2*time.Minute))
		newIncompatible.PayloadJson = slices.Clone(template.PayloadJson)
		tamperGlobalContextArtifactPayload(&newIncompatible, func(payload map[string]any) {
			payload["global_context_input_hash"] = "stale-input-hash"
		})
		finalizeStoredArtifactHash(t, &newIncompatible)
		oldHash := artifactDocumentHash(t, oldArt)

		for i, shuffle := range shuffleOrders {
			t.Run(shuffleOrderName(i), func(t *testing.T) {
				localStore := &shuffledArtifactStore{
					persistingArtifactStore: persistingArtifactStore{artifacts: slices.Clone(store.artifacts)},
					shuffle:                 shuffle,
				}
				replaceGlobalContextArtifacts(&localStore.persistingArtifactStore, runID, oldArt, newIncompatible)

				localGC := &countingRetryGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
				localSvc := newSharedChunkPipelineService(t, localStore, aipipeline.NewDeterministicChunkAnalysisRunner())
				aipipeline.SetGlobalContextClientForTest(localSvc, localGC)

				res, err := localSvc.Run(context.Background(), func() aipipeline.RunSellerAIPipelineRequest {
					req := sharedChunkPipelineRequest()
					req.ExistingPipelineRunID = runID
					req.DispatchID = 1
					req.LeaseGuard = noopLeaseGuard{}
					return req
				}())
				if err != nil {
					t.Fatal(err)
				}
				if localGC.generateCalls != 0 {
					t.Fatalf("shuffle=%d generate=%d want 0", i, localGC.generateCalls)
				}
				if res.Diagnostics == nil || res.Diagnostics.GlobalContext.ArtifactHash != oldHash {
					t.Fatalf("shuffle=%d artifact_hash=%q want %q", i, res.Diagnostics.GlobalContext.ArtifactHash, oldHash)
				}
			})
		}
	})
}

func shuffleOrderName(i int) string {
	switch i {
	case 0:
		return "append_order"
	case 1:
		return "reversed"
	default:
		return "swapped_ends"
	}
}
