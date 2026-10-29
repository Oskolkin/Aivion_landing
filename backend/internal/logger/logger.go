package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(env, service string) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()

	cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	cfg.Encoding = "json"
	cfg.OutputPaths = []string{"stderr"}
	cfg.ErrorOutputPaths = []string{"stderr"}

	if env == "local" {
		cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}

	base, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	return base.With(
		zap.String("service", service),
		zap.String("env", env),
	), nil
}
