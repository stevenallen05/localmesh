use std::sync::Arc;

use opentelemetry::trace::TraceContextExt as _;
use tonic::{Request, Response, Status};
use tracing_opentelemetry::OpenTelemetrySpanExt as _;

use crate::db::{Db, DbError};
use crate::proto::hello::greeter_server::Greeter;
use crate::proto::hello::{HelloReply, HelloRequest, PostgresStatsReply};

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
    #[tracing::instrument(skip_all, fields(rpc.method = "say_hello"))]
    async fn say_hello(&self, req: Request<HelloRequest>) -> Result<Response<HelloReply>, Status> {
        let name = req.into_inner().name;
        let name = if name.is_empty() { "world" } else { &name };
        Ok(Response::new(HelloReply {
            message: format!("hello, {name}"),
        }))
    }

    #[tracing::instrument(skip_all, fields(rpc.method = "print_postgres_stats"))]
    async fn print_postgres_stats(
        &self,
        _req: Request<()>,
    ) -> Result<Response<PostgresStatsReply>, Status> {
        // The OTel server span is set by TraceContextLayer + #[instrument];
        // pull its SpanContext for pg_tracing to stitch SQL spans under.
        // Bind the OTel context to a `let` so the SpanRef lifetime extends
        // across the .span_context() call.
        let cx = tracing::Span::current().context();
        let span_ctx = cx.span().span_context().clone();

        self.db.record_noise_event("button_press", &span_ctx).await?;
        let stats = self.db.top_stats(&span_ctx).await?;

        Ok(Response::new(PostgresStatsReply {
            num_backends:    stats.num_backends,
            xact_commit:     stats.xact_commit,
            xact_rollback:   stats.xact_rollback,
            cache_hit_ratio: stats.cache_hit_ratio,
            db_size_bytes:   stats.db_size_bytes,
        }))
    }
}
