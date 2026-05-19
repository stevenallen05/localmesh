use std::net::SocketAddr;
use std::sync::Arc;

use opentelemetry::global;
use tonic::transport::Server;

use server::db::Db;
use server::error_log::ErrorLogLayer;
use server::greeter::GreeterSvc;
use server::identity;
use server::proto::hello::greeter_server::GreeterServer;
use server::rpc_metrics::RpcMetricsLayer;
use server::telemetry::{init_logging, init_meter, init_tracer, BoxError};
use server::trace_context::TraceContextLayer;

#[tokio::main]
async fn main() -> Result<(), BoxError> {
    let (tracer_provider, tracer) = init_tracer()?;
    let meter_provider = init_meter()?;
    // _root keeps the process-wide span alive until main() returns.
    let _root = init_logging(tracer);

    tracing::info!("server starting");

    // Plaintext on the parent's loopback. Sidecar `server-inbound`
    // (ghostunnel) terminates mTLS on :50051 and forwards plain TCP
    // here. App + sidecar share the same network namespace via
    // `network_mode: "service:server"`.
    let addr: SocketAddr = std::env::var("SERVER_BIND_ADDR")
        .unwrap_or_else(|_| "127.0.0.1:50052".to_string())
        .parse()?;

    // Postgres first: fail-fast if the pool can't connect or migrations
    // can't apply. A half-migrated server serving traffic is worse than
    // a clean exit.
    let db = Arc::new(Db::connect_from_env().await?);
    db.run_migrations().await?;
    tracing::info!("postgres pool ready, migrations applied");

    tracing::info!(address = %addr, "listening");

    let metrics_layer = RpcMetricsLayer::new(&global::meter("server.rpc"));

    // Layer order — first .layer() call is the outermost. TraceContextLayer
    // must come first so a span is entered before inner layers can attach
    // events to it via the tracing-opentelemetry bridge.
    Server::builder()
        .layer(TraceContextLayer)
        .layer(metrics_layer)
        .layer(ErrorLogLayer)
        .add_service(GreeterServer::with_interceptor(
            GreeterSvc::new(db),
            identity::auth_interceptor,
        ))
        .serve(addr)
        .await?;

    drop(_root);
    tracer_provider.shutdown()?;
    // Meter shutdown's `OTelSdkResult` doesn't impl `Into<BoxError>`.
    meter_provider
        .shutdown()
        .map_err(|e| -> BoxError { e.to_string().into() })?;
    Ok(())
}
