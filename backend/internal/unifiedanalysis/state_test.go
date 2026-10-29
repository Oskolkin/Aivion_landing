package unifiedanalysis

import "testing"

func TestCanTransitionUnifiedAnalysis(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{StatusCreated, StatusPackBuilding, true},
		{StatusPackBuilding, StatusPackValidating, true},
		{StatusPackValidating, StatusPackReady, true},
		{StatusPackReady, StatusProviderContainerCreating, true},
		{StatusProviderRunning, StatusProviderCompleted, true},
		{StatusProviderCompleted, StatusOutputValidating, true},
		{StatusOutputValidating, StatusPublicationReady, true},
		{StatusOutputValidating, StatusPublicationBlocked, true},
		{StatusProviderCompleted, StatusCleanupPending, false},
		{StatusFailed, StatusPackBuilding, false},
		{StatusCancelled, StatusPackBuilding, false},
		{StatusPublicationReady, StatusPackBuilding, false},
		{StatusCreated, StatusCreated, true},
		{"legacy", StatusPackBuilding, false},
		// Stage 5 publication lifecycle: publication_ready is wait-terminal (see
		// TestIsWaitTerminal) but must still be able to advance through publishing
		// to published/publication_failed.
		{StatusPublicationReady, StatusPublishing, true},
		{StatusPublicationReady, StatusPublished, true},
		{StatusPublicationReady, StatusPublicationFailed, true},
		{StatusPublishing, StatusPublished, true},
		{StatusPublishing, StatusPublicationFailed, true},
		{StatusPublishing, StatusPackBuilding, false},
		{StatusPublished, StatusPublishing, false},
		{StatusPublicationFailed, StatusPublishing, false},
	}
	for _, tc := range cases {
		got := CanTransition(ModeUnifiedAnalysis, tc.from, tc.to)
		if got != tc.want {
			t.Fatalf("CanTransition(%q -> %q) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestIsWaitTerminal(t *testing.T) {
	if !IsWaitTerminal(StatusProviderCompleted, false) {
		t.Fatal("smoke should wait-terminal on provider_completed")
	}
	if IsWaitTerminal(StatusProviderCompleted, true) {
		t.Fatal("analysis should not wait-terminal on provider_completed")
	}
	if !IsWaitTerminal(StatusPublicationReady, true) {
		t.Fatal("analysis should wait-terminal on publication_ready")
	}
	if !IsWaitTerminal(StatusPublished, true) {
		t.Fatal("analysis should wait-terminal on published")
	}
	if !IsWaitTerminal(StatusPublicationBlocked, true) {
		t.Fatal("analysis should wait-terminal on publication_blocked")
	}
}

func TestIsActiveUnifiedRunStatus_PublicationStatuses(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{StatusPublicationReady, false},
		{StatusPublishing, false},
		{StatusPublished, false},
		{StatusPublicationFailed, false},
		{StatusPublicationBlocked, false},
		{StatusFailed, false},
		{StatusCancelled, false},
		{StatusCleanupCompleted, false},
		{StatusProviderCompleted, false},
		{StatusCreated, true},
		{StatusOutputValidating, true},
	}
	for _, tc := range cases {
		got := IsActiveUnifiedRunStatus(tc.status)
		if got != tc.want {
			t.Fatalf("IsActiveUnifiedRunStatus(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestIsTerminalRunStatus_PublicationStatuses(t *testing.T) {
	if IsTerminalRunStatus(StatusPublicationReady) {
		t.Fatal("publication_ready must not be hard-terminal: it still needs to transition to publishing/published/publication_failed")
	}
	if IsTerminalRunStatus(StatusPublishing) {
		t.Fatal("publishing must not be hard-terminal")
	}
	if !IsTerminalRunStatus(StatusPublished) {
		t.Fatal("published must be hard-terminal")
	}
	if !IsTerminalRunStatus(StatusPublicationFailed) {
		t.Fatal("publication_failed must be hard-terminal")
	}
}
