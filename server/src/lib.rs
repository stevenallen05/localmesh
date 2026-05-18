// Library entry — keeps logic testable. `main.rs` is the thin binary.

pub mod catalog;
pub mod db;
pub mod error_log;
pub mod grafana;
pub mod greeter;
pub mod rpc_metrics;
pub mod telemetry;
pub mod trace_context;
pub mod proto {
    pub mod hello {
        include!("proto/hello.rs");
    }
    pub mod metrics_v1 {
        include!("proto/metrics.v1.rs");
    }
}
