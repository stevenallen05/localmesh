use std::net::SocketAddr;
use std::sync::Arc;

use tonic::transport::Server;

use server::catalog::CatalogSvc;
use server::grafana::GrafanaClient;
use server::greeter::GreeterSvc;
use server::proto::hello::greeter_server::GreeterServer;
use server::proto::metrics_v1::catalog_server::CatalogServer;
use server::telemetry::{init_tracer, BoxError};

#[tokio::main]
async fn main() -> Result<(), BoxError> {
    let provider = init_tracer()?;

    let addr: SocketAddr = std::env::var("SERVER_ADDR")
        .unwrap_or_else(|_| "0.0.0.0:50051".to_string())
        .parse()?;

    let grafana = Arc::new(GrafanaClient::from_env().await?);

    println!("server: listening on {addr}");

    Server::builder()
        .add_service(GreeterServer::new(GreeterSvc))
        .add_service(CatalogServer::new(CatalogSvc::new(grafana)))
        .serve(addr)
        .await?;

    provider.shutdown()?;
    Ok(())
}
