package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
)

// envvar_OTEL_METRICS_EXPORTER is documented at
// https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/#exporter-selection
const envvar_OTEL_METRICS_EXPORTER = "OTEL_METRICS_EXPORTER"

// envvar_OTEL_PROPAGATORS is documented at
// https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/#general-sdk-configuration
const envvar_OTEL_PROPAGATORS = "OTEL_PROPAGATORS"

var meter = otel.Meter("github.com/authgear/authgear-server/pkg/lib/otelauthgear")

func mustInt64Counter(name string, options ...metric.Int64CounterOption) metric.Int64Counter {
	counter, err := meter.Int64Counter(name, options...)
	if err != nil {
		panic(err)
	}
	return counter
}

var Counter = mustInt64Counter(
	"test",
	metric.WithDescription("example counter"),
	metric.WithUnit("{test}"),
)

// SetupOTelSDKGlobally sets up the global propagator and the global meter provider.
// Setting these globally allows us to define metric globally.
// Additionally, it returns a context that MUST BE used as the background context.
// The returned context contains a *sdkresource.Resource.
// The returned context contains a *otelhttp.Labeler.
func SetupOTelSDKGlobally(ctx context.Context) (outCtx context.Context, shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// Ensure shutdown is called when we encounter an error.
	defer func() {
		if err != nil {
			err = errors.Join(err, shutdown(ctx))
		}
	}()

	// Set up resource.
	res, err := newResource(ctx)
	if err != nil {
		return
	}
	outCtx = ctx

	// Set up propagator.
	propagator, err := newPropagator()
	if err != nil {
		return
	}
	// Set the global propagator.
	otel.SetTextMapPropagator(propagator)

	// Set up meter provider.
	meterProvider, err := newMeterProvider(ctx, res)
	if err != nil {
		return
	}
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	// set the global meter provider.
	otel.SetMeterProvider(meterProvider)

	return
}

func newResource(ctx context.Context) (*sdkresource.Resource, error) {
	return sdkresource.New(
		ctx,

		// Information about the otel SDK itself.
		sdkresource.WithTelemetrySDK(),

		// OTEL_RESOURCE_ATTRIBUTES and OTEL_SERVICE_NAME
		sdkresource.WithFromEnv(),

		// The following is WithProcess, except that WithProcessCommandArgs is excluded.
		// The arguments MAY contain sensitive information (such as database password) that
		// we DO NOT want to include.
		sdkresource.WithProcessPID(),
		sdkresource.WithProcessExecutableName(),
		sdkresource.WithProcessExecutablePath(),
		sdkresource.WithProcessOwner(),
		sdkresource.WithProcessRuntimeName(),
		sdkresource.WithProcessRuntimeVersion(),
		sdkresource.WithProcessRuntimeDescription(),

		// Information about the OS.
		sdkresource.WithOS(),

		// Information about container, if it is run as a container.
		sdkresource.WithContainer(),

		// os.Hostname
		sdkresource.WithHost(),

		// /etc/machine-id or /var/lib/dbus/machine-id
		// Since it could fail, we do not include it now.
		// sdkresource.WithHostID(),
	)
}

func newPropagator() (out propagation.TextMapPropagator, err error) {
	// The specification says the default value of OTEL_PROPAGATORS is "tracecontext,baggage"
	// And that is a sane default.

	OTEL_PROPAGATORS := strings.TrimSpace(os.Getenv(envvar_OTEL_PROPAGATORS))

	// Handle default value.
	if OTEL_PROPAGATORS == "" {
		OTEL_PROPAGATORS = "tracecontext,baggage"
	}

	// Handle "none"
	if OTEL_PROPAGATORS == "none" {
		// This is the official way to construct a no-op propagator.
		// See https://github.com/open-telemetry/opentelemetry-go/blob/v1.32.0/internal/global/propagator.go#L29
		out = propagation.NewCompositeTextMapPropagator()
		return
	}

	var propagators []propagation.TextMapPropagator
	parts := strings.Split(OTEL_PROPAGATORS, ",")
	for _, part := range parts {
		switch part {
		case "tracecontext":
			propagators = append(propagators, propagation.TraceContext{})
		case "baggage":
			propagators = append(propagators, propagation.Baggage{})
		default:
			err = fmt.Errorf("unsupported value: %v=%v", envvar_OTEL_PROPAGATORS, OTEL_PROPAGATORS)
			return
		}
	}

	out = propagation.NewCompositeTextMapPropagator(propagators...)
	return
}

func newMeterProvider(ctx context.Context, res *sdkresource.Resource) (*sdkmetric.MeterProvider, error) {
	options := []sdkmetric.Option{
		sdkmetric.WithResource(res),
	}

	exporters, err := newMetricExportersFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	for _, exporter := range exporters {
		// Use PeriodicReader because it supports configuration via environment variables.
		// See https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/#periodic-exporting-metricreader
		reader := sdkmetric.NewPeriodicReader(exporter)
		options = append(options, sdkmetric.WithReader(reader))
	}

	meterProvider := sdkmetric.NewMeterProvider(options...)
	return meterProvider, nil
}

func newMetricExportersFromEnv(ctx context.Context) (exporters []sdkmetric.Exporter, err error) {
	// The specification says the default value of OTEL_METRICS_EXPORTER is "otlp".
	// The documentation of the Go SDK says NewMeterProvider does not have any Reader.
	// Without any Reader, it does nothing.
	// See https://pkg.go.dev/go.opentelemetry.io/otel/sdk/metric#NewMeterProvider
	//
	// I think the behavior of the SDK is a more sane default.
	// The export of metrics should be OPT-IN, rather than OPT-OUT.
	// This makes Authgear backwards-compatible if OTEL_METRICS_EXPORTER is not set.
	OTEL_METRICS_EXPORTER := strings.TrimSpace(os.Getenv(envvar_OTEL_METRICS_EXPORTER))
	if OTEL_METRICS_EXPORTER == "" || OTEL_METRICS_EXPORTER == "none" {
		return nil, nil
	}

	// The spec says the implementation SHOULD support comma-separated list.
	parts := strings.Split(OTEL_METRICS_EXPORTER, ",")
	for _, part := range parts {
		switch part {
		case "otlp":
			exporter, err := otlpmetrichttp.New(ctx)
			if err != nil {
				return nil, err
			}
			exporters = append(exporters, exporter)
		case "console":
			exporter, err := stdoutmetric.New()
			if err != nil {
				return nil, err
			}
			exporters = append(exporters, exporter)
		default:
			err = fmt.Errorf("unsupported value: %v=%v", envvar_OTEL_METRICS_EXPORTER, OTEL_METRICS_EXPORTER)
			return
		}
	}

	return
}

func main() {
	os.Setenv("OTEL_METRICS_EXPORTER", "otlp")
	os.Setenv("OTEL_METRIC_EXPORT_INTERVAL", "5000")
	os.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "http://localhost:4318/v1/metrics")
	os.Setenv("OTEL_SERVICE_NAME", "myservice")

	ctx := context.Background()

	ctx, shutdown, err := SetupOTelSDKGlobally(ctx)
	if err != nil {
		panic(err)
	}
	defer shutdown(ctx)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		Counter.Add(r.Context(), 1)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
<title>otel</title>
</head>
<body>
	<p>
	Visit <a href="http://localhost:9090/graph?g0.expr=test_total" target="_blank">http://localhost:9090</a> and you should should see a test_total metric.
	</p>
	<p>
	FYI: Each request will increment test_total by 1.
	</p>
</body>
</html>
		`))
	})

	http.ListenAndServe(":3000", nil)
}
