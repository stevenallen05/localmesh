//! Tower `Layer` that logs non-OK gRPC `Status` results at `warn!`/`error!`
//! by code class. Logging happens inside the inner service's future so the
//! active span (set by `TraceContextLayer`) is entered and the
//! `tracing-opentelemetry` bridge attaches `trace_id`/`span_id` to the event.

use std::task::{Context, Poll};

use http::HeaderMap;
use opentelemetry::trace::TraceContextExt as _;
use tonic::Code;
use tower::{Layer, Service};
use tracing::{error, warn, Level};
use tracing_opentelemetry::OpenTelemetrySpanExt as _;

/// gRPC code class → tracing level. Client errors (4xx-equivalent) and
/// transient/load errors (UNAVAILABLE / DEADLINE_EXCEEDED / etc.) get
/// `warn!`; genuine server errors (INTERNAL / UNKNOWN / DATA_LOSS / etc.)
/// get `error!`. `OK` returns `None` — the layer doesn't log success.
pub(crate) fn code_to_level(code: Code) -> Option<Level> {
    match code {
        Code::Ok => None,
        // Client errors (4xx-equivalent) — caller did something wrong.
        Code::InvalidArgument
        | Code::NotFound
        | Code::AlreadyExists
        | Code::FailedPrecondition
        | Code::OutOfRange
        | Code::PermissionDenied
        | Code::Unauthenticated => Some(Level::WARN),
        // Transient / load — service is fine, conditions aren't.
        Code::Unavailable
        | Code::DeadlineExceeded
        | Code::Aborted
        | Code::Cancelled
        | Code::ResourceExhausted => Some(Level::WARN),
        // Server errors (5xx-equivalent) — the service is broken.
        Code::Internal
        | Code::Unknown
        | Code::DataLoss
        | Code::Unimplemented => Some(Level::ERROR),
    }
}

#[derive(Clone, Default)]
pub struct ErrorLogLayer;

impl<S> Layer<S> for ErrorLogLayer {
    type Service = ErrorLog<S>;
    fn layer(&self, inner: S) -> Self::Service {
        ErrorLog { inner }
    }
}

#[derive(Clone)]
pub struct ErrorLog<S> {
    inner: S,
}

/// Read `grpc-status` off the response **headers** (trailers-only path).
/// Caveat: for streaming/unary responses that emit a body, the gRPC status
/// lives in trailers, not headers. The demo's error paths all return
/// `Err(Status::…)` from handlers before any body is sent, which tonic
/// transmits as a trailers-only response — those land in headers and we
/// catch them. Mid-stream errors are out of scope; a richer implementation
/// would poll the body's trailers via `http_body::Body::poll_trailers`.
fn extract_grpc_code(headers: &HeaderMap) -> tonic::Code {
    let raw = headers
        .get("grpc-status")
        .and_then(|v| v.to_str().ok())
        .and_then(|s| s.parse::<i32>().ok())
        .unwrap_or(0);
    tonic::Code::from_i32(raw)
}

impl<S, ReqBody, ResBody> Service<http::Request<ReqBody>> for ErrorLog<S>
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
        let path = req.uri().path().to_string();
        // Standard tower idiom — clone + replace so we call the readied
        // service, not a stale handle. Mirrors rpc_metrics.rs.
        let clone = self.inner.clone();
        let mut inner = std::mem::replace(&mut self.inner, clone);
        Box::pin(async move {
            let resp = inner.call(req).await?;
            let code = extract_grpc_code(resp.headers());
            if let Some(level) = code_to_level(code) {
                // `tracing-opentelemetry` keeps the OTel span context on
                // the active tracing span, but the JSON formatter doesn't
                // auto-extract trace_id / span_id into event records. Pull
                // them out manually so the Loki event carries the IDs and
                // the Grafana derived-field can navigate to Tempo.
                let cx = tracing::Span::current().context();
                let sc = cx.span().span_context().clone();
                let trace_id = sc.trace_id().to_string();
                let span_id = sc.span_id().to_string();
                match level {
                    Level::ERROR => error!(
                        rpc.path = %path, code = ?code,
                        trace_id = %trace_id, span_id = %span_id,
                        "gRPC handler error"
                    ),
                    Level::WARN => warn!(
                        rpc.path = %path, code = ?code,
                        trace_id = %trace_id, span_id = %span_id,
                        "gRPC handler warning"
                    ),
                    _ => {}
                }
            }
            Ok(resp)
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tonic::Code;
    use tracing::Level;

    #[test]
    fn ok_is_not_logged() {
        assert_eq!(code_to_level(Code::Ok), None);
    }

    #[test]
    fn client_errors_warn() {
        for c in [
            Code::InvalidArgument,
            Code::NotFound,
            Code::AlreadyExists,
            Code::FailedPrecondition,
            Code::OutOfRange,
            Code::PermissionDenied,
            Code::Unauthenticated,
        ] {
            assert_eq!(code_to_level(c), Some(Level::WARN), "{c:?}");
        }
    }

    #[test]
    fn server_errors_error() {
        for c in [
            Code::Internal,
            Code::Unknown,
            Code::DataLoss,
            Code::Unimplemented,
        ] {
            assert_eq!(code_to_level(c), Some(Level::ERROR), "{c:?}");
        }
    }

    #[test]
    fn transient_errors_warn() {
        for c in [
            Code::Unavailable,
            Code::DeadlineExceeded,
            Code::Aborted,
            Code::Cancelled,
            Code::ResourceExhausted,
        ] {
            assert_eq!(code_to_level(c), Some(Level::WARN), "{c:?}");
        }
    }

    fn headers_with_status(s: &str) -> http::HeaderMap {
        let mut h = http::HeaderMap::new();
        h.insert("grpc-status", s.parse().unwrap());
        h
    }

    #[test]
    fn extract_missing_header_is_ok() {
        assert_eq!(extract_grpc_code(&http::HeaderMap::new()), Code::Ok);
    }

    #[test]
    fn extract_numeric_status_parses() {
        assert_eq!(extract_grpc_code(&headers_with_status("5")), Code::NotFound);
        assert_eq!(extract_grpc_code(&headers_with_status("13")), Code::Internal);
    }

    #[test]
    fn extract_non_numeric_falls_back_to_ok() {
        assert_eq!(extract_grpc_code(&headers_with_status("abc")), Code::Ok);
    }
}
