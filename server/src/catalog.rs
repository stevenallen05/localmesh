use std::sync::Arc;

use opentelemetry::global::{self, BoxedSpan};
use opentelemetry::trace::{SpanKind, Tracer};
use opentelemetry::KeyValue;
use tonic::{Request, Response, Status};

use crate::grafana::{
    GfDashboard, GfDataSource, GfPanel, GrafanaClient, GrafanaError,
};
use crate::proto::metrics_v1::catalog_server::Catalog;
use crate::proto::metrics_v1::{
    dashboard::Panel as PanelOneof, CreateDashboardRequest, Dashboard, DataSource,
    DeleteDashboardRequest, GetDashboardRequest, GetDataSourceRequest, ListDashboardsRequest,
    ListDashboardsResponse, ListDataSourcesRequest, ListDataSourcesResponse, ListMetricsRequest,
    ListMetricsResponse, StatPanel, TablePanel, TimeseriesPanel, UpdateDashboardRequest,
};
use crate::telemetry::MetadataMap;

pub struct CatalogSvc {
    grafana: Arc<GrafanaClient>,
}

impl CatalogSvc {
    pub fn new(grafana: Arc<GrafanaClient>) -> Self {
        Self { grafana }
    }
}

// Returns the span guard rather than holding it locally — the caller must
// bind it (e.g., `let _span = open_span(...);`) so Drop fires at end of the
// handler, not at end of this helper.
fn open_span(req_metadata: &tonic::metadata::MetadataMap, method: &'static str) -> BoxedSpan {
    let parent_cx =
        global::get_text_map_propagator(|p| p.extract(&MetadataMap(req_metadata)));
    let tracer = global::tracer("server");
    tracer
        .span_builder(format!("Catalog/{method}"))
        .with_kind(SpanKind::Server)
        .with_attributes([
            KeyValue::new("rpc.system", "grpc"),
            KeyValue::new("rpc.method", method),
        ])
        .start_with_context(&tracer, &parent_cx)
}

impl From<GrafanaError> for Status {
    fn from(e: GrafanaError) -> Self {
        match e {
            GrafanaError::NotFound       => Status::not_found(e.to_string()),
            GrafanaError::Unauthorized   => Status::internal(e.to_string()),
            GrafanaError::Unavailable(_) => Status::unavailable(e.to_string()),
            GrafanaError::Decode(_)      => Status::internal(e.to_string()),
        }
    }
}

