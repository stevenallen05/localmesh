//! Identity extraction at the gRPC server boundary.
//!
//! Two identity types, two interceptors, both PII-clean in telemetry.
//!
//! `User` comes from verified JWT claims via `auth_interceptor`. The
//! Authorization header is set by oauth2-proxy at the Caddy ingress
//! (`--pass-authorization-header`); www proxies it opaquely into gRPC
//! metadata. Server validates signature + issuer + audience + exp
//! against Dex's JWKS cache. Missing or invalid token yields
//! `Status::unauthenticated`. Handlers read `User` from request
//! extensions for business logic (WhoAmI response, db row author).
//! They must not echo PII onto OTel span attributes — that ingress
//! stamp lives at Caddy via the `enduser_attrs` plugin.
//!
//! `PeerIdentity` is extracted from the inbound TLS peer cert via
//! `tls::PeerLeafCert::spiffe_uri()`. Identifies the calling workload
//! (e.g., `spiffe://.../www`). Same telemetry restriction.
//!
//! Also exposes `ClaimsForDb`, a tiny (iss, sub) tuple stashed on
//! request extensions by `auth_interceptor` so `Greeter.SayHello` can
//! persist verified-JWT identity into the hello_messages table without
//! re-parsing the token.

use std::sync::Arc;

use tonic::{Request, Status};

use crate::auth::{AuthError, Jwks};

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

#[derive(Clone, Debug, Default)]
pub struct ClaimsForDb {
    pub iss: String,
    pub sub: String,
}

/// Tonic interceptor factory: returns a closure that validates the
/// `authorization` gRPC metadata as a JWT against the supplied JWKS
/// cache and inserts a `User` extension with the verified claims.
/// Missing or invalid token → Unauthenticated.
//
// `Status` is large (~176B) vs. the Ok request, but boxing it would
// break tonic's interceptor signature. Same trade-off as catalog.rs's
// `build_dashboard_payload`.
#[allow(clippy::result_large_err)]
pub fn auth_interceptor(
    jwks: Arc<Jwks>,
) -> impl Fn(Request<()>) -> Result<Request<()>, Status> + Clone {
    move |mut req: Request<()>| {
        let token = req
            .metadata()
            .get("authorization")
            .and_then(|v| v.to_str().ok())
            .ok_or_else(|| Status::unauthenticated("missing authorization metadata"))?;
        let token = token.strip_prefix("Bearer ").unwrap_or(token);
        let claims = jwks.validate(token).map_err(|e| match e {
            AuthError::Missing | AuthError::Malformed => Status::unauthenticated("malformed token"),
            AuthError::UnknownKid(_) => Status::unauthenticated("unknown signing key"),
            AuthError::Invalid(_) => Status::unauthenticated("invalid token"),
        })?;
        // Stash issuer + subject for downstream business-data persistence
        // (hello_messages row in greeter.rs). Built directly from the
        // already-validated claims — no second validate() call.
        req.extensions_mut().insert(ClaimsForDb {
            iss: claims.iss.clone(),
            sub: claims.sub.clone(),
        });
        req.extensions_mut().insert(User {
            id: claims.sub,
            email: claims.email.unwrap_or_default(),
            name: claims.preferred_username.unwrap_or_default(),
        });
        Ok(req)
    }
}

/// Tonic interceptor: extracts the SPIFFE URI from the inbound TLS peer
/// cert (provided by tonic's TLS layer via `TlsConnectInfo` in request
/// extensions) and inserts it into request extensions as a typed
/// `PeerIdentity`. Skipped if the connection is non-TLS, the peer
/// presented no cert, or the cert carries no SPIFFE URI SAN.
#[allow(clippy::result_large_err)]
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
            req.extensions_mut()
                .insert(PeerIdentity { spiffe_uri: uri });
        }
    }
    Ok(req)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::auth::Jwks;
    use jsonwebtoken::{encode, Algorithm, DecodingKey, EncodingKey, Header};
    use serde::Serialize;
    use tonic::metadata::{AsciiMetadataKey, MetadataValue};

    const TEST_PRIVATE_PEM: &str = include_str!("../testdata/auth_test_priv.pem");
    const TEST_PUBLIC_PEM: &str = include_str!("../testdata/auth_test_pub.pem");
    const TEST_ISS: &str = "https://dex.test.lvh.me:8443/dex";
    const TEST_AUD: &str = "localmesh-dev";

    fn make_token(sub: &str, email: &str) -> String {
        #[derive(Serialize)]
        struct C<'a> {
            sub: &'a str,
            email: &'a str,
            preferred_username: &'a str,
            iss: &'a str,
            aud: &'a str,
            exp: i64,
        }
        let mut header = Header::new(Algorithm::RS256);
        header.kid = Some("test-kid".into());
        let key = EncodingKey::from_rsa_pem(TEST_PRIVATE_PEM.as_bytes()).unwrap();
        let exp = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_secs() as i64
            + 60;
        encode(
            &header,
            &C {
                sub,
                email,
                preferred_username: sub,
                iss: TEST_ISS,
                aud: TEST_AUD,
                exp,
            },
            &key,
        )
        .unwrap()
    }

    fn jwks() -> Arc<Jwks> {
        let key = DecodingKey::from_rsa_pem(TEST_PUBLIC_PEM.as_bytes()).unwrap();
        Arc::new(Jwks::with_static_key("test-kid", key, TEST_ISS, TEST_AUD))
    }

    fn req_with_auth(value: &str) -> Request<()> {
        let mut req = Request::new(());
        let key = AsciiMetadataKey::from_static("authorization");
        let v: MetadataValue<_> = value.parse().unwrap();
        req.metadata_mut().insert(key, v);
        req
    }

    #[test]
    fn valid_bearer_token_inserts_user() {
        let token = make_token("alice", "alice@example.invalid");
        let req = auth_interceptor(jwks())(req_with_auth(&format!("Bearer {token}"))).unwrap();
        let u = req.extensions().get::<User>().unwrap();
        assert_eq!(u.id, "alice");
        assert_eq!(u.email, "alice@example.invalid");
    }

    #[test]
    fn valid_bare_token_inserts_user() {
        // oauth2-proxy's X-Auth-Request-Access-Token doesn't carry "Bearer "
        let token = make_token("bob", "bob@example.invalid");
        let req = auth_interceptor(jwks())(req_with_auth(&token)).unwrap();
        let u = req.extensions().get::<User>().unwrap();
        assert_eq!(u.id, "bob");
    }

    #[test]
    fn missing_authorization_rejects() {
        let result = auth_interceptor(jwks())(Request::new(()));
        let err = result.unwrap_err();
        assert_eq!(err.code(), tonic::Code::Unauthenticated);
    }

    #[test]
    fn invalid_token_rejects() {
        let result = auth_interceptor(jwks())(req_with_auth("Bearer not-a-jwt"));
        let err = result.unwrap_err();
        assert_eq!(err.code(), tonic::Code::Unauthenticated);
    }

    // Peer interceptor tests unchanged — preserve.
    #[test]
    fn peer_interceptor_no_op_when_no_tls_info() {
        let req = peer_interceptor(Request::new(())).unwrap();
        assert!(req.extensions().get::<PeerIdentity>().is_none());
    }
}
