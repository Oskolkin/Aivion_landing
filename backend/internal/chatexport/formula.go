package chatexport

// escapeFormulaCell prevents spreadsheet applications from interpreting
// user-provided text as a formula when an export is opened.
func escapeFormulaCell(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

func escapeFormulaRow(values []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = escapeFormulaCell(value)
	}
	return out
}
