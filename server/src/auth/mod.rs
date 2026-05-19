//! JWT validation against Dex's JWKS. Cache primed at startup;
//! background task refreshes every 15 min; per-request lookups are
//! sync against the in-memory cache.

use std::collections::HashMap;
use std::sync::Arc;

use jsonwebtoken::{Algorithm, DecodingKey, Validation};
use serde::Deserialize;
use tokio::sync::RwLock;

#[derive(Debug, thiserror::Error)]
pub enum AuthError {
    #[error("missing authorization header")]
    Missing,
    #[error("malformed authorization header")]
    Malformed,
    #[error("token signing key not in JWKS cache: kid={0}")]
    UnknownKid(String),
    #[error("jwt validation failed: {0}")]
    Invalid(#[from] jsonwebtoken::errors::Error),
}

#[derive(Debug, Clone, Deserialize)]
pub struct Claims {
    pub sub:                 String,
    pub email:               Option<String>,
    pub preferred_username:  Option<String>,
    pub iss:                 String,
    pub exp:                 i64,
    // `aud` deliberately omitted from the typed Claims struct — OIDC
    // allows it as either a string or an array, and jsonwebtoken's
    // `Validation::set_audience` handles both internally without needing
    // a typed field here. Adding `aud: Option<String>` would silently
    // fail Serde-deserialization when Dex emits `["localmesh-dev"]`.
}

#[derive(Clone)]
pub struct Jwks {
    keys: Arc<RwLock<HashMap<String, DecodingKey>>>,
    issuer: String,
    audience: String,
}

impl Jwks {
    /// Construct an empty cache. Use `fetch_from` to prime against Dex,
    /// or `with_static_key` in tests.
    pub fn new(issuer: impl Into<String>, audience: impl Into<String>) -> Self {
        Self {
            keys: Arc::new(RwLock::new(HashMap::new())),
            issuer: issuer.into(),
            audience: audience.into(),
        }
    }

    /// Test-only constructor: inject a single (kid, key) pair.
    #[cfg(test)]
    pub fn with_static_key(
        kid: &str,
        key: DecodingKey,
        issuer: impl Into<String>,
        audience: impl Into<String>,
    ) -> Self {
        let mut map = HashMap::new();
        map.insert(kid.to_string(), key);
        Self {
            keys: Arc::new(RwLock::new(map)),
            issuer: issuer.into(),
            audience: audience.into(),
        }
    }

    pub fn validate(&self, token: &str) -> Result<Claims, AuthError> {
        let header = jsonwebtoken::decode_header(token)?;
        let kid = header.kid.ok_or_else(|| AuthError::UnknownKid("no kid".into()))?;

        // Sync cache read on the hot path; no I/O.
        let keys = self.keys.try_read().map_err(|_| AuthError::UnknownKid(kid.clone()))?;
        let key = keys.get(&kid).ok_or_else(|| AuthError::UnknownKid(kid.clone()))?;

        let mut validation = Validation::new(Algorithm::RS256);
        validation.set_issuer(&[&self.issuer]);
        validation.set_audience(&[&self.audience]);
        validation.validate_exp = true;

        let data = jsonwebtoken::decode::<Claims>(token, key, &validation)?;
        Ok(data.claims)
    }
}

#[derive(Debug, Deserialize)]
struct Jwk {
    kid: String,
    kty: String,                  // "RSA"
    n:   String,                  // base64url RSA modulus
    e:   String,                  // base64url RSA exponent
    #[allow(dead_code)]
    alg: Option<String>,
}

#[derive(Debug, Deserialize)]
struct JwksResponse {
    keys: Vec<Jwk>,
}

impl Jwks {
    /// One-shot fetch + parse from Dex's JWKS endpoint. Replaces the
    /// in-memory cache atomically.
    pub async fn fetch_from(&self, url: &str) -> Result<(), AuthError> {
        // map_err is intentionally lossy — JWKS-fetch failure is a single-bit
        // "did we get keys?" signal. Refine to a dedicated AuthError::FetchFailed
        // variant if richer reporting is needed.
        let body: JwksResponse = reqwest::get(url).await
            .map_err(|_| AuthError::Invalid(jsonwebtoken::errors::Error::from(
                jsonwebtoken::errors::ErrorKind::InvalidToken
            )))?
            .json().await
            .map_err(|_| AuthError::Invalid(jsonwebtoken::errors::Error::from(
                jsonwebtoken::errors::ErrorKind::InvalidToken
            )))?;
        let mut map = HashMap::new();
        for jwk in body.keys {
            if jwk.kty != "RSA" { continue; }
            let key = DecodingKey::from_rsa_components(&jwk.n, &jwk.e)?;
            map.insert(jwk.kid, key);
        }
        let mut keys = self.keys.write().await;
        *keys = map;
        Ok(())
    }

