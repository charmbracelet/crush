// Package telemetry provides OpenTelemetry tracing support for Crush.
//
// Tracing is enabled by setting OTEL_EXPORTER_OTLP_ENDPOINT or configuring
// telemetry in crush.json. When enabled, spans are exported to an
// OTLP-compatible collector (Jaeger, Tempo, Honeycomb, etc.).
package telemetry

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const (
	// DefaultServiceName is the default service name for traces.
	DefaultServiceName = "crush"

	// DefaultMaxContentLength is the default max length for captured content.
	DefaultMaxContentLength = 4096
)

// Span names for different operations.
const (
	SpanSession         = "crush.session"
	SpanAgentRun        = "crush.agent.run"
	SpanLLMRequest      = "crush.llm.request"
	SpanLLMStreamChunk  = "crush.llm.stream_chunk"
	SpanLLMResponse     = "crush.llm.response"
	SpanToolExecute     = "crush.tool.execute"
	SpanSummarize       = "crush.summarize"
	SpanTitleGeneration = "crush.title_generation"
)

// Config holds telemetry configuration.
type Config struct {
	Enabled          bool              `json:"enabled,omitempty"`
	Endpoint         string            `json:"endpoint,omitempty"`
	Protocol         string            `json:"protocol,omitempty"` // "grpc" or "http/protobuf"
	ServiceName      string            `json:"service_name,omitempty"`
	CaptureContent   bool              `json:"capture_content,omitempty"`
	MaxContentLength int               `json:"max_content_length,omitempty"`
	SampleRate       float64           `json:"sample_rate,omitempty"`
	Headers          map[string]string `json:"headers,omitempty"`
}

var (
	globalTracer     trace.Tracer
	globalConfig     Config
	globalShutdown   func(context.Context) error
	initOnce         sync.Once
	initialized      bool
	tracerMu         sync.RWMutex
	noopTracerSingle = noop.NewTracerProvider().Tracer("")
)

// Init initializes the telemetry system with the given configuration.
// If endpoint is not set (via config or env), telemetry is disabled.
func Init(ctx context.Context, cfg Config, version string) error {
	var initErr error
	initOnce.Do(func() {
		initErr = doInit(ctx, cfg, version)
	})
	return initErr
}

func doInit(ctx context.Context, cfg Config, version string) error {
	// Merge environment variables with config (env takes precedence).
	mergeEnvConfig(&cfg)

	// If no endpoint is configured, telemetry is disabled.
	if cfg.Endpoint == "" {
		slog.Debug("telemetry disabled: no endpoint configured")
		globalTracer = noopTracerSingle
		return nil
	}

	// Set defaults.
	if cfg.ServiceName == "" {
		cfg.ServiceName = DefaultServiceName
	}
	if cfg.Protocol == "" {
		cfg.Protocol = "grpc"
	}
	if cfg.MaxContentLength == 0 {
		cfg.MaxContentLength = DefaultMaxContentLength
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 1.0
	}

	globalConfig = cfg

	// Create exporter based on protocol.
	var exporter sdktrace.SpanExporter
	var err error

	switch strings.ToLower(cfg.Protocol) {
	case "http", "http/protobuf":
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(cfg.Endpoint),
		}
		if !strings.HasPrefix(cfg.Endpoint, "https://") {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		for k, v := range cfg.Headers {
			v = expandEnvValue(v)
			opts = append(opts, otlptracehttp.WithHeaders(map[string]string{k: v}))
		}
		exporter, err = otlptracehttp.New(ctx, opts...)
	default: // grpc
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
		}
		if !strings.HasPrefix(cfg.Endpoint, "https://") {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		for k, v := range cfg.Headers {
			v = expandEnvValue(v)
			opts = append(opts, otlptracegrpc.WithHeaders(map[string]string{k: v}))
		}
		exporter, err = otlptracegrpc.New(ctx, opts...)
	}

	if err != nil {
		slog.Error("failed to create OTLP exporter", "error", err)
		globalTracer = noopTracerSingle
		return err
	}

	// Create resource with service info.
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		slog.Error("failed to create resource", "error", err)
		globalTracer = noopTracerSingle
		return err
	}

	// Create sampler.
	var sampler sdktrace.Sampler
	if cfg.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.SampleRate <= 0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Create tracer provider.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global tracer provider.
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracerMu.Lock()
	globalTracer = tp.Tracer(cfg.ServiceName)
	globalShutdown = tp.Shutdown
	initialized = true
	tracerMu.Unlock()

	slog.Info("telemetry initialized",
		"endpoint", cfg.Endpoint,
		"protocol", cfg.Protocol,
		"service", cfg.ServiceName,
		"sample_rate", cfg.SampleRate,
	)

	return nil
}

// Shutdown gracefully shuts down the telemetry system.
func Shutdown(ctx context.Context) error {
	tracerMu.RLock()
	shutdown := globalShutdown
	tracerMu.RUnlock()

	if shutdown != nil {
		return shutdown(ctx)
	}
	return nil
}

