package aipipeline

import (
	"os"
	"testing"
)

// pipelineAmbientEnvKeys lists environment variables that change pipeline
// behavior (feature flags, provider mode, credentials) but that individual
// tests are expected to control explicitly via t.Setenv. If one of these
// leaks in from the invoking shell (for example a locally exported
// AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED=true left over from pilot
// testing, since docker-compose defaults it to true), tests that rely on
// the documented package defaults fail nondeterministically depending on
// who/where `go test` is invoked, even though nothing in the package
// changed.
var pipelineAmbientEnvKeys = []string{
	"AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED",
	"AI_PROVIDER_MODE",
	"OPENAI_API_KEY",
	"SELLER_AI_PIPELINE_ENABLED",
	"ACTIONS_AI_FIRST",
	"LEGACY_AI_POST_SYNC_ENABLED",
	"ALLOW_DEV_AI_PIPELINE",
}

// TestMain clears ambient environment variables that influence pipeline
// behavior before running the suite so results are hermetic regardless of
// what happens to be exported in the invoking shell. Individual tests
// remain free to opt back in via t.Setenv, which Go restores automatically
// once each test (and its subtests) complete.
func TestMain(m *testing.M) {
	for _, key := range pipelineAmbientEnvKeys {
		os.Unsetenv(key)
	}
	os.Exit(m.Run())
}
