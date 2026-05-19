// Library entry — keeps logic testable. `main.rs` is the thin binary.

pub mod auth;
pub mod db;
pub mod error_log;
pub mod greeter;
pub mod identity;
pub mod rpc_metrics;
pub mod telemetry;
pub mod tls;
pub mod trace_context;
pub mod proto {
    pub mod hello {
        include!("proto/hello.rs");
    }
}
