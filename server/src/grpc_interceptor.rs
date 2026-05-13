use opentelemetry::global;
use opentelemetry::propagation::Extractor;
use tonic::metadata::KeyRef;
use tonic::{metadata::MetadataMap, Request, Status};
use tracing::Span;
use tracing_opentelemetry::OpenTelemetrySpanExt;

struct MetadataExtractor<'a>(&'a MetadataMap);

impl<'a> Extractor for MetadataExtractor<'a> {
    fn get(&self, key: &str) -> Option<&str> {
        self.0.get(key).and_then(|v| v.to_str().ok())
    }
    fn keys(&self) -> Vec<&str> {
        self.0
            .keys()
            .filter_map(|k| match k {
                KeyRef::Ascii(name) => Some(name.as_str()),
                KeyRef::Binary(_) => None,
            })
            .collect()
    }
}

/// Tonic interceptor — extracts B3 headers from incoming metadata and
/// attaches the propagated OpenTelemetry context to the current span so
/// the handler's `#[instrument]`-created span becomes a child of the
/// upstream trace.
pub fn extract_trace_context<T>(req: Request<T>) -> Result<Request<T>, Status> {
    let cx = global::get_text_map_propagator(|prop| {
        prop.extract(&MetadataExtractor(req.metadata()))
    });
    let _ = Span::current().set_parent(cx);
    Ok(req)
}
