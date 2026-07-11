package observability

import (
	"context"
	"testing"

	"github.com/jiangchengyu998/demo-go/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestConfigureTracingExtractsB3Context(t *testing.T) {
	settings := config.Settings{
		AppName:                    "test",
		DeploymentEnvironment:      "test",
		OTELSDKDisabled:            false,
		OTELExporterDisabled:       true,
		TracingSamplingProbability: 1,
	}
	shutdown, err := ConfigureTracing(context.Background(), settings)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
	})

	carrier := propagation.MapCarrier{
		"x-b3-traceid": "3f56333eae972a2f6ea7eac4ff4aa8d8",
		"x-b3-spanid":  "6ea7eac4ff4aa8d8",
		"x-b3-sampled": "1",
	}
	ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
	spanContext := trace.SpanContextFromContext(ctx)

	if got := spanContext.TraceID().String(); got != "3f56333eae972a2f6ea7eac4ff4aa8d8" {
		t.Fatalf("trace id = %s", got)
	}
	if got := spanContext.SpanID().String(); got != "6ea7eac4ff4aa8d8" {
		t.Fatalf("span id = %s", got)
	}
}
