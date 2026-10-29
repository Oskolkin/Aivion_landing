package actions

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
)

func guardActionsAIFirstInputTokens(input ActionsAIFirstInput) (estimated int, limit int, err error) {
	wireBytes, err := MarshalActionsAIFirstInputWire(input)
	if err != nil {
		return 0, 0, &ActionsGenerationError{
			Code: ActionsErrorInputTokenEstimationFailed,
			Err:  fmt.Errorf("marshal wire input: %w", err),
		}
	}
	estimated = aitokens.EstimateBytesTokens(wireBytes)
	limit = aitokens.MaxInputTokensPerRequest()
	if estimated > limit {
		return estimated, limit, &ActionsGenerationError{
			Code: ActionsErrorInputTooLarge,
			Err:  fmt.Errorf("estimated input tokens %d exceed limit %d", estimated, limit),
		}
	}
	return estimated, limit, nil
}
