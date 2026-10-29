package main

import (
	"testing"
)

func TestValidateRunAIPipelineFlagsRestartRequiresSource(t *testing.T) {
	if code := runAIPipeline([]string{"--restart-from-stage=actions"}); code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
}

func TestValidateRunAIPipelineFlagsRestartUnsupportedStage(t *testing.T) {
	if code := runAIPipeline([]string{"--resume-run-id=70", "--restart-from-stage=dashboard"}); code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
}

func TestValidateRunAIPipelineFlagsRestartAndForce(t *testing.T) {
	if code := runAIPipeline([]string{"--resume-run-id=70", "--restart-from-stage=actions", "--force=true"}); code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
}

func TestValidateRunAIPipelineFlagsRestartActionsDisabled(t *testing.T) {
	if code := runAIPipeline([]string{"--resume-run-id=70", "--restart-from-stage=actions", "--actions=false"}); code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
}

func TestValidateRunAIPipelineFlagsRestartDashboardDisabled(t *testing.T) {
	if code := runAIPipeline([]string{"--resume-run-id=70", "--restart-from-stage=actions", "--dashboard=false"}); code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
}
