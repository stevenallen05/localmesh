use std::net::SocketAddr;
use std::sync::Arc;

use opentelemetry::global;
use tonic::transport::Server;

use server::catalog::CatalogSvc;
use server::db::Db;
use server::error_log::ErrorLogLayer;
use server::grafana::GrafanaClient;
use server::greeter::GreeterSvc;
use server::identity;
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
    // Inbound mTLS: server's leaf cert + LocalMesh CA for client verify-CA.
    // See server/src/tls.rs.
    let tls = server::tls::server_tls_config()?;

    // JWKS — prime against Dex over the docker-network plaintext URL;
    // background task refreshes every 15 min. iss/aud match what
    // Dex stamps + what oauth2-proxy negotiated.
    let dex_jwks_url = std::env::var("DEX_JWKS_URL")
        .unwrap_or_else(|_| "http://dex:5556/dex/keys".to_string());
    let dex_issuer = std::env::var("DEX_ISSUER").unwrap_or_else(|_| {
        format!(
            "https://dex.{}.{}:8443/dex",
            std::env::var("PROJECT_NAME").unwrap_or_default(),
            std::env::var("LOCAL_DOMAIN").unwrap_or_default(),
        )
    });
    let oidc_aud =
        std::env::var("OIDC_AUDIENCE").unwrap_or_else(|_| "localmesh-dev".to_string());

    let jwks = Arc::new(server::auth::Jwks::new(&dex_issuer, &oidc_aud));
    jwks.fetch_from(&dex_jwks_url)
        .await
        .map_err(|e| -> BoxError { format!("JWKS prime failed: {e}").into() })?;
    tracing::info!(jwks_url = %dex_jwks_url, "JWKS primed");
    let _jwks_refresher = jwks
        .clone()
        .spawn_refresher(dex_jwks_url.clone(), std::time::Duration::from_secs(15 * 60));

    Server::builder()
        .tls_config(tls)?
        .layer(TraceContextLayer)
        .layer(metrics_layer)
        .layer(ErrorLogLayer)
        // Compose auth_interceptor + peer_interceptor at each service entry
        // point. Both insert into request extensions; handlers read them
        // for business logic without echoing PII onto OTel spans (the
        // PII-at-ingress rule lives at the Caddy edge via enduser_attrs).
        // JWKS instantiated above; primed once at startup, refreshed every
        // 15 min by a background task. The Arc clone is cheap per request;
        // the cache read is sync.
        .add_service(GreeterServer::with_interceptor(
            GreeterSvc::new(db),
            {
                let auth = identity::auth_interceptor(jwks.clone());
                // `Status` is large but boxing it would break tonic's
                // interceptor signature; same trade-off documented in
                // identity::auth_interceptor.
                #[allow(clippy::result_large_err)]
                let f = move |req| identity::peer_interceptor(auth(req)?);
                f
            },
        ))
        .add_service(CatalogServer::with_interceptor(
            CatalogSvc::new(grafana),
            {
                let auth = identity::auth_interceptor(jwks.clone());
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
