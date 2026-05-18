use std::collections::HashMap;

use opentelemetry::global;
use opentelemetry::propagation::Extractor;
use opentelemetry::trace::TracerProvider as _;
use opentelemetry::KeyValue;
use opentelemetry_otlp::{MetricExporter, SpanExporter, WithExportConfig};
use opentelemetry_sdk::metrics::{PeriodicReader, SdkMeterProvider};
use opentelemetry_sdk::propagation::TraceContextPropagator;
use opentelemetry_sdk::trace::SdkTracerProvider;
// In opentelemetry_sdk 0.31 this is a back-compat alias for SdkTracer
// (`pub use tracer::SdkTracer as Tracer;` in the SDK's mod.rs). The alias
// is kept specifically so tracing-opentelemetry builds. If cargo check
// stops resolving this on a future SDK bump, switch to `SdkTracer`.
use opentelemetry_sdk::trace::Tracer;
use opentelemetry_sdk::Resource;
use tonic::metadata::{KeyRef, MetadataMap as TonicMetadataMap};
use uuid::Uuid;

pub type BoxError = Box<dyn std::error::Error + Send + Sync>;

/// Parse the OTel-standard `OTEL_RESOURCE_ATTRIBUTES` env-var format:
/// a comma-separated list of `key=value` pairs. Whitespace around keys
/// and values is trimmed; entries without `=` are dropped.
pub(crate) fn parse_otel_resource_attrs(raw: &str) -> HashMap<String, String> {
    raw.split(',')
        .filter_map(|pair| {
            let (k, v) = pair.split_once('=')?;
            let k = k.trim();
            let v = v.trim();
            if k.is_empty() {
                None
            } else {
                Some((k.to_string(), v.to_string()))
            }
        })
        .collect()
}

/// Build the shared OTel `Resource` from `OTEL_SERVICE_NAME` +
/// `OTEL_RESOURCE_ATTRIBUTES`, attaching a per-process `service.instance.id`.
/// Used by both the tracer and meter providers so traces and metrics agree
/// on identity.
pub(crate) fn build_resource() -> Resource {
    let service_name = std::env::var("OTEL_SERVICE_NAME").unwrap_or_else(|_| "server".to_string());
    let attrs = parse_otel_resource_attrs(
        std::env::var("OTEL_RESOURCE_ATTRIBUTES")
            .unwrap_or_default()
            .as_str(),
    );
    let mut builder = Resource::builder()
        .with_service_name(service_name)
        .with_attribute(KeyValue::new(
            "service.instance.id",
            Uuid::new_v4().to_string(),
        ));
    for (k, v) in attrs {
        builder = builder.with_attribute(KeyValue::new(k, v));
    }
    builder.build()
}

/// Build the OTel tracer provider, install W3C as the global propagator,
/// register the provider globally, and return it (plus a tracer handle
/// for `tracing-opentelemetry`) so `main` can wire logging and shut the
/// provider down cleanly. Modelled on opentelemetry-rust/examples/tracing-grpc.
///
/// The HTTP/protobuf exporter is used so traces and metrics share a
/// single OTLP endpoint (:4318). `.with_endpoint()` for HTTP takes the
/// full URL — `/v1/traces` must be appended explicitly.
pub fn init_tracer() -> Result<(SdkTracerProvider, Tracer), BoxError> {
    global::set_text_map_propagator(TraceContextPropagator::new());

    let endpoint = std::env::var("OTEL_EXPORTER_OTLP_ENDPOINT")
        .unwrap_or_else(|_| "http://otel-collector:4318".to_string());

    let exporter = SpanExporter::builder()
        .with_http()
        .with_endpoint(format!("{}/v1/traces", endpoint))
        .build()?;

    let provider = SdkTracerProvider::builder()
        .with_batch_exporter(exporter)
        .with_resource(build_resource())
        .build();

    global::set_tracer_provider(provider.clone());
    let tracer = provider.tracer("server");
    Ok((provider, tracer))
}

