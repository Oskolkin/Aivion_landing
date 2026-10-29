package replay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/eval"
)

// StartEvaluationRun creates an analysis run that cannot publish.
func StartEvaluationRun(ctx context.Context, repo *analysispg.Repository, in analysispg.Run) (analysispg.Run, error) {
	if in.Kind == "" {
		in.Kind = "EVALUATION"
	}
	return repo.CreateEvaluationRun(ctx, in)
}

func AssertNoPublish(ctx context.Context, repo *analysispg.Repository, sellerID int64, before *int64) error {
	after, err := repo.CurrentPublicationID(ctx, sellerID)
	if err != nil {
		return err
	}
	if (before == nil) != (after == nil) {
		return fmt.Errorf("EVALUATION mutated current publication pointer")
	}
	if before != nil && after != nil && *before != *after {
		return fmt.Errorf("EVALUATION mutated current publication pointer")
	}
	return nil
}

func HashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func DeterminismCheck(a, b []byte) error {
	if HashBytes(a) != HashBytes(b) {
		return fmt.Errorf(eval.DeterminismViolation)
	}
	return nil
}
