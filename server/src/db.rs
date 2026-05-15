//! Postgres client wrapper. Owns the pool, runs migrations, exposes the two
//! queries the demo `PrintPostgresStats` RPC needs. No `tonic` dependency —
//! the gRPC error mapping lives in `greeter.rs` where the orphan rule allows it.

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

#[derive(Debug, sqlx::FromRow)]
pub struct TopStats {
    pub num_backends:    i32,
    pub xact_commit:     i64,
    pub xact_rollback:   i64,
    pub cache_hit_ratio: f64,
    pub db_size_bytes:   i64,
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

    /// Writes one row to `noise_events`. The traceparent comment is prepended
    /// so pg_tracing can stitch the INSERT under the caller's gRPC span.
    pub async fn record_noise_event(
        &self,
        kind: &str,
        parent: &SpanContext,
    ) -> Result<(), DbError> {
        let sql = format!(
            "{} INSERT INTO noise_events (kind) VALUES ($1)",
            traceparent_comment_for(parent),
        );
        sqlx::query(&sql).bind(kind).execute(&self.pool).await?;
        Ok(())
    }

    /// Reads the five headline `pg_stat_database` metrics for the current
    /// database. `numbackends` is cast to int4 to match `TopStats::num_backends`;
    /// `cache_hit_ratio` is guarded against divide-by-zero on a freshly reset
    /// stats view.
    pub async fn top_stats(&self, parent: &SpanContext) -> Result<TopStats, DbError> {
        let sql = format!("{} {}", traceparent_comment_for(parent), TOP_STATS_SQL);
        let row: TopStats = sqlx::query_as(&sql).fetch_one(&self.pool).await?;
        Ok(row)
    }
}

const TOP_STATS_SQL: &str = r#"
    SELECT
      numbackends::int4              AS num_backends,
      xact_commit                    AS xact_commit,
      xact_rollback                  AS xact_rollback,
      CASE WHEN (blks_hit + blks_read) = 0 THEN 0.0
           ELSE blks_hit::float8 / (blks_hit + blks_read)
      END                            AS cache_hit_ratio,
      pg_database_size(datname)      AS db_size_bytes
    FROM pg_stat_database
    WHERE datname = current_database()
"#;

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
