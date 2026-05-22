//! Identity extraction at the gRPC server boundary.
//!
//! `User` comes from forwarded gRPC metadata. `www` sets `x-user-id`,
//! `x-user-email`, `x-user-name` when calling `server`; the trust anchor
//! is the ghostunnel sidecar's `--allow-uri` allowlist, which gates
//! which workloads can reach this server. Server does not validate the
//! JWT — that happens at the Caddy + oauth2-proxy ingress. PII-at-ingress
//! rule holds: handlers read `User` for business logic but must not echo
//! it onto OTel span attributes or indexed Loki labels.
//!
//! `ClaimsForDb` populates the hello_messages row author with
//! (issuer="forwarded", subject=user_id) — the forwarded shape doesn't
//! carry the upstream JWT's iss/sub separately.
//!
//! TODO: needs_prod_decisions phantom-token broker for cryptographic
//! claim narrowing per workload SPIFFE URI.

use tonic::{Request, Status};

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct User {
    pub id: String,
    pub email: String,
    pub name: String,
}

#[derive(Clone, Debug, Default)]
pub struct ClaimsForDb {
    pub iss: String,
    pub sub: String,
}

/// Tonic interceptor: reads forwarded user identity from gRPC metadata
/// and inserts `User` + `ClaimsForDb` into request extensions. Trust
/// anchor is upstream: the sidecar's `--allow-uri` allowlist policed
/// who could reach this server in the first place.
///
/// Missing `x-user-id` is a contract violation — the upstream (www) is
/// expected to always forward identity through to here. Reject rather
/// than serve anonymous traffic.
// `Status` is large; boxing it would break tonic's interceptor signature.
#[allow(clippy::result_large_err)]
pub fn auth_interceptor(mut req: Request<()>) -> Result<Request<()>, Status> {
    let id = req
        .metadata()
        .get("x-user-id")
        .and_then(|v| v.to_str().ok())
        .ok_or_else(|| Status::unauthenticated("missing x-user-id metadata"))?
        .to_string();
    let email = req
        .metadata()
        .get("x-user-email")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("")
        .to_string();
    let name = req
        .metadata()
        .get("x-user-name")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("")
        .to_string();
    req.extensions_mut().insert(ClaimsForDb {
        iss: "forwarded".into(),
        sub: id.clone(),
    });
    req.extensions_mut().insert(User { id, email, name });
    Ok(req)
}

#[cfg(test)]
mod tests {
    use super::*;
    use tonic::metadata::{AsciiMetadataKey, MetadataValue};
    use tonic::Request;

    fn req_with(headers: &[(&str, &str)]) -> Request<()> {
        let mut req = Request::new(());
        for (k, v) in headers {
            let key = AsciiMetadataKey::from_bytes(k.as_bytes()).unwrap();
            let val: MetadataValue<_> = v.parse().unwrap();
            req.metadata_mut().insert(key, val);
        }
        req
    }

    #[test]
    fn metadata_inserts_user() {
        let req = auth_interceptor(req_with(&[
            ("x-user-id", "alice"),
            ("x-user-email", "alice@example.invalid"),
            ("x-user-name", "Alice"),
        ]))
        .unwrap();
        let u = req.extensions().get::<User>().unwrap();
        assert_eq!(u.id, "alice");
        assert_eq!(u.email, "alice@example.invalid");
        assert_eq!(u.name, "Alice");
    }

    #[test]
    fn missing_user_id_rejects() {
        let result = auth_interceptor(req_with(&[("x-user-email", "bob@example.invalid")]));
        assert_eq!(result.unwrap_err().code(), tonic::Code::Unauthenticated);
    }

    #[test]
    fn email_and_name_default_when_absent() {
        let req = auth_interceptor(req_with(&[("x-user-id", "carol")])).unwrap();
        let u = req.extensions().get::<User>().unwrap();
        assert_eq!(u.id, "carol");
        assert_eq!(u.email, "");
        assert_eq!(u.name, "");
    }
}
