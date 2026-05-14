use std::net::SocketAddr;

use tonic::transport::Server;

use server::greeter::GreeterSvc;
use server::proto::hello::greeter_server::GreeterServer;
use server::telemetry::{init_tracer, BoxError};

#[tokio::main]
async fn main() -> Result<(), BoxError> {
    let provider = init_tracer()?;

    let addr: SocketAddr = std::env::var("SERVER_ADDR")
        .unwrap_or_else(|_| "0.0.0.0:50051".to_string())
        .parse()?;
    println!("server: listening on {addr}");

    Server::builder()
        .add_service(GreeterServer::new(GreeterSvc))
        .serve(addr)
        .await?;

    provider.shutdown()?;
    Ok(())
}
