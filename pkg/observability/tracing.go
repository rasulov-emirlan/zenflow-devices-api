// Package observability wires the OpenTelemetry tracer provider.
//
// InitTracer builds a TracerProvider exporting OTLP/gRPC and installs it as
// the global provider. It returns a shutdown function the caller must run on
// process exit to flush pending spans. Propagators are W3C TraceContext +
// Baggage so HTTP clients/servers behave predictably across hops.
package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Version is the service version reported to OTLP. Wired from build flags if
// desired; kept as a var so cmd/* can override via -ldflags.
var Version = "dev"

// InitTracer configures the global OTEL TracerProvider. The returned shutdown
// function is safe to call even if initialization partially failed.
//
// endpoint is expected as "host:port" (no scheme) per the OTLP gRPC exporter
// convention; insecure transport is used for in-cluster collector traffic.
func InitTracer(ctx context.Context, serviceName, endpoint, env string) (func(context.Context) error, error) {
	// Resource attributes identify this process in the trace backend.
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(Version),
			semconv.DeploymentEnvironment(env),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	exp, err := otlptracegrpc.New(dialCtx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		return nil, fmt.Errorf("otlp exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp,
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
		// Default sampler is AlwaysSample in dev-scale; switch to parent-based
		// ratio sampling once volume warrants it via env.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdown := func(ctx context.Context) error {
		// Best-effort flush, then hard shutdown so we don't block the process.
		_ = tp.ForceFlush(ctx)
		return tp.Shutdown(ctx)
	}
	return shutdown, nil
}
