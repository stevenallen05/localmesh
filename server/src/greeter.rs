use std::sync::Arc;

use opentelemetry::trace::TraceContextExt as _;
use tonic::{Request, Response, Status};
use tracing_opentelemetry::OpenTelemetrySpanExt as _;

use crate::db::{Db, DbError};
use crate::identity::{PeerIdentity, User};
use crate::proto::hello::greeter_server::Greeter;
use crate::proto::hello::{HelloReply, HelloRequest, PostgresStatsReply, WhoAmI};

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
        // Pull identity facts from request extensions (set by the user +
        // peer interceptors); they go into the response payload, NOT into
        // span attributes — PII-at-ingress rule (spec §9.3).
        let user = req.extensions().get::<User>().cloned();
        let peer = req.extensions().get::<PeerIdentity>().cloned();

        tracing::debug!(
            user.id = user.as_ref().map(|u| u.id.as_str()).unwrap_or(""),
            peer.uri = peer.as_ref().map(|p| p.spiffe_uri.as_str()).unwrap_or(""),
            "handling say_hello"
        );

        // Current OTel trace id, hex-encoded, so the response payload can
        // datalink into Tempo.
        let trace_id = tracing::Span::current()
            .context()
            .span()
            .span_context()
            .trace_id()
            .to_string();

        let who = WhoAmI {
            mtls_peer_uri: peer.map(|p| p.spiffe_uri).unwrap_or_default(),
            user_id:       user.as_ref().map(|u| u.id.clone()).unwrap_or_default(),
            user_email:    user.as_ref().map(|u| u.email.clone()).unwrap_or_default(),
            trace_id,
        };

        let name = req.into_inner().name;
        let name = if name.is_empty() { "world" } else { &name };
        Ok(Response::new(HelloReply {
            message:  format!("hello, {name}"),
            who_am_i: Some(who),
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
