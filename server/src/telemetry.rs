use opentelemetry::global;
use opentelemetry::propagation::Extractor;
use opentelemetry_otlp::{SpanExporter, WithExportConfig};
use opentelemetry_sdk::propagation::TraceContextPropagator;
use opentelemetry_sdk::trace::SdkTracerProvider;
use opentelemetry_sdk::Resource;
use tonic::metadata::{KeyRef, MetadataMap as TonicMetadataMap};

pub type BoxError = Box<dyn std::error::Error + Send + Sync>;

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
