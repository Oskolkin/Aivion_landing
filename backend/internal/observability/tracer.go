package observability

import (
	"context"
	"os"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const (
	TracerAnalysis = "aivion.analysis"
	TracerChat     = "aivion.chat"
	TracerEval     = "aivion.eval"
	TracerGenAI    = "aivion.gen_ai"
)

var (
	mu       sync.Mutex
	provider *sdktrace.TracerProvider
)

type TracerConfig struct {
	ServiceName string
	Environment string
	Endpoint    string
}

// InitTracer installs an OTLP HTTP tracer when OTEL_EXPORTER_OTLP_ENDPOINT is set.
// Otherwise a process-safe no-op tracer is used and the API/worker keep running.
func InitTracer(ctx context.Context, cfg TracerConfig) (func(context.Context) error, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "aivion"
	}
	if endpoint == "" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(context.Context) error { return nil }, nil
	}
	exp, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(normalizeOTLPEndpoint(endpoint)))
	if err != nil {
		return nil, err
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(nz(cfg.Environment, os.Getenv("APP_ENV"))),
		),
	)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	mu.Lock()
	provider = tp
	mu.Unlock()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp.Shutdown, nil
}

func Shutdown(ctx context.Context) {
	mu.Lock()
	tp := provider
	mu.Unlock()
	if tp != nil {
		_ = tp.Shutdown(ctx)
	}
}

func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func normalizeOTLPEndpoint(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	if strings.Contains(s, "://") {
		return s
	}
	return "http://" + s
}

func nz(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