    /// Spawn a background task that refreshes the cache every `interval`.
    /// Takes `Arc<Self>` (not `Self`) so the caller's Arc clone can be
    /// passed directly — `self`-by-value would force moving the Jwks
    /// out of the caller's Arc, which is impossible.
    /// Returns a JoinHandle the caller can drop on shutdown.
    pub fn spawn_refresher(self: Arc<Self>, url: String, interval: std::time::Duration) -> tokio::task::JoinHandle<()> {
        tokio::spawn(async move {
            let mut tick = tokio::time::interval(interval);
            tick.tick().await;   // consume the immediate-fire tick
            loop {
                tick.tick().await;
                if let Err(e) = self.fetch_from(&url).await {
                    tracing::warn!(error = ?e, "JWKS refresh failed");
                }
            }
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use jsonwebtoken::{encode, EncodingKey, Header};
    use serde::Serialize;

    // 2048-bit RSA keypair for tests. Generated once with:
    //   openssl genrsa -out test.key 2048 && openssl rsa -in test.key -pubout
    // Checked into server/testdata/ — never used in production.
    const TEST_PRIVATE_PEM: &str = include_str!("../../testdata/auth_test_priv.pem");
    const TEST_PUBLIC_PEM:  &str = include_str!("../../testdata/auth_test_pub.pem");

    const TEST_ISS: &str = "https://dex.test.lvh.me:8443/dex";
    const TEST_AUD: &str = "localmesh-dev";

    #[derive(Serialize)]
    struct TestClaims<'a> {
        sub: &'a str,
        email: &'a str,
        preferred_username: &'a str,
        iss: &'a str,
        aud: &'a str,
        exp: i64,
    }

    fn make_token(claims: TestClaims) -> String {
        let mut header = Header::new(Algorithm::RS256);
        header.kid = Some("test-kid".into());
        let key = EncodingKey::from_rsa_pem(TEST_PRIVATE_PEM.as_bytes()).unwrap();
        encode(&header, &claims, &key).unwrap()
    }

    fn jwks() -> Jwks {
        let key = DecodingKey::from_rsa_pem(TEST_PUBLIC_PEM.as_bytes()).unwrap();
        Jwks::with_static_key("test-kid", key, TEST_ISS, TEST_AUD)
    }

    fn now_plus(secs: i64) -> i64 {
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH).unwrap()
            .as_secs() as i64 + secs
    }

    #[test]
    fn valid_token_yields_claims() {
        let token = make_token(TestClaims {
            sub: "alice",
            email: "alice@example.invalid",
            preferred_username: "Alice Example",
            iss: TEST_ISS,
            aud: TEST_AUD,
            exp: now_plus(60),
        });
        let claims = jwks().validate(&token).unwrap();
        assert_eq!(claims.sub, "alice");
        assert_eq!(claims.email.as_deref(), Some("alice@example.invalid"));
    }

    #[test]
    fn expired_token_rejected() {
        // jsonwebtoken's default Validation::leeway is 60s; push exp well
        // past that so the token is unambiguously expired.
        let token = make_token(TestClaims {
            sub: "alice",
            email: "alice@example.invalid",
            preferred_username: "Alice",
            iss: TEST_ISS,
            aud: TEST_AUD,
            exp: now_plus(-3600),
        });
        let err = jwks().validate(&token).unwrap_err();
        assert!(matches!(err, AuthError::Invalid(_)), "got {err:?}");
    }

    #[test]
    fn wrong_issuer_rejected() {
        let token = make_token(TestClaims {
            sub: "alice",
            email: "alice@x",
            preferred_username: "Alice",
            iss: "https://attacker.invalid/idp",
            aud: TEST_AUD,
            exp: now_plus(60),
        });
        let err = jwks().validate(&token).unwrap_err();
        assert!(matches!(err, AuthError::Invalid(_)), "got {err:?}");
    }

    #[test]
    fn wrong_audience_rejected() {
        let token = make_token(TestClaims {
            sub: "alice",
            email: "alice@x",
            preferred_username: "Alice",
            iss: TEST_ISS,
            aud: "other-app",
            exp: now_plus(60),
        });
        let err = jwks().validate(&token).unwrap_err();
        assert!(matches!(err, AuthError::Invalid(_)), "got {err:?}");
    }

    #[test]
    fn unknown_kid_rejected() {
        // Use a key not in the cache.
        let other_key = DecodingKey::from_rsa_pem(TEST_PUBLIC_PEM.as_bytes()).unwrap();
        let cache = Jwks::with_static_key("different-kid", other_key, TEST_ISS, TEST_AUD);
        let token = make_token(TestClaims {
            sub: "alice", email: "a@x", preferred_username: "Alice",
            iss: TEST_ISS, aud: TEST_AUD, exp: now_plus(60),
        });
        let err = cache.validate(&token).unwrap_err();
        assert!(matches!(err, AuthError::UnknownKid(_)), "got {err:?}");
    }
}
