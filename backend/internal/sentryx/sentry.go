package sentryx

import (
	"time"

	"github.com/getsentry/sentry-go"
)

type Config struct {
	DSN         string
	Environment string
	Release     string
}

func Init(cfg Config) error {
	if cfg.DSN == "" {
		return nil
	}

	return sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.DSN,
		Environment:      cfg.Environment,
		Release:          cfg.Release,
		AttachStacktrace: true,
	})
}

func Flush() {
	sentry.Flush(2 * time.Second)
}
