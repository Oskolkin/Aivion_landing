package chatv2

import (
	"errors"
	"fmt"
	"testing"
)

func TestExportErrorWrappingPreservesSentinel(t *testing.T) {
	err := fmt.Errorf("export attachment lookup: %w", ErrExportForbidden)
	if !errors.Is(err, ErrExportForbidden) {
		t.Fatal("errors.Is must preserve ErrExportForbidden")
	}
	err = fmt.Errorf("finalize.owner_fence: %w", ErrIdempotencyOwnershipLost)
	if !errors.Is(err, ErrIdempotencyOwnershipLost) {
		t.Fatal("errors.Is must preserve ErrIdempotencyOwnershipLost")
	}
}
