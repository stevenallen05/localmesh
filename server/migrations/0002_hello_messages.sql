-- One row per Greeter.SayHello invocation. Demonstrates persisting
-- verified-JWT identity (vs telemetry-only enduser.* stamping at the
-- Caddy ingress). The PII-at-ingress rule constrains telemetry, not
-- business data — see docs/stakeholder/DESIGN_DECISIONS.md row :56.
CREATE TABLE hello_messages (
    id          BIGSERIAL    PRIMARY KEY,
    message     TEXT         NOT NULL,
    jwt_issuer  TEXT         NOT NULL,
    jwt_subject TEXT         NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
