package actions

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrActionNotFound = errors.New("action not found")
var ErrEntityGroupNotFound = errors.New("action entity group not found")
var ErrGenerationUnavailable = errors.New("actions generation unavailable")
var ErrInvalidSellerID = errors.New("invalid seller_id")

var ErrInvalidMetricDate = errors.New("invalid metric_date")
var ErrUnsupportedExportFormat = errors.New("unsupported export format")
var ErrUnsupportedActionExport = errors.New("unsupported action export")

var actionKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9._:-]+$`)

func ValidateActionKey(actionKey string) error {
	key := strings.TrimSpace(actionKey)
	if key == "" {
		return fmt.Errorf("action_key is required")
	}
	if len(key) > 200 {
		return fmt.Errorf("action_key is too long")
	}
	if strings.Contains(key, "/") {
		return fmt.Errorf("action_key must not contain '/'")
	}
	if !actionKeyPattern.MatchString(key) {
		return fmt.Errorf("action_key contains invalid characters")
	}
	return nil
}
