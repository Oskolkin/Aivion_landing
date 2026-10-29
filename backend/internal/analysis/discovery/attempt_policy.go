package discovery

type AIAttemptPolicy struct {
	Version          string `json:"version"`
	MaxModelAttempts int    `json:"max_model_attempts"`
}

func CurrentAttemptPolicy() AIAttemptPolicy {
	return AIAttemptPolicy{Version: AIAttemptPolicyV1, MaxModelAttempts: 2}
}

type AttemptClass string

const (
	AttemptValid      AttemptClass = "VALID"
	AttemptRefused    AttemptClass = "REFUSED"
	AttemptIncomplete AttemptClass = "INCOMPLETE"
	AttemptInvalid    AttemptClass = "INVALID"
	AttemptProvider   AttemptClass = "PROVIDER_ERROR"
)

func ShouldRerun(class AttemptClass, attemptNo, max int) bool {
	if attemptNo >= max {
		return false
	}
	switch class {
	case AttemptRefused, AttemptIncomplete, AttemptInvalid:
		return true
	default:
		return false
	}
}
