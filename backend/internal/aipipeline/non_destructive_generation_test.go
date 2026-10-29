package aipipeline_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/publishedsnapshot"
)

func TestForceRerunCreatesNewPipelineRun(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	tracking := &pipelineTrackingStore{fakeStore: fakeStore{}}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		tracking,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})

	req := basePipelineRequest()
	req.ForceRegenerate = true
	res1, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res1.PipelineRunID == 0 || res2.PipelineRunID == 0 {
		t.Fatalf("expected pipeline ids, got %d and %d", res1.PipelineRunID, res2.PipelineRunID)
	}
	if res1.PipelineRunID == res2.PipelineRunID {
		t.Fatalf("force rerun must create a new pipeline run")
	}
	if tracking.createCalls < 2 {
		t.Fatalf("expected at least 2 pipeline creates, got %d", tracking.createCalls)
	}
}

func TestFailedPipelineDoesNotPublishSnapshot(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	publisher := &recordingSnapshotPublisher{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		&fakeDashboardRunner{fail: true},
		&fakeActionsRunner{},
		&fakeStore{},
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(publisher)

	res, err := svc.Run(context.Background(), basePipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Published {
		t.Fatal("failed dashboard must not publish")
	}
	if publisher.calls != 0 {
		t.Fatalf("publisher must not be called, got %d", publisher.calls)
	}
}

func TestPublicationFailureDoesNotChangePreviousSnapshot(t *testing.T) {
	prev := publishedsnapshot.Snapshot{
		PipelineRunID:  11,
		DashboardRunID: "d1",
		ActionsRunID:   21,
		PublishedAt:    time.Now().UTC().Add(-time.Hour),
	}
	publisher := &failingSnapshotPublisher{previous: prev}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		&fakeStore{},
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(publisher)

	res, err := svc.Run(context.Background(), basePipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Published {
		t.Fatal("publication failure must not mark pipeline published")
	}
	if publisher.calls != 1 {
		t.Fatalf("expected one publication attempt, got %d", publisher.calls)
	}
}

func TestEmptyActionsAllowsSuccessfulPipelineCompletion(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		&fakeDashboardRunner{},
		&fakeActionsRunner{empty: true},
		&fakeStore{},
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})

	res, err := svc.Run(context.Background(), basePipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != aipipeline.PipelineStatusCompleted {
		t.Fatalf("status=%s", res.Status)
	}
	if res.ActionsStatus != "empty" {
		t.Fatalf("actions_status=%s", res.ActionsStatus)
	}
	if !res.Published {
		t.Fatal("empty actions with completed dashboard should publish")
	}
}

func TestDevResetForbiddenInProduction(t *testing.T) {
	t.Setenv("ALLOW_DEV_SEED", "1")
	t.Setenv("APP_ENV", "production")
	if err := devseed.AssertDevSeedAllowed(); err == nil {
		t.Fatal("expected production dev reset block")
	}
}

type pipelineTrackingStore struct {
	fakeStore
	createCalls int
}

func (s *pipelineTrackingStore) CreatePipelineRun(ctx context.Context, params aipipeline.CreatePipelineRunParams) (int64, error) {
	s.createCalls++
	return s.fakeStore.CreatePipelineRun(ctx, params)
}

type recordingSnapshotPublisher struct {
	calls int
}

func (p *recordingSnapshotPublisher) PublishPipelineSnapshot(ctx context.Context, pipelineRunID int64) (aipipeline.PublishResult, error) {
	p.calls++
	return aipipeline.PublishResult{
		Snapshot:  publishedsnapshot.Snapshot{PipelineRunID: pipelineRunID, PublishedAt: time.Now().UTC()},
		IsCurrent: true,
	}, nil
}

type failingSnapshotPublisher struct {
	previous publishedsnapshot.Snapshot
	calls    int
}

func (p *failingSnapshotPublisher) PublishPipelineSnapshot(ctx context.Context, pipelineRunID int64) (aipipeline.PublishResult, error) {
	p.calls++
	return aipipeline.PublishResult{Snapshot: p.previous}, errors.New("publication_failed")
}
