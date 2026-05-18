use std::net::SocketAddr;
use std::sync::Arc;

use opentelemetry::global;
use tonic::transport::Server;

use server::catalog::CatalogSvc;
use server::db::Db;
use server::error_log::ErrorLogLayer;
use server::grafana::GrafanaClient;
use server::greeter::GreeterSvc;
use server::proto::hello::greeter_server::GreeterServer;
use server::proto::metrics_v1::catalog_server::CatalogServer;
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

    let addr: SocketAddr = std::env::var("SERVER_ADDR")
        .unwrap_or_else(|_| "0.0.0.0:50051".to_string())
        .parse()?;

    // Postgres first: fail-fast if the pool can't connect or migrations
    // can't apply. A half-migrated server serving traffic is worse than
    // a clean exit.
    let db = Arc::new(Db::connect_from_env().await?);
    db.run_migrations().await?;
    tracing::info!("postgres pool ready, migrations applied");

    let grafana = Arc::new(GrafanaClient::from_env().await?);

    tracing::info!(address = %addr, "listening");

    let metrics_layer = RpcMetricsLayer::new(&global::meter("server.rpc"));

    // Layer order — first .layer() call is the outermost. TraceContextLayer
    // must come first so a span is entered before any inner layer can emit
    // events under it (the error-log layer attaches its WARN/ERROR
    // emissions to the active span via the tracing-opentelemetry bridge).
    Server::builder()
        .layer(TraceContextLayer)
        .layer(metrics_layer)
        .layer(ErrorLogLayer)
        .add_service(GreeterServer::new(GreeterSvc::new(db)))
        .add_service(CatalogServer::new(CatalogSvc::new(grafana)))
        .serve(addr)
        .await?;

    // Drop the root span before shutting the OTel providers down so the
    // close event reaches still-live exporters. Shutdown is idempotent,
    // so order is cosmetic — but intentional. Meter shutdown returns
    // `OTelSdkResult`, which doesn't impl `Into<BoxError>`; map it via
    // `to_string()` since the error type is opaque to the caller.
    drop(_root);
    tracer_provider.shutdown()?;
    meter_provider
        .shutdown()
        .map_err(|e| -> BoxError { e.to_string().into() })?;
    Ok(())
}
