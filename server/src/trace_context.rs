//! Tower `Layer` that extracts the W3C parent context from inbound gRPC
//! metadata, builds a server-kind span, and enters it for the duration of
//! the inner service's future. Replaces the hand-rolled `span_builder`
//! blocks that used to live at the top of every handler.

use std::task::{Context, Poll};

use opentelemetry::global;
use opentelemetry::trace::{SpanKind, TraceContextExt as _, Tracer as _};
use opentelemetry::KeyValue;
use tower::{Layer, Service};
use tracing::Instrument as _;
use tracing_opentelemetry::OpenTelemetrySpanExt as _;

use crate::rpc_metrics::split_grpc_path;
use crate::telemetry::MetadataMap;

#[derive(Clone, Default)]
pub struct TraceContextLayer;

impl<S> Layer<S> for TraceContextLayer {
    type Service = TraceContext<S>;
    fn layer(&self, inner: S) -> Self::Service {
        TraceContext { inner }
    }
}

#[derive(Clone)]
pub struct TraceContext<S> {
    inner: S,
}

impl<S, ReqBody, ResBody> Service<http::Request<ReqBody>> for TraceContext<S>
where
    S: Service<http::Request<ReqBody>, Response = http::Response<ResBody>>
        + Clone
        + Send
        + 'static,
    S::Future: Send + 'static,
    ReqBody: Send + 'static,
    ResBody: Send + 'static,
{
    type Response = S::Response;
    type Error = S::Error;
    type Future = std::pin::Pin<
        Box<dyn std::future::Future<Output = Result<Self::Response, Self::Error>> + Send>,
    >;

    fn poll_ready(&mut self, cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
        self.inner.poll_ready(cx)
    }

    fn call(&mut self, req: http::Request<ReqBody>) -> Self::Future {
        // The W3C propagator needs a tonic MetadataMap to extract from.
        // Build one from the inbound http headers.
        let tonic_meta = tonic::metadata::MetadataMap::from_headers(req.headers().clone());
        let parent_cx =
            global::get_text_map_propagator(|p| p.extract(&MetadataMap(&tonic_meta)));

        let path = req.uri().path().to_string();
        let (svc, method) = split_grpc_path(&path);
        let tracer = global::tracer("server");
        let otel_span = tracer
            .span_builder(format!("{svc}/{method}"))
            .with_kind(SpanKind::Server)
            .with_attributes([
                KeyValue::new("rpc.system", "grpc"),
                KeyValue::new("rpc.service", svc.to_string()),
                KeyValue::new("rpc.method", method.to_string()),
            ])
            .start_with_context(&tracer, &parent_cx);

        // Bridge: tracing span inherits the OTel context. `#[instrument]`
        // on the handler then adds a child tracing span; the
        // tracing-opentelemetry layer keeps the OTel context active for
        // events emitted under that span (so `tracing::info!` lines carry
        // trace_id/span_id automatically).
        let tracing_span = tracing::info_span!("rpc.server");
        // `set_parent` returns `()` on infallible paths and a Result in some
        // tracing-opentelemetry 0.32 builds; `let _` covers both.
        let _ = tracing_span.set_parent(opentelemetry::Context::current_with_span(otel_span));

        let clone = self.inner.clone();
        let mut inner = std::mem::replace(&mut self.inner, clone);
        Box::pin(async move { inner.call(req).await }.instrument(tracing_span))
    }
}
