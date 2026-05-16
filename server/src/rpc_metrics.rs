//! Tower `Layer` that records `rpc.server.duration` (OTel semconv) per gRPC request.
//!
//! Inserted into tonic's `Server::builder().layer(...)`. The layer owns a
//! `Histogram<f64>` built once at startup; each request records its duration
//! tagged with `rpc.system`, `rpc.service`, `rpc.method`, `rpc.grpc.status_code`.

use std::task::{Context, Poll};
use std::time::Instant;

use opentelemetry::metrics::{Histogram, Meter};
use opentelemetry::KeyValue;
use tower::{Layer, Service};

/// Split a tonic request URI path (`/foo.Bar/Baz`) into `(service, method)`.
/// Returns `("", "")` for anything that doesn't match the gRPC shape — those
/// requests will record under empty `rpc.service` / `rpc.method` attrs rather
/// than crash the middleware.
pub(crate) fn split_grpc_path(path: &str) -> (&str, &str) {
    let trimmed = path.strip_prefix('/').unwrap_or(path);
    match trimmed.split_once('/') {
        Some((svc, method)) => (svc, method),
        None => ("", ""),
    }
}

#[derive(Clone)]
pub struct RpcMetricsLayer {
    histogram: Histogram<f64>,
}

impl RpcMetricsLayer {
    pub fn new(meter: &Meter) -> Self {
        let histogram = meter
            .f64_histogram("rpc.server.duration")
            .with_description("Duration of gRPC server requests")
            .with_unit("s")
            .build();
        Self { histogram }
    }
}

impl<S> Layer<S> for RpcMetricsLayer {
    type Service = RpcMetrics<S>;
    fn layer(&self, inner: S) -> Self::Service {
        RpcMetrics {
            inner,
            histogram: self.histogram.clone(),
        }
    }
}

#[derive(Clone)]
pub struct RpcMetrics<S> {
    inner: S,
    histogram: Histogram<f64>,
}

impl<S, Req, Res> Service<http::Request<Req>> for RpcMetrics<S>
where
    S: Service<http::Request<Req>, Response = http::Response<Res>> + Clone + Send + 'static,
    S::Future: Send + 'static,
    Req: Send + 'static,
    Res: Send + 'static,
{
    type Response = S::Response;
    type Error = S::Error;
    type Future = std::pin::Pin<
        Box<dyn std::future::Future<Output = Result<Self::Response, Self::Error>> + Send>,
    >;

    fn poll_ready(&mut self, cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
        self.inner.poll_ready(cx)
    }

    fn call(&mut self, req: http::Request<Req>) -> Self::Future {
        let start = Instant::now();
        let path = req.uri().path().to_string();
        let histogram = self.histogram.clone();
        // tonic services are Clone; cloning here is the standard tower idiom
        // for ensuring we call the readied service rather than a stale handle.
        let clone = self.inner.clone();
        let mut inner = std::mem::replace(&mut self.inner, clone);
        Box::pin(async move {
            let resp = inner.call(req).await?;
            let (svc, method) = split_grpc_path(&path);
            let status_code = resp
                .headers()
                .get("grpc-status")
                .and_then(|v| v.to_str().ok())
                .unwrap_or("0")
                .to_string();
            let attrs = [
                KeyValue::new("rpc.system", "grpc"),
                KeyValue::new("rpc.service", svc.to_string()),
                KeyValue::new("rpc.method", method.to_string()),
                KeyValue::new("rpc.grpc.status_code", status_code),
            ];
            histogram.record(start.elapsed().as_secs_f64(), &attrs);
            Ok(resp)
        })
    }
}

#[cfg(test)]
mod tests {
    use super::split_grpc_path;

    #[test]
    fn splits_typical_grpc_path() {
        assert_eq!(
            split_grpc_path("/hello.Greeter/SayHello"),
            ("hello.Greeter", "SayHello")
        );
    }

    #[test]
    fn splits_path_without_leading_slash() {
        assert_eq!(
            split_grpc_path("metrics.v1.Catalog/List"),
            ("metrics.v1.Catalog", "List")
        );
    }

    #[test]
    fn returns_empty_for_malformed() {
        assert_eq!(split_grpc_path("/no-slash"), ("", ""));
        assert_eq!(split_grpc_path(""), ("", ""));
    }
}
