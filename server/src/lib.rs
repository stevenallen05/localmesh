// Library entry — keeps logic testable. `main.rs` is the thin binary.

pub mod catalog;
pub mod grafana;
pub mod greeter;
pub mod telemetry;
pub mod proto {
    pub mod hello {
        include!("proto/hello.rs");
    }
    pub mod metrics_v1 {
        include!("proto/metrics.v1.rs");
    }
}
