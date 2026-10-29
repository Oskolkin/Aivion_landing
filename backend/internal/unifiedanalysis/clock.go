package unifiedanalysis

import "time"

// Clock abstracts time for tests.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now().UTC()
}

// RealClock is the production clock implementation.
var RealClock Clock = realClock{}
