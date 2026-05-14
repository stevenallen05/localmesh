// `dead_code` allowed: response-shape fields are populated by serde and the
// GrafanaClient methods are read by chunk 3's catalog.rs (incoming).
#![allow(dead_code)]

use std::time::Duration;

use serde::Deserialize;

#[derive(Debug)]
pub(crate) enum GrafanaError {
    NotFound,
    Unauthorized,
    Unavailable(String),
    Decode(String),
}

impl std::fmt::Display for GrafanaError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::NotFound => write!(f, "grafana: not found"),
            Self::Unauthorized => write!(f, "grafana: unauthorized (server-config bug — token rejected)"),
            Self::Unavailable(m) => write!(f, "grafana: unavailable: {m}"),
            Self::Decode(m) => write!(f, "grafana: decode: {m}"),
        }
    }
}

impl std::error::Error for GrafanaError {}

#[derive(Debug, Deserialize)]
pub(crate) struct GfDataSource {
    pub(crate) uid:  String,
    pub(crate) name: String,
    #[serde(rename = "type")]
    pub(crate) kind: String,   // serde rename because `type` is a Rust keyword.
    pub(crate) url:  String,
}

#[derive(Debug, Deserialize)]
pub(crate) struct GfSearchEntry {
    pub(crate) uid:   String,
    pub(crate) title: String,
}

// Subset of the dashboard-get response we actually read. Grafana returns
// a {dashboard, meta} envelope; we only need .dashboard.
#[derive(Debug, Deserialize)]
pub(crate) struct GfDashboardEnvelope {
    pub(crate) dashboard: GfDashboard,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct GfDashboard {
    pub(crate) uid:    String,
    pub(crate) title:  String,
    pub(crate) panels: Vec<GfPanel>,
}

#[derive(Debug, Deserialize)]
pub(crate) struct GfPanel {
    #[serde(rename = "type")]
    pub(crate) kind:       String,    // "timeseries", "stat", "table", ...
    pub(crate) datasource: Option<GfDatasourceRef>,
    pub(crate) targets:    Option<Vec<GfTarget>>,
}

#[derive(Debug, Deserialize)]
pub(crate) struct GfDatasourceRef {
    pub(crate) uid: String,
}

#[derive(Debug, Deserialize)]
pub(crate) struct GfTarget {
    pub(crate) expr: Option<String>,   // PromQL expression
}

#[derive(Debug, Deserialize)]
pub(crate) struct GfMetricsResponse {
    pub(crate) status: String,
    pub(crate) data:   Vec<String>,
}

// Constants for the bootstrap-token wait loop.
pub(crate) const TOKEN_WAIT_TIMEOUT_SECS: u64 = 30;
pub(crate) const TOKEN_WAIT_INTERVAL_MS:  u64 = 500;

pub struct GrafanaClient {
    pub(crate) http:  reqwest::Client,
    pub(crate) base:  String,
    pub(crate) token: String,
}

pub(crate) fn parse_data_sources(body: &str) -> Result<Vec<GfDataSource>, GrafanaError> {
    serde_json::from_str(body).map_err(|e| GrafanaError::Decode(e.to_string()))
}

pub(crate) fn parse_dashboard(body: &str) -> Result<GfDashboard, GrafanaError> {
    let env: GfDashboardEnvelope =
        serde_json::from_str(body).map_err(|e| GrafanaError::Decode(e.to_string()))?;
    Ok(env.dashboard)
}

pub(crate) fn parse_prom_metrics(body: &str) -> Result<Vec<String>, GrafanaError> {
    let resp: GfMetricsResponse =
        serde_json::from_str(body).map_err(|e| GrafanaError::Decode(e.to_string()))?;
    if resp.status != "success" {
        return Err(GrafanaError::Decode(format!("non-success status: {}", resp.status)));
    }
    Ok(resp.data)
}

use std::path::Path;
use tokio::time::sleep;

pub(crate) type ClientResult<T> = Result<T, GrafanaError>;

impl GrafanaClient {
    pub async fn from_env() -> Result<Self, Box<dyn std::error::Error + Send + Sync>> {
        let base = std::env::var("GRAFANA_API_URL")
            .unwrap_or_else(|_| "http://grafana:3000".to_string());
        let path = std::env::var("GRAFANA_API_TOKEN_FILE")
            .unwrap_or_else(|_| "/run/grafana/token".to_string());
        let token = wait_for_token_file(&path).await?;
        Ok(Self {
            http: reqwest::Client::new(),
            base,
            token,
        })
    }
}

async fn wait_for_token_file(path: &str) -> Result<String, Box<dyn std::error::Error + Send + Sync>> {
    let deadline = std::time::Instant::now()
        + Duration::from_secs(TOKEN_WAIT_TIMEOUT_SECS);
    loop {
        // Blocking I/O is fine here: this runs once during startup before the
        // server is serving requests. spawn_blocking is for hot paths.
        if Path::new(path).exists() {
            let s = std::fs::read_to_string(path)?;
            let trimmed = s.trim().to_string();
            if !trimmed.is_empty() {
                return Ok(trimmed);
            }
        }
        if std::time::Instant::now() >= deadline {
            return Err(format!(
                "grafana token file {path} not present after {}s",
                TOKEN_WAIT_TIMEOUT_SECS
            )
            .into());
        }
        println!("server: waiting for grafana token file at {path}");
        sleep(Duration::from_millis(TOKEN_WAIT_INTERVAL_MS)).await;
    }
}

impl GrafanaClient {
    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base, path)
    }

    async fn get_text(&self, path: &str) -> ClientResult<String> {
        let resp = self.http
            .get(self.url(path))
            .bearer_auth(&self.token)
            .send()
            .await
            .map_err(|e| GrafanaError::Unavailable(e.to_string()))?;
        map_status(&resp)?;
        resp.text().await.map_err(|e| GrafanaError::Decode(e.to_string()))
    }

    async fn delete(&self, path: &str) -> ClientResult<()> {
        let resp = self.http
            .delete(self.url(path))
            .bearer_auth(&self.token)
            .send()
            .await
            .map_err(|e| GrafanaError::Unavailable(e.to_string()))?;
        map_status(&resp)?;
        Ok(())
    }

    async fn post_json(&self, path: &str, body: &serde_json::Value) -> ClientResult<String> {
        let resp = self.http
            .post(self.url(path))
            .bearer_auth(&self.token)
            .json(body)
            .send()
            .await
            .map_err(|e| GrafanaError::Unavailable(e.to_string()))?;
        map_status(&resp)?;
        resp.text().await.map_err(|e| GrafanaError::Decode(e.to_string()))
    }

    pub(crate) async fn list_data_sources(&self) -> ClientResult<Vec<GfDataSource>> {
        let body = self.get_text("/api/datasources").await?;
        parse_data_sources(&body)
    }

    pub(crate) async fn get_data_source(&self, uid: &str) -> ClientResult<GfDataSource> {
        let body = self.get_text(&format!("/api/datasources/uid/{uid}")).await?;
        serde_json::from_str(&body).map_err(|e| GrafanaError::Decode(e.to_string()))
    }

    pub(crate) async fn list_dashboard_entries(&self) -> ClientResult<Vec<GfSearchEntry>> {
        let body = self.get_text("/api/search?type=dash-db").await?;
        serde_json::from_str(&body).map_err(|e| GrafanaError::Decode(e.to_string()))
    }

    pub(crate) async fn get_dashboard(&self, uid: &str) -> ClientResult<GfDashboard> {
        let body = self.get_text(&format!("/api/dashboards/uid/{uid}")).await?;
        parse_dashboard(&body)
    }

    pub(crate) async fn upsert_dashboard(&self, payload: &serde_json::Value) -> ClientResult<GfDashboard> {
        // Grafana echoes {uid, slug, version, status, ...}. Re-fetch by uid for the
        // full panel body so callers see the same shape as List/Get.
        let resp_body = self.post_json("/api/dashboards/db", payload).await?;
        #[derive(Deserialize)]
        struct PostResp { uid: String }
        let posted: PostResp = serde_json::from_str(&resp_body)
            .map_err(|e| GrafanaError::Decode(e.to_string()))?;
        self.get_dashboard(&posted.uid).await
    }

    pub(crate) async fn delete_dashboard(&self, uid: &str) -> ClientResult<()> {
        self.delete(&format!("/api/dashboards/uid/{uid}")).await
    }

    pub(crate) async fn list_prometheus_metrics(&self, datasource_uid: &str) -> ClientResult<Vec<String>> {
        let body = self
            .get_text(&format!(
                "/api/datasources/uid/{datasource_uid}/resources/api/v1/label/__name__/values"
            ))
            .await?;
        parse_prom_metrics(&body)
    }
}

