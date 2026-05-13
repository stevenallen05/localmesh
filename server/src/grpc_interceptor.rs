use opentelemetry::global;
use opentelemetry::propagation::Extractor;
use opentelemetry::Context;
use tonic::metadata::{KeyRef, MetadataMap};

/// Wraps a tonic `MetadataMap` so the OTel propagator can read B3 / W3C
/// headers off the incoming gRPC request.
pub struct MetadataExtractor<'a>(pub &'a MetadataMap);

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

/// Extract the upstream OpenTelemetry context from incoming gRPC metadata.
/// Call from inside the handler body (not from a tonic interceptor — the
/// interceptor runs before the handler's #[instrument] span is created, so
/// set_parent there has nothing to attach to).
pub fn parent_context(metadata: &MetadataMap) -> Context {
    global::get_text_map_propagator(|prop| {
        prop.extract(&MetadataExtractor(metadata))
    })
}
