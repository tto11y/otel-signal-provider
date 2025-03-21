package main

import (
	"context"
	"errors"
	"fmt"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
)

func main() {
	// todo add a switch based on console argument
	fireTraces()
	//fireMetrics()
}

func createTransport() *http.Transport {
	return &http.Transport{
		// DialContext lets you access the connection details
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}

			// Print source IP and port when a connection is established
			localAddr := conn.LocalAddr().(*net.TCPAddr)
			fmt.Printf("Source IP: %s, Source Port: %d\n", localAddr.IP.String(), localAddr.Port)

			return conn, nil
		},
	}
}

func fireMetrics() {
	//ctx := context.Background()

	meterProvider, err := newMeterProvider()
	if err != nil {
		slog.Error("Could not create meter provider:", err)
		return
	}

	otel.SetMeterProvider(meterProvider)
	//meter := meterProvider.Meter("test service")

	if err := runtime.Start(
		runtime.WithMinimumReadMemStatsInterval(10*time.Second),
		runtime.WithMeterProvider(meterProvider),
	); err != nil {
		slog.Error("Could not start:", err)
		return
	}

	//counter, err := meter.Int64Counter("example_counter")
	//if err != nil {
	//	slog.Error("Could not create int counter:", err)
	//	return
	//}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		//for i := 0; i < 1000; i++ {
		//	counter.Add(ctx, 1)
		//	time.Sleep(10 * time.Millisecond)
		//}

		time.Sleep(10 * time.Second)
	}()

	// Wait for a moment to ensure the data is sent
	slog.Info("waiting on goroutine to finish...")
	wg.Wait()
	slog.Info("shutdown...")
}

func newMeterProvider() (*metric.MeterProvider, error) {
	res, err := resource.New(
		context.Background(),
		resource.WithTelemetrySDK(), // Discover and provide information about the OpenTelemetry SDK used.
		resource.WithProcess(),      // Discover and provide process information.
		resource.WithOS(),           // Discover and provide OS information.
		resource.WithContainer(),    // Discover and provide container information.
		resource.WithHost(),         // Discover and provide host information.
		resource.WithAttributes(
			attribute.String("", semconv.SchemaURL),
			semconv.ServiceName("test service jeijj"),
		), // Add custom resource attributes.
		// resource.WithDetectors(thirdparty.Detector{}), // Bring your own external Detector implementation.
	)
	if errors.Is(err, resource.ErrPartialResource) || errors.Is(err, resource.ErrSchemaURLConflict) {
		return nil, fmt.Errorf("sentinel error while creating OpenTelemetry metric resource: %w", err)
	} else if err != nil {
		return nil, fmt.Errorf("error while creating OpenTelemetry metric resource: %w", err)
	}

	httpSecureOption := otlpmetrichttp.WithInsecure()

	// this exporter sends OTLP metrics using protobufs over HTTP
	httpMetricExporter, err := otlpmetrichttp.New(
		context.Background(),
		httpSecureOption,
		// todo is this URL correct?? => what's the correct URL for the OTLP / Prometheus receiver ??
		otlpmetrichttp.WithEndpoint("localhost:4318"), // this is the default URL
	)
	if err != nil {
		return nil, fmt.Errorf("error while creating OpenTelemetry metric exporter: %w", err)
	}

	periodicReader := metric.NewPeriodicReader(httpMetricExporter, metric.WithInterval(100*time.Millisecond))

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(periodicReader),
		metric.WithResource(res),
	)

	return meterProvider, nil
}

func fireTraces() {
	ctx := context.Background()

	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		panic(err)
	}

	// Set up the tracer provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewSchemaless(
			semconv.ServiceNameKey.String("test-service"),
		)),
	)
	defer func() {
		_ = tp.Shutdown(ctx)
	}()

	// Set the global tracer provider
	otel.SetTracerProvider(tp)

	// Create a tracer
	tracer := otel.Tracer("test-tracer")

	// Create and export a test trace
	ctx, span := tracer.Start(ctx, "test-span")
	time.Sleep(100 * time.Millisecond) // Simulate work
	span.End()

	// todo test apply this in the CIM-API for error cases
	//span.RecordError(ctx, errors.New("span error"), trace.WithErrorStatus(codes.Error))

	ctx, span = tracer.Start(ctx, "test-span")
	time.Sleep(100 * time.Millisecond) // Simulate work
	span.End()

	// Wait for a moment to ensure the data is sent
	time.Sleep(5 * time.Second)
}
