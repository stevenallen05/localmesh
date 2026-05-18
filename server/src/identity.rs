//! Identity extraction at the gRPC server boundary.
//!
//! Two identity types, two interceptors, both PII-clean in telemetry:
//!
//!   * `User` — pulled from `x-user-{id,email,name}` gRPC metadata. The
//!     www edge sets these via `userMetadata(user)` in `www/lib/identity.ts`.
//!     Handlers read it from request extensions for business-logic use
//!     (e.g., populating a WhoAmI response payload) and may DEBUG-log it,
//!     but must not stamp it onto OTel span attributes — that ingress
//!     stamp is www's job, per the PII-at-ingress rule (spec §9.3).
//!
//!   * `PeerIdentity` — extracted from the inbound TLS peer cert via
//!     `tls::PeerLeafCert::spiffe_uri()`. Identifies the calling workload
//!     (e.g., `spiffe://.../www`). Same telemetry restriction: read for
//!     business logic and DEBUG logs only.
//!
//! Enforcement of the PII rule is documented + code-reviewed today;
//! mechanical enforcement (Vector / Tempo scrubbing) is deferred.
//! `TODO: needs_prod_decisions PII enforcement (regex / lint).`

use tonic::{Request, Status};

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct User {
    pub id: String,
    pub email: String,
    pub name: String,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PeerIdentity {
    pub spiffe_uri: String,
}

/// Tonic interceptor: extracts the `x-user-*` metadata trio into a `User`
/// and inserts it into the request extensions. All three keys required;
/// partial identity is dropped (no extension, no error).
pub fn user_interceptor(mut req: Request<()>) -> Result<Request<()>, Status> {
    let md = req.metadata();
    let pull = |k: &str| md.get(k).and_then(|v| v.to_str().ok()).map(String::from);
    if let (Some(id), Some(email), Some(name)) = (
        pull("x-user-id"),
        pull("x-user-email"),
        pull("x-user-name"),
    ) {
        req.extensions_mut().insert(User { id, email, name });
    }
    Ok(req)
}

/// Tonic interceptor: extracts the SPIFFE URI from the inbound TLS peer
/// cert (provided by tonic's TLS layer via `TlsConnectInfo` in request
/// extensions) and inserts it into request extensions as a typed
/// `PeerIdentity`. Skipped if the connection is non-TLS, the peer
/// presented no cert, or the cert carries no SPIFFE URI SAN.
pub fn peer_interceptor(mut req: Request<()>) -> Result<Request<()>, Status> {
    use tonic::transport::server::{TcpConnectInfo, TlsConnectInfo};

    let leaf_der: Option<Vec<u8>> = req
        .extensions()
        .get::<TlsConnectInfo<TcpConnectInfo>>()
        .and_then(|tls| tls.peer_certs())
        .and_then(|certs| certs.first().map(|c| c.as_ref().to_vec()));

    if let Some(der) = leaf_der {
        let leaf = crate::tls::PeerLeafCert { der };
        if let Some(uri) = leaf.spiffe_uri() {
            req.extensions_mut().insert(PeerIdentity { spiffe_uri: uri });
        }
    }
    Ok(req)
}

#[cfg(test)]
mod tests {
    use super::*;
    use tonic::metadata::{AsciiMetadataKey, MetadataValue};

    fn req_with_meta(pairs: &[(&'static str, &str)]) -> Request<()> {
        let mut req = Request::new(());
        for (k, v) in pairs {
            let key = AsciiMetadataKey::from_static(*k);
            let value: MetadataValue<_> = v.parse().unwrap();
            req.metadata_mut().insert(key, value);
        }
        req
    }

    #[test]
    fn user_interceptor_inserts_when_all_three_present() {
        let req = user_interceptor(req_with_meta(&[
            ("x-user-id", "alice"),
            ("x-user-email", "a@x.invalid"),
            ("x-user-name", "Alice"),
        ]))
        .unwrap();
        let u = req.extensions().get::<User>().unwrap();
        assert_eq!(
            u,
            &User {
                id: "alice".into(),
                email: "a@x.invalid".into(),
                name: "Alice".into()
            }
        );
    }

    #[test]
    fn user_interceptor_skips_when_partial() {
        let req = user_interceptor(req_with_meta(&[("x-user-id", "alice")])).unwrap();
        assert!(req.extensions().get::<User>().is_none());
    }

    #[test]
    fn user_interceptor_skips_when_empty() {
        let req = user_interceptor(req_with_meta(&[])).unwrap();
        assert!(req.extensions().get::<User>().is_none());
    }

    // Note: peer_interceptor now reads from tonic's TlsConnectInfo
    // extension instead of a directly-injected PeerLeafCert. Building a
    // TlsConnectInfo in a unit test requires private tonic internals; the
    // SPIFFE-URI extraction itself is covered in `tls::tests` via the DER
    // fixture. Only the no-op (non-TLS) path is unit-testable here.

    #[test]
    fn peer_interceptor_no_op_when_no_tls_info() {
        let req = peer_interceptor(Request::new(())).unwrap();
        assert!(req.extensions().get::<PeerIdentity>().is_none());
    }
}
