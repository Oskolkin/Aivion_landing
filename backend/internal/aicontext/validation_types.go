package aicontext

// ValidationStatus values for global context validation.
const (
	ValidationStatusValid             = "valid"
	ValidationStatusValidWithWarnings = "valid_with_warnings"
	ValidationStatusRepairable        = "repairable"
	ValidationStatusInvalid           = "invalid"
)

// IssueSeverity values for validation issues.
const (
	IssueSeverityFatal      = "fatal"
	IssueSeverityRepairable = "repairable"
	IssueSeverityWarning    = "warning"
)
