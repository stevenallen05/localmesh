use std::net::SocketAddr;
use std::sync::Arc;

use tonic::transport::Server;

use server::catalog::CatalogSvc;
use server::db::Db;
use server::grafana::GrafanaClient;
use server::greeter::GreeterSvc;
use server::proto::hello::greeter_server::GreeterServer;
use server::proto::metrics_v1::catalog_server::CatalogServer;
use server::telemetry::{init_tracer, BoxError};

#[tokio::main]
async fn main() -> Result<(), BoxError> {
    let (provider, _tracer) = init_tracer()?;

    let addr: SocketAddr = std::env::var("SERVER_ADDR")
        .unwrap_or_else(|_| "0.0.0.0:50051".to_string())
        .parse()?;

    // Postgres first: fail-fast if the pool can't connect or migrations
    // can't apply. A half-migrated server serving traffic is worse than
    // a clean exit.
    let db = Arc::new(Db::connect_from_env().await?);
    db.run_migrations().await?;
    println!("server: postgres pool ready, migrations applied");

    let grafana = Arc::new(GrafanaClient::from_env().await?);

    println!("server: listening on {addr}");

    Server::builder()
        .add_service(GreeterServer::new(GreeterSvc::new(db)))
        .add_service(CatalogServer::new(CatalogSvc::new(grafana)))
        .serve(addr)
        .await?;

    provider.shutdown()?;
    Ok(())
}
