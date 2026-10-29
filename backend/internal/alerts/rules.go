package alerts

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type RuleContext struct {
	SellerAccountID int64
	NowUnix         int64
}

type Rule interface {
	Name() string
	Evaluate(ctx RuleContext) ([]RuleResult, error)
}

func RuleResultToUpsertInput(sellerAccountID int64, result RuleResult) UpsertAlertInput {
	fingerprint := strings.TrimSpace(result.Fingerprint)
	if fingerprint == "" {
		fingerprint = BuildFingerprint(
			sellerAccountID,
			result.AlertType,
			result.EntityType,
			BuildEntityIdentity(result.EntityType, result.EntityID, result.EntitySKU, result.EntityOfferID),
		)
	}
	return UpsertAlertInput{
		SellerAccountID: sellerAccountID,
		AlertType:       result.AlertType,
		AlertGroup:      result.AlertGroup,
		EntityType:      result.EntityType,
		EntityID:        result.EntityID,
		EntitySKU:       result.EntitySKU,
		EntityOfferID:   result.EntityOfferID,
		Title:           result.Title,
		Message:         result.Message,
		Severity:        result.Severity,
		Urgency:         result.Urgency,
		EvidencePayload: normalizeEvidence(result.EvidencePayload),
		Fingerprint:     fingerprint,
	}
}

func BuildEntityIdentity(entityType EntityType, entityID *string, entitySKU *int64, entityOfferID *string) string {
	parts := []string{string(entityType)}
	if entityID != nil && strings.TrimSpace(*entityID) != "" {
		parts = append(parts, "id:"+strings.TrimSpace(*entityID))
	}
	if entitySKU != nil {
		parts = append(parts, fmt.Sprintf("sku:%d", *entitySKU))
	}
	if entityOfferID != nil && strings.TrimSpace(*entityOfferID) != "" {
		parts = append(parts, "offer:"+strings.TrimSpace(*entityOfferID))
	}
	if len(parts) == 1 {
		parts = append(parts, "global")
	}
	return strings.Join(parts, "|")
}

func BuildFingerprint(sellerAccountID int64, alertType AlertType, entityType EntityType, entityIdentity string) string {
	key := fmt.Sprintf("%d|%s|%s|%s",
		sellerAccountID,
		strings.TrimSpace(string(alertType)),
		strings.TrimSpace(string(entityType)),
		strings.TrimSpace(entityIdentity),
	)
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