// Tracer returns the global tracer. If telemetry is not initialized,
// returns a no-op tracer.
func Tracer() trace.Tracer {
	tracerMu.RLock()
	defer tracerMu.RUnlock()

	if globalTracer == nil {
		return noopTracerSingle
	}
	return globalTracer
}

// IsEnabled returns true if telemetry is enabled and initialized.
func IsEnabled() bool {
	tracerMu.RLock()
	defer tracerMu.RUnlock()
	return initialized
}

// CaptureContent returns true if content capture is enabled.
func CaptureContent() bool {
	tracerMu.RLock()
	defer tracerMu.RUnlock()
	return globalConfig.CaptureContent
}

// MaxContentLength returns the maximum content length to capture.
func MaxContentLength() int {
	tracerMu.RLock()
	defer tracerMu.RUnlock()
	if globalConfig.MaxContentLength == 0 {
		return DefaultMaxContentLength
	}
	return globalConfig.MaxContentLength
}

// StartSpan starts a new span with the given name and options.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, opts...)
}

// mergeEnvConfig merges environment variables into the config.
func mergeEnvConfig(cfg *Config) {
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		cfg.Endpoint = endpoint
		cfg.Enabled = true
	}
	if protocol := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"); protocol != "" {
		cfg.Protocol = protocol
	}
	if serviceName := os.Getenv("OTEL_SERVICE_NAME"); serviceName != "" {
		cfg.ServiceName = serviceName
	}
	if capture := os.Getenv("CRUSH_OTEL_CAPTURE_CONTENT"); capture != "" {
		cfg.CaptureContent, _ = strconv.ParseBool(capture)
	}
	if maxLen := os.Getenv("CRUSH_OTEL_MAX_CONTENT_LENGTH"); maxLen != "" {
		if n, err := strconv.Atoi(maxLen); err == nil {
			cfg.MaxContentLength = n
		}
	}
	if sampleRate := os.Getenv("OTEL_TRACES_SAMPLER_ARG"); sampleRate != "" {
		if rate, err := strconv.ParseFloat(sampleRate, 64); err == nil {
			cfg.SampleRate = rate
		}
	}
}

// expandEnvValue expands environment variable references in a string.
// Supports $VAR and ${VAR} syntax.
func expandEnvValue(s string) string {
	if varName, ok := strings.CutPrefix(s, "$"); ok {
		varName = strings.Trim(varName, "{}")
		return os.Getenv(varName)
	}
	return s
}

// TruncateString truncates a string to the max content length.
func TruncateString(s string) string {
	maxLen := MaxContentLength()
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// WrapToolExecution wraps a tool execution function with telemetry tracing.
func WrapToolExecution(ctx context.Context, toolName string, input string) (context.Context, func(outputLen int, err error)) {
	ctx, span := StartSpan(ctx, SpanToolExecute,
		trace.WithAttributes(
			AttrToolName.String(toolName),
		),
	)

	// Optionally capture input if content capture is enabled.
	if CaptureContent() {
		span.SetAttributes(AttrToolInput.String(TruncateString(input)))
	}

	return ctx, func(outputLen int, err error) {
		span.SetAttributes(
			AttrToolOutputLen.Int(outputLen),
			AttrToolSuccess.Bool(err == nil),
		)
		if err != nil {
			span.SetAttributes(AttrToolError.String(err.Error()))
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}

// Common attribute keys.
var (
	AttrSessionID       = attribute.Key("session.id")
	AttrSessionName     = attribute.Key("session.name")
	AttrCrushVersion    = attribute.Key("crush.version")
	AttrModelProvider   = attribute.Key("crush.model.provider")
	AttrModelID         = attribute.Key("crush.model.id")
	AttrLLMProvider     = attribute.Key("llm.provider")
	AttrLLMModel        = attribute.Key("llm.model")
	AttrLLMMaxTokens    = attribute.Key("llm.request.max_tokens")
	AttrLLMTemperature  = attribute.Key("llm.request.temperature")
	AttrLLMMsgCount     = attribute.Key("llm.request.message_count")
	AttrLLMInputTokens  = attribute.Key("llm.response.tokens.input")
	AttrLLMOutputTokens = attribute.Key("llm.response.tokens.output")
	AttrLLMStopReason   = attribute.Key("llm.response.stop_reason")
	AttrLLMReqContent   = attribute.Key("llm.request.content")
	AttrLLMRespContent  = attribute.Key("llm.response.content")
	AttrToolName        = attribute.Key("tool.name")
	AttrToolInput       = attribute.Key("tool.input")
	AttrToolOutputLen   = attribute.Key("tool.output.length")
	AttrToolSuccess     = attribute.Key("tool.success")
	AttrToolError       = attribute.Key("tool.error")
)
