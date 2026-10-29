package aidatapack

import (
	"regexp"
	"strings"
)

const OrderDeclineGroundingMismatchCode = "order_decline_grounding_mismatch"

var orderDeclinePercentClaimPattern = regexp.MustCompile(`(?i)(заказ\w*|orders?).{0,80}(снизил\w*|упал\w*|уменьшил\w*|declin\w*).{0,40}\d`)

// NumericalClaimIssue flags AI numeric claims without matching canonical fact grounding.
type NumericalClaimIssue struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Details string `json:"details"`
}

// DetectOrderDeclinePercentClaim reports order-decline percentage wording in text.
func DetectOrderDeclinePercentClaim(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if !strings.Contains(strings.ToLower(text), "заказ") && !strings.Contains(strings.ToLower(text), "order") {
		return false
	}
	return orderDeclinePercentClaimPattern.MatchString(text)
}

// HasOrderDeclineGrounding checks evidence facts support an order-decline claim.
func HasOrderDeclineGrounding(factIDs []string) bool {
	hasDelta := false
	hasLast := false
	hasPrevious := false
	for _, id := range factIDs {
		id = strings.TrimSpace(id)
		switch id {
		case "account.orders.last_7d_vs_previous_7d_delta_pct",
			"account.orders.last_7d_vs_previous_7d_delta_abs":
			hasDelta = true
		case "account.last_7d.orders":
			hasLast = true
		case "account.previous_7d.orders":
			hasPrevious = true
		}
	}
	return hasDelta || (hasLast && hasPrevious)
}

// ValidateOrderDeclineGrounding returns a warning when order-decline text lacks order facts.
func ValidateOrderDeclineGrounding(text string, factIDs []string, field string) []NumericalClaimIssue {
	if !DetectOrderDeclinePercentClaim(text) {
		return nil
	}
	if HasOrderDeclineGrounding(factIDs) {
		return nil
	}
	return []NumericalClaimIssue{{
		Code:    OrderDeclineGroundingMismatchCode,
		Field:   field,
		Details: "order decline claim requires account order delta or period facts",
	}}
}
