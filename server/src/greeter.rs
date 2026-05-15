use opentelemetry::global;
use opentelemetry::trace::{SpanKind, Tracer};
use opentelemetry::KeyValue;
use tonic::{Request, Response, Status};

use crate::proto::hello::greeter_server::Greeter;
use crate::proto::hello::{HelloReply, HelloRequest};
use crate::telemetry::MetadataMap;

pub struct GreeterSvc;

#[tonic::async_trait]
impl Greeter for GreeterSvc {
    async fn say_hello(&self, req: Request<HelloRequest>) -> Result<Response<HelloReply>, Status> {
        // Extract the upstream W3C context off the incoming metadata, then
        // start a server-kind span as a child of it. The named `_span`
        // binding keeps the span alive until end-of-scope — binding to bare
        // `_` would drop it immediately and record zero duration.
        let parent_cx =
            global::get_text_map_propagator(|p| p.extract(&MetadataMap(req.metadata())));
        let tracer = global::tracer("server");
        let _span = tracer
            .span_builder("Greeter/say_hello")
            .with_kind(SpanKind::Server)
            .with_attributes([
                KeyValue::new("rpc.system", "grpc"),
                KeyValue::new("rpc.method", "say_hello"),
            ])
            .start_with_context(&tracer, &parent_cx);

        let name = req.into_inner().name;
        let name = if name.is_empty() { "world" } else { &name };
        Ok(Response::new(HelloReply {
            message: format!("hello, {name}"),
        }))
    }

    async fn print_postgres_stats(
        &self,
        _req: Request<()>,
    ) -> Result<Response<crate::proto::hello::PostgresStatsReply>, Status> {
        // Stub — wired to postgres in the next commit.
        Err(Status::unimplemented("print_postgres_stats not yet wired"))
    }
}
