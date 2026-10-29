package pricingconstraints

import "strings"

type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Add(problem string) {
	if strings.TrimSpace(problem) == "" {
		return
	}
	e.Problems = append(e.Problems, problem)
}

func (e *ValidationError) Empty() bool {
	return len(e.Problems) == 0
}

func (e *ValidationError) Error() string {
	if len(e.Problems) == 0 {
		return "validation failed"
	}
	return "validation failed: " + strings.Join(e.Problems, "; ")
}
