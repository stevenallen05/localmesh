//! Inbound mTLS for the gRPC server.
//!
//! Loads `id.crt` / `id.key` / `trust.ca.crt` from `/run/server/` (the
//! per-container cert dir produced by `make certs` from
//! `project.toml`'s `[[services]]` entry for `server`).
//!
//! Verification level: verify-CA — clients must present a cert that
//! chains to the LocalMesh CA. Identity ENRICHMENT (extracting the
//! SPIFFE URI from the peer cert for logging / WhoAmI responses)
//! happens in `crate::identity::peer_interceptor`; transport-time
//! REJECTION based on identity is out of scope here.
//!
//! TODO: needs_prod_decisions tighten client verification to a SPIFFE
//! URI allowlist — prod uses mesh AuthorizationPolicy at the sidecar.

use std::fs;
use std::path::Path;

use tonic::transport::{Certificate, Identity, ServerTlsConfig};

const CERTS_DIR: &str = "/run/server";

#[derive(Clone, Debug)]
pub struct PeerLeafCert {
    pub der: Vec<u8>,
}

impl PeerLeafCert {
    /// Extract the first SPIFFE URI SAN from the leaf cert. Returns
    /// `None` if the cert isn't parseable or carries no SPIFFE URI.
    pub fn spiffe_uri(&self) -> Option<String> {
        let (_, parsed) = x509_parser::parse_x509_certificate(&self.der).ok()?;
        for ext in parsed.extensions() {
            if let x509_parser::extensions::ParsedExtension::SubjectAlternativeName(san) =
                ext.parsed_extension()
            {
                for name in &san.general_names {
                    if let x509_parser::extensions::GeneralName::URI(uri) = name {
                        if uri.starts_with("spiffe://") {
                            return Some(uri.to_string());
                        }
                    }
                }
            }
        }
        None
    }
}

/// Build the tonic `ServerTlsConfig` from the per-container cert dir.
/// Requires client certs and validates them against the trust bundle.
pub fn server_tls_config() -> Result<ServerTlsConfig, std::io::Error> {
    let cert = fs::read(Path::new(CERTS_DIR).join("id.crt"))?;
    let key = fs::read(Path::new(CERTS_DIR).join("id.key"))?;
    let ca = fs::read(Path::new(CERTS_DIR).join("trust.ca.crt"))?;

    Ok(ServerTlsConfig::new()
        .identity(Identity::from_pem(&cert, &key))
        .client_ca_root(Certificate::from_pem(&ca)))
}

#[cfg(test)]
mod tests {
    use super::*;

    /// DER-encoded fixture cert with a SPIFFE URI SAN. Regenerate via:
    ///   ./scripts/secrets-gen.py
    ///   openssl x509 -in .secrets/certs/server/id.crt -outform DER | xxd -p | tr -d '\n'
    const FIXTURE_HEX: &str = include_str!("testdata/id.server.der.hex");

    fn fixture_der() -> Vec<u8> {
        let hex = FIXTURE_HEX.trim();
        (0..hex.len())
            .step_by(2)
            .map(|i| u8::from_str_radix(&hex[i..i + 2], 16).unwrap())
            .collect()
    }

    #[test]
    fn spiffe_uri_extracted_from_san() {
        let leaf = PeerLeafCert {
            der: fixture_der(),
        };
        let uri = leaf.spiffe_uri().expect("SPIFFE URI present");
        assert!(uri.starts_with("spiffe://"), "got: {uri}");
        assert!(
            uri.contains("server"),
            "expected SPIFFE URI to mention 'server', got: {uri}"
        );
    }

    #[test]
    fn spiffe_uri_returns_none_for_garbage_bytes() {
        let leaf = PeerLeafCert {
            der: vec![0u8; 8],
        };
        assert!(leaf.spiffe_uri().is_none());
    }
}
