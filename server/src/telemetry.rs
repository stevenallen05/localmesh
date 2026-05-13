use anyhow::Result;
use opentelemetry::global;
use opentelemetry_otlp::WithExportConfig;
use opentelemetry_sdk::{
    metrics::SdkMeterProvider, trace::SdkTracerProvider, Resource,
};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt, EnvFilter};

/// RAII guard — drop on process exit flushes both providers.
pub struct TelemetryGuard {
    tracer_provider: SdkTracerProvider,
    meter_provider: SdkMeterProvider,
}

impl Drop for TelemetryGuard {
    fn drop(&mut self) {
        let _ = self.tracer_provider.shutdown();
        let _ = self.meter_provider.shutdown();
    }
}

pub fn init() -> Result<TelemetryGuard> {
    let endpoint = std::env::var("OTEL_EXPORTER_OTLP_ENDPOINT")
        .unwrap_or_else(|_| "http://otel-collector:4317".to_string());
    let service_name =
        std::env::var("OTEL_SERVICE_NAME").unwrap_or_else(|_| "server".to_string());

    let resource = Resource::builder().with_service_name(service_name).build();

    // Tracer provider — OTLP gRPC.
    let span_exporter = opentelemetry_otlp::SpanExporter::builder()
        .with_tonic()
        .with_endpoint(&endpoint)
        .build()?;
    let tracer_provider = SdkTracerProvider::builder()
        .with_batch_exporter(span_exporter)
        .with_resource(resource.clone())
        .build();
    global::set_tracer_provider(tracer_provider.clone());
    let tracer = opentelemetry::trace::TracerProvider::tracer(&tracer_provider, "server");

    // Meter provider — OTLP gRPC, cumulative (default) for VM compat.
    let metric_exporter = opentelemetry_otlp::MetricExporter::builder()
        .with_tonic()
        .with_endpoint(&endpoint)
        .build()?;
    let meter_provider = SdkMeterProvider::builder()
        .with_periodic_exporter(metric_exporter)
        .with_resource(resource)
        .build();
    global::set_meter_provider(meter_provider.clone());

    // tracing-subscriber: fmt + OTel bridge.
    tracing_subscriber::registry()
        .with(EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info")))
        .with(tracing_subscriber::fmt::layer())
        .with(tracing_opentelemetry::layer().with_tracer(tracer))
        .init();

    Ok(TelemetryGuard {
        tracer_provider,
        meter_provider,
    })
}
