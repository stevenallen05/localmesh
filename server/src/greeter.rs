use std::sync::Arc;

use opentelemetry::global;
use opentelemetry::trace::{Span, SpanKind, Tracer};
use opentelemetry::KeyValue;
use tonic::{Request, Response, Status};

use crate::db::{Db, DbError};
use crate::proto::hello::greeter_server::Greeter;
use crate::proto::hello::{HelloReply, HelloRequest, PostgresStatsReply};
use crate::telemetry::MetadataMap;

pub struct GreeterSvc {
    db: Arc<Db>,
}

impl GreeterSvc {
    pub fn new(db: Arc<Db>) -> Self {
        Self { db }
    }
}

// db.rs is tonic-free by design; the mapping to gRPC status lives here, where
// the orphan rule allows it (DbError is local to this crate).
impl From<DbError> for Status {
    fn from(e: DbError) -> Self {
        if let DbError::Sqlx(inner) = &e {
            if matches!(
                inner,
                sqlx::Error::PoolTimedOut
                    | sqlx::Error::PoolClosed
                    | sqlx::Error::Io(_)
                    | sqlx::Error::Tls(_)
            ) {
                return Status::unavailable(e.to_string());
            }
        }
        Status::internal(e.to_string())
    }
}

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
        req: Request<()>,
    ) -> Result<Response<PostgresStatsReply>, Status> {
        // Same span-creation pattern as say_hello — kept inline rather than
        // lifted into a shared helper, per the spec (don't half-migrate a
        // third call site).
        let parent_cx =
            global::get_text_map_propagator(|p| p.extract(&MetadataMap(req.metadata())));
        let tracer = global::tracer("server");
        let span = tracer
            .span_builder("Greeter/print_postgres_stats")
            .with_kind(SpanKind::Server)
            .with_attributes([
                KeyValue::new("rpc.system", "grpc"),
                KeyValue::new("rpc.method", "print_postgres_stats"),
            ])
            .start_with_context(&tracer, &parent_cx);

        // Clone the SpanContext now so the borrow on `span` ends before we
        // move into the async DB calls. `span` lives to end of scope and
        // exports on Drop after the Ok(...) expression evaluates.
        let span_ctx = span.span_context().clone();

        self.db.record_noise_event("button_press", &span_ctx).await?;
        let stats = self.db.top_stats(&span_ctx).await?;

        let _keep_span_alive = span;

        Ok(Response::new(PostgresStatsReply {
            num_backends:    stats.num_backends,
            xact_commit:     stats.xact_commit,
            xact_rollback:   stats.xact_rollback,
            cache_hit_ratio: stats.cache_hit_ratio,
            db_size_bytes:   stats.db_size_bytes,
        }))
    }
}
