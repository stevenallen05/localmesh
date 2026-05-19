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
        // Read all extensions BEFORE consuming `req` via into_inner().
        // Identity facts (User, PeerIdentity) go into the response payload,
        // NOT into span attributes — PII-at-ingress rule (spec §9.3).
        // ClaimsForDb feeds the hello_messages INSERT below.
        let user = req.extensions().get::<User>().cloned();
        let peer = req.extensions().get::<PeerIdentity>().cloned();
        let claims_for_db = req.extensions()
            .get::<crate::identity::ClaimsForDb>()
            .cloned()
            .unwrap_or_default();

        tracing::debug!(
            user.id = user.as_ref().map(|u| u.id.as_str()).unwrap_or(""),
            peer.uri = peer.as_ref().map(|p| p.spiffe_uri.as_str()).unwrap_or(""),
            "handling say_hello"
        );

        // Current OTel SpanContext for pg_tracing trace-stitching AND the
        // WhoAmI response trace_id (one extraction, two consumers).
        let cx = tracing::Span::current().context();
        let span_ctx = cx.span().span_context().clone();
        let trace_id = span_ctx.trace_id().to_string();

        let who = WhoAmI {
            mtls_peer_uri: peer.map(|p| p.spiffe_uri).unwrap_or_default(),
            user_id:       user.as_ref().map(|u| u.id.clone()).unwrap_or_default(),
            user_email:    user.as_ref().map(|u| u.email.clone()).unwrap_or_default(),
            trace_id,
        };

        // Now safe to consume req.
        let name = req.into_inner().name;
        let name = if name.is_empty() { "world" } else { &name };
        let reply_message = format!("hello, {name}");

        // Persist verified JWT-derived identity into business data.
        // The PII-at-ingress rule constrains telemetry, not business data we
        // choose to write — see DESIGN_DECISIONS row :56.
        self.db.record_hello_message(
            &reply_message,
            &claims_for_db.iss,
            &claims_for_db.sub,
            &span_ctx,
        ).await?;

        Ok(Response::new(HelloReply {
            message:  reply_message,
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
