package policy

import (
	"math/rand"
	"time"
)

const VersionV1 = "analysis_orchestration_policy_v1"

// FailureClass is the closed Prompt №8 attempt failure taxonomy.
type FailureClass string

const (
	TransientDependency FailureClass = "TRANSIENT_DEPENDENCY"
	Timeout             FailureClass = "TIMEOUT"
	LeaseExpired        FailureClass = "LEASE_EXPIRED"
	WorkerLost          FailureClass = "WORKER_LOST"
	ContractViolation   FailureClass = "CONTRACT_VIOLATION"
	CorruptedInput      FailureClass = "CORRUPTED_INPUT"
	InternalBug         FailureClass = "INTERNAL_BUG"
	Cancelled           FailureClass = "CANCELLED"
)

type Retry struct {
	Retryable      bool
	MaxAttempts    int
	MaxElapsed     time.Duration
	AttemptTimeout time.Duration
	StageTimeout   time.Duration
	Backoff        time.Duration
	MaxBackoff     time.Duration
	Jitter         time.Duration
}

type Clock interface{ Now() time.Time }
type Random interface{ Int63n(int64) int64 }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

type Policy struct {
	Version       string
	LeaseDuration time.Duration
	Retry         map[FailureClass]Retry
	Timeouts      map[string]time.Duration
	Clock         Clock
	Random        Random
}

func V1(clock Clock, random Random) Policy {
	if clock == nil {
		clock = realClock{}
	}
	if random == nil {
		random = rand.New(rand.NewSource(1))
	}
	return Policy{
		Version:       VersionV1,
		LeaseDuration: 2 * time.Minute,
		Clock:         clock,
		Random:        random,
		Retry: map[FailureClass]Retry{
			TransientDependency: {true, 5, 15 * time.Minute, 90 * time.Second, 15 * time.Minute, time.Second, time.Minute, 500 * time.Millisecond},
			Timeout:             {true, 3, 10 * time.Minute, 2 * time.Minute, 10 * time.Minute, 2 * time.Second, time.Minute, 500 * time.Millisecond},
			LeaseExpired:        {false, 1, 0, 0, 0, 0, 0, 0},
			WorkerLost:          {false, 1, 0, 0, 0, 0, 0, 0},
			ContractViolation:   {false, 1, 0, 90 * time.Second, 90 * time.Second, 0, 0, 0},
			CorruptedInput:      {false, 1, 0, 90 * time.Second, 90 * time.Second, 0, 0, 0},
			InternalBug:         {false, 1, 0, 90 * time.Second, 90 * time.Second, 0, 0, 0},
			Cancelled:           {false, 1, 0, 0, 0, 0, 0, 0},
		},
	}
}

func (p Policy) TimeoutFor(class string) time.Duration {
	if p.Timeouts != nil {
		if d := p.Timeouts[class]; d > 0 {
			return d
		}
	}
	r := p.Retry[TransientDependency]
	timeout := r.AttemptTimeout
	if timeout <= 0 {
		timeout = r.StageTimeout
	}
	if timeout <= 0 {
		timeout = p.LeaseDuration
	}
	return timeout
}

func (p Policy) CanRetry(class FailureClass, attempts int, firstAttempt time.Time) bool {
	r, ok := p.Retry[class]
	if !ok || !r.Retryable || attempts >= r.MaxAttempts {
		return false
	}
	if r.MaxElapsed > 0 && p.Clock.Now().Sub(firstAttempt) >= r.MaxElapsed {
		return false
	}
	return true
}

func (p Policy) Delay(class FailureClass, attempts int) time.Duration {
	r := p.Retry[class]
	d := r.Backoff
	for i := 1; i < attempts && d < r.MaxBackoff; i++ {
		d *= 2
	}
	if r.MaxBackoff > 0 && d > r.MaxBackoff {
		d = r.MaxBackoff
	}
	if r.Jitter > 0 {
		d += time.Duration(p.Random.Int63n(int64(r.Jitter) + 1))
	}
	return d
}

const VersionV2 = "analysis_orchestration_policy_v2"

func V2(clock Clock, random Random) Policy {
	p := V1(clock, random)
	p.Version = VersionV2
	return p
}

const VersionV3 = "analysis_orchestration_policy_v3"

func V3(clock Clock, random Random) Policy {
	p := V2(clock, random)
	p.Version = VersionV3
	p.Timeouts = map[string]time.Duration{
		"stage_default": p.Retry[TransientDependency].AttemptTimeout,
		"ai_default":    120 * time.Second,
		"async_default": 15 * time.Minute,
	}
	return p
}

const VersionV4 = "analysis_orchestration_policy_v4"

func V4(clock Clock, random Random) Policy {
	p := V3(clock, random)
	p.Version = VersionV4
	return p
}

const VersionV5 = "analysis_orchestration_policy_v5"

func V5(clock Clock, random Random) Policy {
	p := V4(clock, random)
	p.Version = VersionV5
	return p
}

const VersionV6 = "analysis_orchestration_policy_v6"

func V6(clock Clock, random Random) Policy {
	p := V5(clock, random)
	p.Version = VersionV6
	if p.Timeouts == nil {
		p.Timeouts = map[string]time.Duration{}
	}
	p.Timeouts["ai_deep"] = 180 * time.Second
	return p
}

const VersionV7 = "analysis_orchestration_policy_v7"

func V7(clock Clock, random Random) Policy {
	p := V6(clock, random)
	p.Version = VersionV7
	return p
}

func Current(clock Clock, random Random) Policy {
	return V7(clock, random)
}
