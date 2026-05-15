use std::collections::HashMap;

use opentelemetry::global;
use opentelemetry::propagation::Extractor;
use opentelemetry_otlp::{SpanExporter, WithExportConfig};
use opentelemetry_sdk::propagation::TraceContextPropagator;
use opentelemetry_sdk::trace::SdkTracerProvider;
use opentelemetry_sdk::Resource;
use tonic::metadata::{KeyRef, MetadataMap as TonicMetadataMap};

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

/// Build the OTel tracer provider, install W3C as the global propagator,
/// register the provider globally, and return it so `main` can shut it
/// down cleanly. Modelled on opentelemetry-rust/examples/tracing-grpc.
pub fn init_tracer() -> Result<SdkTracerProvider, BoxError> {
    global::set_text_map_propagator(TraceContextPropagator::new());

    let endpoint = std::env::var("OTEL_EXPORTER_OTLP_ENDPOINT")
        .unwrap_or_else(|_| "http://otel-collector:4317".to_string());

    let exporter = SpanExporter::builder()
        .with_tonic()
        .with_endpoint(endpoint)
        .build()?;

    let provider = SdkTracerProvider::builder()
        .with_batch_exporter(exporter)
        .with_resource(Resource::builder().with_service_name("server").build())
        .build();

    global::set_tracer_provider(provider.clone());
    Ok(provider)
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
