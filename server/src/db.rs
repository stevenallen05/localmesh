//! Postgres client wrapper. Owns the pool, runs migrations, exposes the
//! INSERT used by `Greeter.SayHello`. No `tonic` dependency — the gRPC
//! error mapping lives in `greeter.rs` where the orphan rule allows it.

use opentelemetry::trace::SpanContext;
use sqlx::postgres::{PgPool, PgPoolOptions};

#[derive(Debug)]
pub enum DbError {
    Sqlx(sqlx::Error),
    Migrate(sqlx::migrate::MigrateError),
}

impl std::fmt::Display for DbError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Sqlx(e)    => write!(f, "db: {e}"),
            Self::Migrate(e) => write!(f, "db migrate: {e}"),
        }
    }
}

impl std::error::Error for DbError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            Self::Sqlx(e)    => Some(e),
            Self::Migrate(e) => Some(e),
        }
    }
}

impl From<sqlx::Error> for DbError {
    fn from(e: sqlx::Error) -> Self { Self::Sqlx(e) }
}

impl From<sqlx::migrate::MigrateError> for DbError {
    fn from(e: sqlx::migrate::MigrateError) -> Self { Self::Migrate(e) }
}

pub struct Db {
    pool: PgPool,
}

impl Db {
    /// Reads `DATABASE_URL` from the environment and opens a pool (default
    /// sqlx pool sizing). Fails if the env var is missing or the initial
    /// connection probe fails.
    pub async fn connect_from_env() -> Result<Self, DbError> {
        let url = std::env::var("DATABASE_URL").map_err(|_| {
            DbError::Sqlx(sqlx::Error::Configuration(
                "DATABASE_URL not set".into(),
            ))
        })?;
        let pool = PgPoolOptions::new().connect(&url).await?;
        Ok(Self { pool })
    }

    /// Apply embedded migrations. Idempotent — sqlx tracks applied versions
    /// in `_sqlx_migrations`. Ok on a clean second run.
    pub async fn run_migrations(&self) -> Result<(), DbError> {
        sqlx::migrate!("./migrations").run(&self.pool).await?;
        Ok(())
    }

    /// Insert one hello_messages row. Prepends the traceparent comment so
    /// pg_tracing stitches parse/plan/exec spans under the gRPC server span.
    pub async fn record_hello_message(
        &self,
        message: &str,
        jwt_issuer: &str,
        jwt_subject: &str,
        parent: &SpanContext,
    ) -> Result<(), DbError> {
        let sql = format!(
            "{} INSERT INTO hello_messages (message, jwt_issuer, jwt_subject) VALUES ($1, $2, $3)",
            traceparent_comment_for(parent),
        );
        sqlx::query(&sql)
            .bind(message)
            .bind(jwt_issuer)
            .bind(jwt_subject)
            .execute(&self.pool)
            .await?;
        Ok(())
    }
}

/// Format a `SpanContext` as a pg_tracing-compatible SQL comment carrying a
/// W3C traceparent. The sampled-flag byte is **derived** from the context's
/// trace flags so an unsampled parent produces `00`, never `01`. Returns an
/// empty string when the context is invalid (no parent in scope) so callers
/// can prepend it unconditionally.
fn traceparent_comment_for(ctx: &SpanContext) -> String {
    if !ctx.is_valid() {
        return String::new();
    }
    let flag_byte = if ctx.is_sampled() { "01" } else { "00" };
    format!(
        "/*traceparent='00-{}-{}-{}'*/",
        ctx.trace_id(),
        ctx.span_id(),
        flag_byte,
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    use opentelemetry::trace::{SpanId, TraceFlags, TraceId, TraceState};

    fn ctx(trace_byte: u8, span_byte: u8, flags: TraceFlags) -> SpanContext {
        SpanContext::new(
            TraceId::from_bytes([trace_byte; 16]),
            SpanId::from_bytes([span_byte; 8]),
            flags,
            false,
            TraceState::default(),
        )
    }

    #[test]
    fn traceparent_comment_for_sampled_context_emits_flag_01() {
        let c = ctx(0x4b, 0xf0, TraceFlags::SAMPLED);
        assert_eq!(
            traceparent_comment_for(&c),
            "/*traceparent='00-4b4b4b4b4b4b4b4b4b4b4b4b4b4b4b4b-f0f0f0f0f0f0f0f0-01'*/"
        );
    }

    #[test]
    fn traceparent_comment_for_unsampled_context_emits_flag_00() {
        let c = ctx(0x01, 0x02, TraceFlags::default());
        assert_eq!(
            traceparent_comment_for(&c),
            "/*traceparent='00-01010101010101010101010101010101-0202020202020202-00'*/"
        );
    }

    #[test]
    fn traceparent_comment_for_invalid_context_emits_empty_string() {
        let c = SpanContext::empty_context();
        assert_eq!(traceparent_comment_for(&c), "");
    }
}
