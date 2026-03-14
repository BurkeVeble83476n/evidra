package telemetry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"

	"samebits.com/evidra/internal/config"
)

type otlpHTTPTransport struct {
	provider *sdkmetric.MeterProvider
	reader   *sdkmetric.ManualReader
	exporter sdkmetric.Exporter
	meter    otelmetric.Meter

	mu     sync.Mutex
	gauges map[string]otelmetric.Float64Gauge
	dirty  bool
	closed bool
}

// NewOTLPHTTP creates a transport that pushes metrics via OTLP/HTTP protobuf.
func NewOTLPHTTP(cfg config.MetricsConfig) (Transport, error) {
	endpoint := strings.TrimSpace(cfg.OTLPEndpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("otlp_http endpoint is required")
	}
	if cfg.Timeout <= 0 {
		return nil, fmt.Errorf("otlp_http timeout must be > 0")
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpointURL(endpoint),
		otlpmetrichttp.WithTimeout(cfg.Timeout),
	)
	if err != nil {
		return nil, fmt.Errorf("create otlp exporter: %w", err)
	}
	reader := sdkmetric.NewManualReader(
		sdkmetric.WithTemporalitySelector(exporter.Temporality),
		sdkmetric.WithAggregationSelector(exporter.Aggregation),
	)

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", "evidra-cli"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)

	meter := provider.Meter("samebits.com/evidra")

	return &otlpHTTPTransport{
		provider: provider,
		reader:   reader,
		exporter: exporter,
		meter:    meter,
		gauges:   make(map[string]otelmetric.Float64Gauge),
	}, nil
}

func (t *otlpHTTPTransport) Emit(ctx context.Context, metric OperationMetric) error {
	metric.Labels = BoundedLabels(metric.Labels)
	if strings.TrimSpace(metric.Name) == "" {
		metric.Name = "evidra.operation.metric"
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return fmt.Errorf("metrics transport is closed")
	}

	gauge, err := t.getOrCreateGaugeLocked(metric.Name)
	if err != nil {
		return fmt.Errorf("create gauge %q: %w", metric.Name, err)
	}

	attrs := otelmetric.WithAttributes(
		attribute.String("tool", metric.Labels.Tool),
		attribute.String("environment", metric.Labels.Environment),
		attribute.String("result_class", metric.Labels.ResultClass),
		attribute.String("signal_name", metric.Labels.SignalName),
		attribute.String("score_band", metric.Labels.ScoreBand),
		attribute.String("assessment_mode", metric.Labels.AssessmentMode),
	)

	gauge.Record(ctx, metric.Value, attrs)
	t.dirty = true
	return nil
}

func (t *otlpHTTPTransport) getOrCreateGaugeLocked(name string) (otelmetric.Float64Gauge, error) {
	if g, ok := t.gauges[name]; ok {
		return g, nil
	}

	g, err := t.meter.Float64Gauge(name)
	if err != nil {
		return nil, err
	}
	t.gauges[name] = g
	return g, nil
}

func (t *otlpHTTPTransport) Flush(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return fmt.Errorf("metrics transport is closed")
	}
	return t.flushLocked(ctx)
}

func (t *otlpHTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var errs []error
	if err := t.flushLocked(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := t.provider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("shutdown meter provider: %w", err))
	}
	if err := t.exporter.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("shutdown otlp exporter: %w", err))
	}
	t.closed = true
	return errors.Join(errs...)
}

func (t *otlpHTTPTransport) flushLocked(ctx context.Context) error {
	if !t.dirty {
		return nil
	}

	var rm metricdata.ResourceMetrics
	if err := t.reader.Collect(ctx, &rm); err != nil {
		return fmt.Errorf("collect metrics: %w", err)
	}
	if len(rm.ScopeMetrics) == 0 {
		t.dirty = false
		return nil
	}
	if err := t.exporter.Export(ctx, &rm); err != nil {
		return fmt.Errorf("export metrics: %w", err)
	}
	if err := t.exporter.ForceFlush(ctx); err != nil {
		return fmt.Errorf("flush exporter: %w", err)
	}
	t.dirty = false
	return nil
}
