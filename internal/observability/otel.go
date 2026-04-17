package observability

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otellog "go.opentelemetry.io/otel/log"
	globallog "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"

	"wlgposter/internal/config"
)

const (
	serviceName         = "wlgposter"
	instrumentationName = "wlgposter/internal/observability"
	exportInterval      = 15 * time.Second
)

func initOTEL(ctx context.Context, cfg *config.Config) (*sdklog.LoggerProvider, *mirrorWriter, *sdkmetric.MeterProvider, error) {
	res := resource.NewWithAttributes(
		"",
		attribute.String("service.name", serviceName),
		attribute.String("deployment.environment.name", cfg.ENV),
	)

	var errs []error

	logProvider, mirror, err := newLogProvider(ctx, res, cfg.OtlpEndpoint)
	if err != nil {
		errs = append(errs, fmt.Errorf("otel logs init: %w", err))
	}

	meterProvider, err := newMeterProvider(ctx, res, cfg.OtlpEndpoint)
	if err != nil {
		errs = append(errs, fmt.Errorf("otel metrics init: %w", err))
	}

	return logProvider, mirror, meterProvider, errors.Join(errs...)
}

func newLogProvider(ctx context.Context, res *resource.Resource, otlpEndpoint string) (*sdklog.LoggerProvider, *mirrorWriter, error) {
	exporter, err := otlploghttp.New(ctx, otlploghttp.WithEndpointURL(fmt.Sprintf("http://%s/v1/logs", otlpEndpoint)))
	if err != nil {
		return nil, nil, err
	}

	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)
	globallog.SetLoggerProvider(provider)

	return provider, newMirrorWriter(globallog.Logger(instrumentationName)), nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource, otlpEndpoint string) (*sdkmetric.MeterProvider, error) {
	exporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(fmt.Sprintf("http://%s/v1/metrics", otlpEndpoint)))
	if err != nil {
		return nil, err
	}

	reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(exportInterval))
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(provider)
	if err := registerRuntimeMetrics(provider.Meter(instrumentationName)); err != nil {
		_ = provider.Shutdown(ctx)
		return nil, err
	}

	return provider, nil
}

func registerRuntimeMetrics(meter metric.Meter) error {
	goroutines, err := meter.Int64ObservableGauge(
		"wlgposter.runtime.goroutines",
		metric.WithDescription("Current number of live goroutines"),
		metric.WithUnit("{goroutine}"),
	)
	if err != nil {
		return err
	}

	heapTotal, err := meter.Int64ObservableGauge(
		"wlgposter.runtime.heap.total",
		metric.WithDescription("Total heap memory obtained from the OS"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return err
	}

	heapUsed, err := meter.Int64ObservableGauge(
		"wlgposter.runtime.heap.used",
		metric.WithDescription("Heap memory currently allocated and in use"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return err
	}

	stackTotal, err := meter.Int64ObservableGauge(
		"wlgposter.runtime.stack.total",
		metric.WithDescription("Total stack memory obtained from the OS"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return err
	}

	stackUsed, err := meter.Int64ObservableGauge(
		"wlgposter.runtime.stack.used",
		metric.WithDescription("Stack memory currently in use by goroutines"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return err
	}

	gcCycles, err := meter.Int64ObservableGauge(
		"wlgposter.runtime.gc.cycles",
		metric.WithDescription("Total completed GC cycles since process start"),
		metric.WithUnit("{cycle}"),
	)
	if err != nil {
		return err
	}

	_, err = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)

		observer.ObserveInt64(goroutines, int64(runtime.NumGoroutine()))
		observer.ObserveInt64(heapTotal, int64(stats.HeapSys))
		observer.ObserveInt64(heapUsed, int64(stats.HeapAlloc))
		observer.ObserveInt64(stackTotal, int64(stats.StackSys))
		observer.ObserveInt64(stackUsed, int64(stats.StackInuse))
		observer.ObserveInt64(gcCycles, int64(stats.NumGC))
		return nil
	}, goroutines, heapTotal, heapUsed, stackTotal, stackUsed, gcCycles)

	return err
}

type logEmitter interface {
	Emit(context.Context, otellog.Record)
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type mirrorWriter struct {
	logger logEmitter

	mu  sync.Mutex
	buf []byte
}

func newMirrorWriter(logger logEmitter) *mirrorWriter {
	return &mirrorWriter{logger: logger}
}

func (w *mirrorWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}

		line := append([]byte(nil), w.buf[:idx]...)
		w.buf = w.buf[idx+1:]
		w.emit(line)
	}

	return len(p), nil
}

func (w *mirrorWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.buf) == 0 {
		return
	}

	line := append([]byte(nil), w.buf...)
	w.buf = w.buf[:0]
	w.emit(line)
}

func (w *mirrorWriter) emit(line []byte) {
	body := strings.TrimRight(string(line), "\r")
	if body == "" {
		return
	}

	now := time.Now()
	record := otellog.Record{}
	record.SetTimestamp(now)
	record.SetObservedTimestamp(now)
	record.SetBody(otellog.StringValue(body))

	if severity, text, ok := parseSeverity(body); ok {
		record.SetSeverity(severity)
		record.SetSeverityText(text)
	}

	w.logger.Emit(context.Background(), record)
}

func parseSeverity(line string) (otellog.Severity, string, bool) {
	clean := ansiRegexp.ReplaceAllString(line, "")

	switch {
	case strings.Contains(clean, " TRC "):
		return otellog.SeverityTrace, "TRC", true
	case strings.Contains(clean, " DBG "):
		return otellog.SeverityDebug, "DBG", true
	case strings.Contains(clean, " INF "):
		return otellog.SeverityInfo, "INF", true
	case strings.Contains(clean, " WRN "):
		return otellog.SeverityWarn, "WRN", true
	case strings.Contains(clean, " ERR "):
		return otellog.SeverityError, "ERR", true
	case strings.Contains(clean, " FTL "):
		return otellog.SeverityFatal, "FTL", true
	case strings.Contains(clean, " PNC "):
		return otellog.SeverityFatal4, "PNC", true
	default:
		return 0, "", false
	}
}
