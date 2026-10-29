package buildinfo

import (
	"runtime"
	"time"
)

var (
	BuildVersion = "dev"
	BuildCommit  = "unknown"
	BuildDate    = "unknown"
	startedAtUTC = time.Now().UTC()
)

type Build struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	BuiltAt string `json:"built_at"`
}

type Runtime struct {
	GoVersion   string `json:"go_version"`
	Environment string `json:"environment"`
	ListenAddr  string `json:"listen_addr,omitempty"`
}

type Info struct {
	Service       string  `json:"service"`
	Status        string  `json:"status"`
	StartedAt     string  `json:"started_at"`
	UptimeSeconds int64   `json:"uptime_seconds"`
	Build         Build   `json:"build"`
	Runtime       Runtime `json:"runtime"`
}

type Provider struct {
	service     string
	environment string
	listenAddr  string
}

func NewProvider(service, environment, listenAddr string) *Provider {
	return &Provider{
		service:     service,
		environment: environment,
		listenAddr:  listenAddr,
	}
}

func (p *Provider) Info() Info {
	now := time.Now().UTC()
	uptime := int64(now.Sub(startedAtUTC).Seconds())
	if uptime < 0 {
		uptime = 0
	}
	return Info{
		Service:       p.service,
		Status:        "ok",
		StartedAt:     startedAtUTC.Format(time.RFC3339),
		UptimeSeconds: uptime,
		Build: Build{
			Version: BuildVersion,
			Commit:  BuildCommit,
			BuiltAt: BuildDate,
		},
		Runtime: Runtime{
			GoVersion:   runtime.Version(),
			Environment: p.environment,
			ListenAddr:  p.listenAddr,
		},
	}
}

func StartedAt() time.Time {
	return startedAtUTC
}
