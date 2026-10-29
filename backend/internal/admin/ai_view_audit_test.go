package admin

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestViewRawAIRequiresAdminActor(t *testing.T) {
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			getRecommendationRawAIFn: func(ctx context.Context, sellerAccountID, recommendationID int64) (*RecommendationRawAI, error) {
				t.Fatalf("repository fetch should not run without actor")
				return nil, nil
			},
		},
	})
	_, err := svc.GetRecommendationRawAI(context.Background(), AdminActor{}, 5, 44)
	if !errors.Is(err, ErrAdminActorRequired) {
		t.Fatalf("expected ErrAdminActorRequired, got %v", err)
	}
}

func TestViewRawAIRawSuccessWritesAuditLog(t *testing.T) {
	var created CreateAdminActionLogInput
	var completed CompleteAdminActionLogInput
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			createActionFn: func(ctx context.Context, input CreateAdminActionLogInput) (*AdminActionLog, error) {
				created = input
				return &AdminActionLog{ID: 501, Status: AdminActionStatusRunning}, nil
			},
			completeActionFn: func(ctx context.Context, input CompleteAdminActionLogInput) (*AdminActionLog, error) {
				completed = input
				return &AdminActionLog{ID: input.ID, Status: AdminActionStatusCompleted}, nil
			},
			getRecommendationRawAIFn: func(ctx context.Context, sellerAccountID, recommendationID int64) (*RecommendationRawAI, error) {
				return &RecommendationRawAI{
					Recommendation: RecommendationItem{ID: recommendationID},
					RelatedAlerts:  []RecommendationAlertItem{{ID: 1}},
					Diagnostics:    []RecommendationRunDiagnosticItem{{ID: 2}},
				}, nil
			},
		},
	})
	_, err := svc.GetRecommendationRawAI(context.Background(), testAdminActor(), 5, 44)
	if err != nil {
		t.Fatal(err)
	}
	if created.ActionType != AdminActionViewRawAIPayload {
		t.Fatalf("action type = %q, want %q", created.ActionType, AdminActionViewRawAIPayload)
	}
	if created.TargetType == nil || *created.TargetType != AdminRawViewTargetRecommendation {
		t.Fatalf("unexpected target_type: %+v", created.TargetType)
	}
	if created.TargetID == nil || *created.TargetID != 44 {
		t.Fatalf("unexpected target_id: %+v", created.TargetID)
	}
	if created.SellerAccountID != 5 {
		t.Fatalf("seller_account_id = %d, want 5", created.SellerAccountID)
	}
	allowedReq := map[string]struct{}{
		"target_type":       {},
		"target_id":         {},
		"seller_account_id": {},
		"ai_model":          {},
		"ai_prompt_version": {},
	}
	for k := range created.RequestPayload {
		if _, ok := allowedReq[k]; !ok {
			t.Fatalf("unexpected audit request_payload key %q", k)
		}
	}
	for _, forbidden := range []string{"raw_ai_response", "raw_openai_response", "prompt", "context", "messages"} {
		if _, bad := created.RequestPayload[forbidden]; bad {
			t.Fatalf("request_payload must not contain %q", forbidden)
		}
	}
	for k, v := range completed.ResultPayload {
		if s, ok := v.(string); ok && strings.Contains(strings.ToLower(s), "sk-") {
			t.Fatalf("result_payload key %q looks like a secret carrier", k)
		}
	}
	if _, bad := completed.ResultPayload["raw_ai_response"]; bad {
		t.Fatalf("result_payload must not include raw_ai_response")
	}
}

func TestViewRawAICompleteFailureBlocksPayload(t *testing.T) {
	fetchCalled := false
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			createActionFn: func(ctx context.Context, input CreateAdminActionLogInput) (*AdminActionLog, error) {
				return &AdminActionLog{ID: 88, Status: AdminActionStatusRunning}, nil
			},
			completeActionFn: func(ctx context.Context, input CompleteAdminActionLogInput) (*AdminActionLog, error) {
				return nil, errors.New("complete failed")
			},
			failActionFn: func(ctx context.Context, input FailAdminActionLogInput) (*AdminActionLog, error) {
				return &AdminActionLog{ID: input.ID, Status: AdminActionStatusFailed}, nil
			},
			getRecommendationRawAIFn: func(ctx context.Context, sellerAccountID, recommendationID int64) (*RecommendationRawAI, error) {
				fetchCalled = true
				return &RecommendationRawAI{Recommendation: RecommendationItem{ID: recommendationID}}, nil
			},
		},
	})
	_, err := svc.GetRecommendationRawAI(context.Background(), testAdminActor(), 5, 44)
	if !errors.Is(err, ErrAdminAuditLogWriteFailed) {
		t.Fatalf("expected ErrAdminAuditLogWriteFailed, got %v", err)
	}
	if !fetchCalled {
		t.Fatalf("expected repository fetch to run before complete failure")
	}
}

func TestViewRawAICreateFailureBlocksPayload(t *testing.T) {
	fetchCalled := false
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			createActionFn: func(ctx context.Context, input CreateAdminActionLogInput) (*AdminActionLog, error) {
				return nil, errors.New("create failed")
			},
			getRecommendationRawAIFn: func(ctx context.Context, sellerAccountID, recommendationID int64) (*RecommendationRawAI, error) {
				fetchCalled = true
				return &RecommendationRawAI{Recommendation: RecommendationItem{ID: recommendationID}}, nil
			},
		},
	})
	_, err := svc.GetRecommendationRawAI(context.Background(), testAdminActor(), 5, 44)
	if !errors.Is(err, ErrAdminAuditLogWriteFailed) {
		t.Fatalf("expected ErrAdminAuditLogWriteFailed, got %v", err)
	}
	if fetchCalled {
		t.Fatalf("expected repository fetch to be skipped when audit create fails")
	}
}

func TestViewRawAIFetchFailureDoesNotWrapAuditError(t *testing.T) {
	svc, _ := NewService(ServiceDeps{
		Repo: &fakeRepo{
			getRecommendationRawAIFn: func(ctx context.Context, sellerAccountID, recommendationID int64) (*RecommendationRawAI, error) {
				return nil, pgx.ErrNoRows
			},
		},
	})
	_, err := svc.GetRecommendationRawAI(context.Background(), testAdminActor(), 5, 44)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
	if errors.Is(err, ErrAdminAuditLogWriteFailed) {
		t.Fatal("fetch failure must not be reported as audit write failure")
	}
}
