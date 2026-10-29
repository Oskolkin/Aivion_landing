package prompt

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/lineendings"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

// RenderedPrompt is the fully resolved (instructions, input) pair for one
// provider run, together with the version identifier used to store it and
// the hash used to detect drift.
type RenderedPrompt struct {
	Profile      PromptProfile
	Version      string
	Instructions string
	Input        string
	Hash         string
}

// Render resolves a PromptProfile into a RenderedPrompt.
//
// PromptProfileSmoke reuses the stage-3 technical smoke instructions/input
// from the provider package (provider.SmokeInstructions / provider.SmokeInput)
// under provider.PromptVersionSmokeV1.
//
// PromptProfileAnalysis renders the production Aivion Unified Analyst
// instructions/input (AnalystV1Instructions / AnalystV1Input) under
// VersionAnalystV1.
func Render(profile PromptProfile) (RenderedPrompt, error) {
	switch profile {
	case PromptProfileSmoke:
		instructions := lineendings.NormalizeLineEndings(provider.SmokeInstructions())
		input := lineendings.NormalizeLineEndings(provider.SmokeInput())
		return RenderedPrompt{
			Profile:      profile,
			Version:      provider.PromptVersionSmokeV1,
			Instructions: instructions,
			Input:        input,
			Hash:         Hash(instructions, input),
		}, nil
	case PromptProfileAnalysis:
		rendered, err := RenderByVersion(CurrentAnalystVersion)
		if err != nil {
			return RenderedPrompt{}, err
		}
		rendered.Profile = profile
		return rendered, nil
	default:
		return RenderedPrompt{}, fmt.Errorf("prompt: unknown profile %q", profile)
	}
}
