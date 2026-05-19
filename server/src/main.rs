use std::net::SocketAddr;
use std::sync::Arc;

use opentelemetry::global;
use tonic::transport::Server;

use server::auth::Jwks;
use server::db::Db;
use server::error_log::ErrorLogLayer;
use server::greeter::GreeterSvc;
use server::identity;
use server::proto::hello::greeter_server::GreeterServer;
use server::rpc_metrics::RpcMetricsLayer;
use server::telemetry::{init_logging, init_meter, init_tracer, BoxError};
use server::trace_context::TraceContextLayer;

/// Build a primed JWKS cache from env vars and launch the background
/// refresher (every 15 min). Returns the cache (shared with interceptors)
/// and the refresher's `JoinHandle`. Caller binds the handle to a local
/// so its `Drop` runs at process exit — dropping cancels the task.
async fn jwks_from_env() -> Result<(Arc<Jwks>, tokio::task::JoinHandle<()>), BoxError> {
    let url = std::env::var("DEX_JWKS_URL")
        .unwrap_or_else(|_| "http://dex:5556/dex/keys".to_string());
    let issuer = std::env::var("DEX_ISSUER").unwrap_or_else(|_| {
        format!(
            "https://dex.{}.{}:8443/dex",
            std::env::var("PROJECT_NAME").unwrap_or_default(),
            std::env::var("LOCAL_DOMAIN").unwrap_or_default(),
        )
    });
    let audience = std::env::var("OIDC_AUDIENCE").unwrap_or_else(|_| "localmesh-dev".to_string());

    let jwks = Arc::new(Jwks::new(&issuer, &audience));
    jwks.fetch_from(&url)
        .await
        .map_err(|e| -> BoxError { format!("JWKS prime failed: {e}").into() })?;
    tracing::info!(jwks_url = %url, "JWKS primed");

    let handle = jwks
        .clone()
        .spawn_refresher(url, std::time::Duration::from_secs(15 * 60));
    Ok((jwks, handle))
}

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

    tracing::info!(address = %addr, "listening");

    let metrics_layer = RpcMetricsLayer::new(&global::meter("server.rpc"));

    // Layer order — first .layer() call is the outermost. TraceContextLayer
    // must come first so a span is entered before any inner layer can emit
    // events under it (the error-log layer attaches its WARN/ERROR
    // emissions to the active span via the tracing-opentelemetry bridge).
    // Inbound mTLS: server's leaf cert + LocalMesh CA for client verify-CA.
    // See server/src/tls.rs.
    let tls = server::tls::server_tls_config()?;

    let (jwks, _jwks_refresher) = jwks_from_env().await?;

    Server::builder()
        .tls_config(tls)?
        .layer(TraceContextLayer)
        .layer(metrics_layer)
        .layer(ErrorLogLayer)
        // auth_interceptor (verifies JWT → User + ClaimsForDb extensions) is
        // composed with peer_interceptor (mTLS SPIFFE URI → PeerIdentity).
        // Handlers read extensions for business logic without echoing PII
        // onto OTel spans (the PII-at-ingress rule lives at the Caddy edge
        // via enduser_attrs). Arc clone is cheap; JWKS cache read is sync.
        .add_service(GreeterServer::with_interceptor(
            GreeterSvc::new(db),
            {
                let auth = identity::auth_interceptor(jwks.clone());
                // `Status` is large but boxing it would break tonic's
                // interceptor signature; same trade-off as
                // identity::auth_interceptor.
                #[allow(clippy::result_large_err)]
                let f = move |req| identity::peer_interceptor(auth(req)?);
                f
            },
        ))
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
