package observability

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/jiangchengyu998/demo-go/internal/config"
)

var (
	requestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_server_requests",
		Help: "Total HTTP requests.",
	}, []string{"method", "path", "status"})
	requestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_request_duration_seconds",
		Help:    "HTTP request latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)

func ConfigureTracing(ctx context.Context, settings config.Settings) (func(context.Context) error, error) {
	if settings.OTELSDKDisabled {
		slog.Info("OTEL SDK disabled")
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(settings.OTELTracesEndpoint))
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", settings.AppName),
			attribute.String("deployment.environment", settings.DeploymentEnvironment),
		),
	)
	if err != nil {
		return nil, err
	}

	provider := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exporter),
		tracesdk.WithSampler(tracesdk.TraceIDRatioBased(settings.TracingSamplingProbability)),
		tracesdk.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	if settings.OTELDebugLoggingEnabled {
		slog.Info("OTEL configured",
			"serviceName", settings.AppName,
			"environment", settings.DeploymentEnvironment,
			"tracesEndpoint", settings.OTELTracesEndpoint,
			"samplingProbability", settings.TracingSamplingProbability,
		)
	}
	return provider.Shutdown, nil
}

func HTTPMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	tracer := otel.Tracer("cloud-deploy-demo-go/http")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := routePath(r)
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := tracer.Start(ctx, r.Method+" "+path, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r.WithContext(ctx))

		duration := time.Since(startedAt)
		status := strconv.Itoa(recorder.status)
		requestCount.WithLabelValues(r.Method, path, status).Inc()
		requestLatency.WithLabelValues(r.Method, path).Observe(duration.Seconds())
		span.SetAttributes(
			attribute.String("http.request.method", r.Method),
			attribute.String("url.path", r.URL.Path),
			attribute.Int("http.response.status_code", recorder.status),
		)

		traceID, spanID := traceFields(span.SpanContext())
		logger.Info("HTTP trace",
			"method", r.Method,
			"uri", r.URL.Path,
			"status", recorder.status,
			"durationMs", duration.Milliseconds(),
			"traceId", traceID,
			"spanId", spanID,
		)
	})
}

func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func routePath(r *http.Request) string {
	if r.URL.Path == "/" ||
		r.URL.Path == "/api/items" ||
		r.URL.Path == "/actuator/health" ||
		r.URL.Path == "/actuator/prometheus" ||
		r.URL.Path == "/swagger-ui.html" ||
		r.URL.Path == "/v3/api-docs" {
		return r.URL.Path
	}
	if hasItemIDPath(r.URL.Path) {
		return "/api/items/{id}"
	}
	return r.URL.Path
}

func hasItemIDPath(path string) bool {
	const prefix = "/api/items/"
	return len(path) > len(prefix) && path[:len(prefix)] == prefix
}

func traceFields(spanContext trace.SpanContext) (string, string) {
	if !spanContext.IsValid() {
		return "none", "none"
	}
	return spanContext.TraceID().String(), spanContext.SpanID().String()
}
