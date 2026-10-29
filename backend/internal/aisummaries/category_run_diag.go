package aisummaries

import (
	"fmt"
	"log"
	"strings"
)

// categoryFailureKind classifies why a category summary did not complete.
type categoryFailureKind string

const (
	categoryFailLoadSKUs       categoryFailureKind = "load_skus_error"
	categoryFailSave           categoryFailureKind = "save_error"
	categoryFailOpenAI         categoryFailureKind = "openai_error"
	categoryFailValidationJSON categoryFailureKind = "validation_error"
	categoryFailValidation     categoryFailureKind = "validation_failed"
)

type categoryFailure struct {
	CategoryID   int64
	CategoryName string
	Kind         categoryFailureKind
	Reason       string
}

type categoryRunDiagnostics struct {
	sellerID int64
	failures []categoryFailure
}

func newCategoryRunDiagnostics(sellerID int64) *categoryRunDiagnostics {
	return &categoryRunDiagnostics{sellerID: sellerID}
}

func (d *categoryRunDiagnostics) record(cid int64, name string, kind categoryFailureKind, reason string) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = string(kind)
	}
	f := categoryFailure{
		CategoryID:   cid,
		CategoryName: name,
		Kind:         kind,
		Reason:       reason,
	}
	d.failures = append(d.failures, f)
	log.Printf(
		"[ai-daily-summaries] category failed seller_account_id=%d category_id=%d category_name=%q reason=%s error=%s",
		d.sellerID, cid, name, kind, reason,
	)
}

func (d *categoryRunDiagnostics) recordErr(cid int64, name string, kind categoryFailureKind, err error) {
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	d.record(cid, name, kind, reason)
}

func (d *categoryRunDiagnostics) firstFailure() *categoryFailure {
	if len(d.failures) == 0 {
		return nil
	}
	return &d.failures[0]
}

// aggregatePartialRunError builds a short message stored on ai_daily_summary_runs.error_message.
func (d *categoryRunDiagnostics) aggregatePartialRunError(failed, total int) string {
	if failed <= 0 || total <= 0 {
		return ""
	}
	msg := fmt.Sprintf("category summaries failed: %d/%d", failed, total)
	if f := d.firstFailure(); f != nil {
		msg += fmt.Sprintf(
			"; first_error=category_id=%d category_name=%q reason=%s: %s",
			f.CategoryID,
			f.CategoryName,
			f.Kind,
			truncateDiagText(f.Reason, 240),
		)
	}
	return truncateDiagText(msg, 900)
}

func truncateDiagText(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func formatFirstCategoryErrorFromRun(runMsg string, f *categoryFailure) string {
	if f != nil {
		return fmt.Sprintf(
			"category_id=%d category_name=%q reason=%s: %s",
			f.CategoryID, f.CategoryName, f.Kind, truncateDiagText(f.Reason, 400),
		)
	}
	return strings.TrimSpace(runMsg)
}