/// Build the OTel meter provider with a periodic-pushing HTTP/protobuf
/// exporter aimed at the same `:4318` collector endpoint as traces.
/// Registers globally so `global::meter("...")` returns a real meter.
pub fn init_meter() -> Result<SdkMeterProvider, BoxError> {
    let endpoint = std::env::var("OTEL_EXPORTER_OTLP_ENDPOINT")
        .unwrap_or_else(|_| "http://otel-collector:4318".to_string());

    let exporter = MetricExporter::builder()
        .with_http()
        .with_endpoint(format!("{}/v1/metrics", endpoint))
        .build()?;

    let reader = PeriodicReader::builder(exporter).build();
    let provider = SdkMeterProvider::builder()
        .with_reader(reader)
        .with_resource(build_resource())
        .build();

    global::set_meter_provider(provider.clone());
    Ok(provider)
}

/// Holds the entered root span for the lifetime of the program.
pub struct RootSpanGuard(#[allow(dead_code)] tracing::span::EnteredSpan);

/// Install the global `tracing` subscriber and open the process-wide
/// root span. Returns a guard that must outlive every event emission.
///
/// Identity attributes (`service.name`, `module_name`, `owned_by`) are no
/// longer embedded in the root span — Vector enriches log events from
/// container labels at the agent boundary (see
/// `docs/engineering/rules/logging-platform.md` §3). The root span still
/// exists so the OTel context is active for the process lifetime.
///
/// Call **after** `init_tracer` so the `tracing-opentelemetry` layer
/// can attach to the global tracer provider.
pub fn init_logging(tracer: Tracer) -> RootSpanGuard {
    use tracing_subscriber::{fmt, prelude::*, EnvFilter};

    tracing_subscriber::registry()
        .with(EnvFilter::try_from_default_env().unwrap_or_else(|_| "info".into()))
        .with(
            fmt::layer()
                .json()
                .with_current_span(true)
                .with_span_list(false)
                .flatten_event(true),
        )
        .with(tracing_opentelemetry::layer().with_tracer(tracer))
        .init();

    let span = tracing::info_span!("app");
    RootSpanGuard(span.entered())
}

/// Adapter: lets the OTel propagator read gRPC headers off a tonic
/// `MetadataMap`. Inlined from the upstream `tracing-grpc` example.
pub(crate) struct MetadataMap<'a>(pub(crate) &'a TonicMetadataMap);

impl Extractor for MetadataMap<'_> {
    fn get(&self, key: &str) -> Option<&str> {
        self.0.get(key).and_then(|v| v.to_str().ok())
    }
    fn keys(&self) -> Vec<&str> {
        self.0
            .keys()
            .map(|k| match k {
                KeyRef::Ascii(v) => v.as_str(),
                KeyRef::Binary(v) => v.as_str(),
            })
            .collect()
    }
}

#[cfg(test)]
mod tests {
    use super::parse_otel_resource_attrs;

    #[test]
    fn parses_typical_compose_value() {
        let m = parse_otel_resource_attrs("module_name=app,owned_by=alice@example.com");
        assert_eq!(m.get("module_name").map(String::as_str), Some("app"));
        assert_eq!(m.get("owned_by").map(String::as_str), Some("alice@example.com"));
    }

    #[test]
    fn tolerates_whitespace_and_empty_entries() {
        let m = parse_otel_resource_attrs("  module_name=app , ,owned_by=bob ");
        assert_eq!(m.get("module_name").map(String::as_str), Some("app"));
        assert_eq!(m.get("owned_by").map(String::as_str), Some("bob"));
        assert_eq!(m.len(), 2);
    }

    #[test]
    fn returns_empty_for_empty_input() {
        assert!(parse_otel_resource_attrs("").is_empty());
    }

    #[test]
    fn ignores_malformed_entries() {
        let m = parse_otel_resource_attrs("good=1,nogood,also=ok");
        assert_eq!(m.len(), 2);
        assert_eq!(m.get("good").map(String::as_str), Some("1"));
        assert_eq!(m.get("also").map(String::as_str), Some("ok"));
    }
}
