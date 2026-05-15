//! Postgres client wrapper. Owns the pool, runs migrations, exposes the two
//! queries the demo `PrintPostgresStats` RPC needs. No `tonic` dependency —
//! the gRPC error mapping lives in `greeter.rs` where the orphan rule allows it.

use opentelemetry::trace::SpanContext;

/// Format a `SpanContext` as a pg_tracing-compatible SQL comment carrying a
/// W3C traceparent. The sampled-flag byte is **derived** from the context's
/// trace flags so an unsampled parent produces `00`, never `01`. Returns
/// an empty string when the context is invalid (no parent in scope) so
/// callers can prepend it unconditionally.
// Consumers (Db::record_noise_event, Db::top_stats) land in the next commit.
#[cfg_attr(not(test), allow(dead_code))]
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