fn to_proto_data_source(gf: GfDataSource) -> DataSource {
    DataSource { uid: gf.uid, name: gf.name, r#type: gf.kind, url: gf.url }
}

// Recognize panel type → set the oneof variant. Returns None for unrecognized
// (multi-panel dashboards or types like "gauge" we don't model).
fn first_panel_to_oneof(panels: &[GfPanel]) -> Option<PanelOneof> {
    if panels.len() != 1 {
        return None;
    }
    let p = &panels[0];
    let ds_uid = p.datasource.as_ref()?.uid.clone();
    let query = p
        .targets
        .as_ref()?
        .first()?
        .expr
        .clone()
        .unwrap_or_default();
    match p.kind.as_str() {
        "timeseries" => Some(PanelOneof::Timeseries(TimeseriesPanel { datasource_uid: ds_uid, query })),
        "stat"       => Some(PanelOneof::Stat(StatPanel             { datasource_uid: ds_uid, query })),
        "table"      => Some(PanelOneof::Table(TablePanel           { datasource_uid: ds_uid, query })),
        _ => None,
    }
}

fn to_proto_dashboard(gf: GfDashboard) -> Dashboard {
    let panel = first_panel_to_oneof(&gf.panels);
    Dashboard {
        uid: gf.uid.clone(),
        name: gf.title,
        dashboard_path: format!("/d/{}", gf.uid),
        embed_path: format!("/d-solo/{}?panelId=1", gf.uid),
        panel,
    }
}

// Build the Grafana POST /api/dashboards/db payload from a proto Dashboard.
// Only the timeseries variant reaches here — others short-circuit upstream.
// `Status` is large (~176B) vs. the Ok JSON value; boxing it would force a
// matching dance at every call site. Not worth it for one validator.
#[allow(clippy::result_large_err)]
fn build_dashboard_payload(d: &Dashboard, overwrite: bool) -> Result<serde_json::Value, Status> {
    use PanelOneof::*;
    let (ds_uid, query) = match d.panel.as_ref() {
        Some(Timeseries(p)) => (&p.datasource_uid, &p.query),
        Some(Stat(_)) | Some(Table(_)) => {
            return Err(Status::unimplemented("stat/table panels are wire-only stubs"));
        }
        None => return Err(Status::invalid_argument("dashboard.panel oneof unset")),
    };
    if ds_uid.is_empty() {
        return Err(Status::invalid_argument("panel.datasource_uid empty"));
    }
    if d.name.is_empty() {
        return Err(Status::invalid_argument("dashboard.name empty"));
    }
    let mut dash = serde_json::json!({
        "title": d.name,
        "schemaVersion": 39,
        "panels": [{
            "id": 1,
            "type": "timeseries",
            "title": d.name,
            "datasource": { "uid": ds_uid },
            "targets": [{ "expr": query, "refId": "A" }],
            "gridPos": { "h": 8, "w": 24, "x": 0, "y": 0 }
        }]
    });
    if !d.uid.is_empty() {
        dash["uid"] = serde_json::Value::String(d.uid.clone());
    }
    Ok(serde_json::json!({ "dashboard": dash, "overwrite": overwrite }))
}

#[tonic::async_trait]
impl Catalog for CatalogSvc {
    async fn list_data_sources(
        &self,
        req: Request<ListDataSourcesRequest>,
    ) -> Result<Response<ListDataSourcesResponse>, Status> {
        let _span = open_span(req.metadata(), "list_data_sources");
        let items = self.grafana.list_data_sources().await?;
        Ok(Response::new(ListDataSourcesResponse {
            data_sources: items.into_iter().map(to_proto_data_source).collect(),
        }))
    }

    async fn get_data_source(
        &self,
        req: Request<GetDataSourceRequest>,
    ) -> Result<Response<DataSource>, Status> {
        let _span = open_span(req.metadata(), "get_data_source");
        let uid = req.into_inner().uid;
        if uid.is_empty() {
            return Err(Status::invalid_argument("uid empty"));
        }
        let gf = self.grafana.get_data_source(&uid).await?;
        Ok(Response::new(to_proto_data_source(gf)))
    }

    async fn list_dashboards(
        &self,
        req: Request<ListDashboardsRequest>,
    ) -> Result<Response<ListDashboardsResponse>, Status> {
        let _span = open_span(req.metadata(), "list_dashboards");
        let entries = self.grafana.list_dashboard_entries().await?;
        // N+1: per-uid get to pull full panel info. Acceptable at <100
        // dashboards (demo scale); spec documents the cap.
        let mut out = Vec::with_capacity(entries.len());
        for e in entries {
            let full = self.grafana.get_dashboard(&e.uid).await?;
            out.push(to_proto_dashboard(full));
        }
        Ok(Response::new(ListDashboardsResponse { dashboards: out }))
    }

    async fn get_dashboard(
        &self,
        req: Request<GetDashboardRequest>,
    ) -> Result<Response<Dashboard>, Status> {
        let _span = open_span(req.metadata(), "get_dashboard");
        let uid = req.into_inner().uid;
        if uid.is_empty() {
            return Err(Status::invalid_argument("uid empty"));
        }
        let gf = self.grafana.get_dashboard(&uid).await?;
        Ok(Response::new(to_proto_dashboard(gf)))
    }

    async fn create_dashboard(
        &self,
        req: Request<CreateDashboardRequest>,
    ) -> Result<Response<Dashboard>, Status> {
        let _span = open_span(req.metadata(), "create_dashboard");
        let mut d = req
            .into_inner()
            .dashboard
            .ok_or_else(|| Status::invalid_argument("dashboard missing"))?;
        d.uid.clear();   // Create — Grafana mints the uid.
        let payload = build_dashboard_payload(&d, false)?;
        let gf = self.grafana.upsert_dashboard(&payload).await?;
        Ok(Response::new(to_proto_dashboard(gf)))
    }

    async fn update_dashboard(
        &self,
        req: Request<UpdateDashboardRequest>,
    ) -> Result<Response<Dashboard>, Status> {
        let _span = open_span(req.metadata(), "update_dashboard");
        let d = req
            .into_inner()
            .dashboard
            .ok_or_else(|| Status::invalid_argument("dashboard missing"))?;
        if d.uid.is_empty() {
            return Err(Status::invalid_argument("dashboard.uid required for update"));
        }
        let payload = build_dashboard_payload(&d, true)?;
        let gf = self.grafana.upsert_dashboard(&payload).await?;
        Ok(Response::new(to_proto_dashboard(gf)))
    }

    async fn delete_dashboard(
        &self,
        req: Request<DeleteDashboardRequest>,
    ) -> Result<Response<()>, Status> {
        let _span = open_span(req.metadata(), "delete_dashboard");
        let uid = req.into_inner().uid;
        if uid.is_empty() {
            return Err(Status::invalid_argument("uid empty"));
        }
        self.grafana.delete_dashboard(&uid).await?;
        Ok(Response::new(()))
    }

    async fn list_metrics(
        &self,
        req: Request<ListMetricsRequest>,
    ) -> Result<Response<ListMetricsResponse>, Status> {
        let _span = open_span(req.metadata(), "list_metrics");
        let ds_uid = req.into_inner().datasource_uid;
        if ds_uid.is_empty() {
            return Err(Status::invalid_argument("datasource_uid empty"));
        }
        // Filter: only Prometheus-compat datasources return non-empty.
        let ds = self.grafana.get_data_source(&ds_uid).await?;
        if ds.kind != "prometheus" {
            return Ok(Response::new(ListMetricsResponse { names: vec![] }));
        }
        let names = self.grafana.list_prometheus_metrics(&ds_uid).await?;
        Ok(Response::new(ListMetricsResponse { names }))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn create_with_panel_unset_is_invalid_argument() {
        let d = Dashboard {
            uid: String::new(),
            name: "x".into(),
            dashboard_path: String::new(),
            embed_path: String::new(),
            panel: None,
        };
        let err = build_dashboard_payload(&d, false).unwrap_err();
        assert_eq!(err.code(), tonic::Code::InvalidArgument);
    }
}
