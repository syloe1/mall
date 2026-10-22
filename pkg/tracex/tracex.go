package tracex

import (
	"context"
	"mall/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func Init(ctx context.Context, cfg config.TraceConfig) (func(context.Context) error, error) {
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "mall-api"
	}
	if cfg.JaegerEndpoint == "" {
		cfg.JaegerEndpoint = "http://127.0.0.1:14268/api/traces"
	}

	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.JaegerEndpoint)))
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx, resource.WithAttributes(attribute.String("service.name", cfg.ServiceName)))
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return provider.Shutdown, nil
}
