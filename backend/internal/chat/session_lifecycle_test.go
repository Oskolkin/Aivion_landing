package chat

import (
	"context"
	"testing"
)

func TestBuildChatTitle(t *testing.T) {
	got := BuildChatTitle("Разберите 8 SKU", "ignored question", "fallback")
	if got != "Разберите 8 SKU" {
		t.Fatalf("got %q", got)
	}
	got = BuildChatTitle("", "Сделай ABC-анализ", "fallback")
	if got != "Сделай ABC-анализ" {
		t.Fatalf("got %q", got)
	}
}

func TestValidateInputAttachmentFile(t *testing.T) {
	if err := validateInputAttachmentFile("report.pdf", "application/pdf", 1024); err != nil {
		t.Fatalf("expected pdf ok: %v", err)
	}
	if err := validateInputAttachmentFile("bad.exe", "application/octet-stream", 1024); err == nil {
		t.Fatal("expected exe rejection")
	}
	if err := validateInputAttachmentFile("big.pdf", "application/pdf", chatAttachmentMaxFileBytes()+1); err == nil {
		t.Fatal("expected size rejection")
	}
}

func TestGetSessionByActionKeyDoesNotCreate(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	before := len(repo.sessions)
	got, err := svc.GetSessionByActionKey(context.Background(), 1, "action:1")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil session, got %+v", got)
	}
	if len(repo.sessions) != before {
		t.Fatal("lookup must not create session")
	}
}

func TestResolveSessionActionUnique(t *testing.T) {
	repo := newFakeRepo()
	svc, err := NewServiceWithDeps(ServiceDeps{
		Repo:         repo,
		AIClient:     &fakeAIClient{},
		ToolExecutor: &fakeToolExecutor{},
	})
	if err != nil {
		t.Fatal(err)
	}
	key := "action:stock:1"
	first, created, err := svc.resolveSession(context.Background(), AskInput{
		SellerAccountID: 1,
		ActionKey:       key,
	}, "question one")
	if err != nil || !created || first == nil {
		t.Fatalf("first create failed: created=%v err=%v", created, err)
	}
	second, created2, err := svc.resolveSession(context.Background(), AskInput{
		SellerAccountID: 1,
		ActionKey:       key,
	}, "question two")
	if err != nil || created2 || second == nil {
		t.Fatalf("second resolve failed: created=%v err=%v", created2, err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected same session id, got %d and %d", first.ID, second.ID)
	}
}