fn map_status(resp: &reqwest::Response) -> ClientResult<()> {
    use reqwest::StatusCode as S;
    match resp.status() {
        s if s.is_success() => Ok(()),
        S::NOT_FOUND => Err(GrafanaError::NotFound),
        S::UNAUTHORIZED | S::FORBIDDEN => Err(GrafanaError::Unauthorized),
        s => Err(GrafanaError::Unavailable(format!("grafana {s}"))),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_data_sources_list() {
        let body = r#"[
            {"uid":"vm","name":"VictoriaMetrics","type":"prometheus","url":"http://vm:8428"},
            {"uid":"tempo","name":"Tempo","type":"tempo","url":"http://tempo:3200"}
        ]"#;
        let parsed = parse_data_sources(body).expect("parse");
        assert_eq!(parsed.len(), 2);
        assert_eq!(parsed[0].uid, "vm");
        assert_eq!(parsed[0].kind, "prometheus");
        assert_eq!(parsed[1].kind, "tempo");
    }

    #[test]
    fn parses_dashboard_with_timeseries_panel() {
        let body = r#"{
            "dashboard": {
                "uid": "abc",
                "title": "rate(up)",
                "panels": [{
                    "type": "timeseries",
                    "datasource": {"uid": "vm"},
                    "targets": [{"expr": "up"}]
                }]
            },
            "meta": {}
        }"#;
        let parsed = parse_dashboard(body).expect("parse");
        assert_eq!(parsed.uid, "abc");
        assert_eq!(parsed.title, "rate(up)");
        assert_eq!(parsed.panels.len(), 1);
        let p = &parsed.panels[0];
        assert_eq!(p.kind, "timeseries");
        assert_eq!(p.datasource.as_ref().unwrap().uid, "vm");
        assert_eq!(p.targets.as_ref().unwrap()[0].expr.as_deref(), Some("up"));
    }

    #[test]
    fn parses_dashboard_with_unknown_panel_kind() {
        let body = r#"{
            "dashboard": {
                "uid": "xyz",
                "title": "weird",
                "panels": [{"type": "gauge", "datasource": {"uid": "vm"}, "targets": []}]
            },
            "meta": {}
        }"#;
        let parsed = parse_dashboard(body).expect("parse");
        assert_eq!(parsed.panels[0].kind, "gauge");
    }

    #[test]
    fn parses_prometheus_metrics_response() {
        let body = r#"{"status":"success","data":["up","go_gc_duration_seconds","node_cpu_seconds_total"]}"#;
        let parsed = parse_prom_metrics(body).expect("parse");
        assert_eq!(parsed.len(), 3);
        assert_eq!(parsed[0], "up");
    }
}
