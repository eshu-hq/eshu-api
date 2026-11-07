// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package auth

// OIDCLogin documents the D9 OIDC-login tier-C clone lane: the login
// start and callback GETs under /api/v0/auth/oidc (Refs
// eshu-hq/eshu#9669, part of #6707; first of the three sequential
// family lanes: OIDC, then GitHub, then SAML). Core ships the whole
// auth surface in one file, so no whole-file byte-identical port can
// serve these routes only; this subset keeps the line-verbatim
// property instead, gated by scripts/check-parity.sh.
//
// Provenance: each payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/auth/routes.go at the D9 wave
// pin 40e88be128ca876f7333588137af5a780bf31b93 (see
// docs/provenance.md). Keep it in lockstep with the handler and
// contracts/d9-oidc-login.md. The entries reference only the
// BrowserSessionResponse schema the D8 lane already ported — no new
// schemas. Tier-C dispatch only, not termination.
const OIDCLogin = `
    "/api/v0/auth/oidc/login": {
      "get": {
        "tags": ["auth"],
        "summary": "Start a dashboard OIDC login",
        "description": "Starts the backend OpenID Connect Authorization Code flow for dashboard users. The server stores only SHA-256 hashes for state, nonce, and redirect URI proof, then redirects the browser to the configured provider. Provider group claims are later mapped to Eshu roles and grants; raw OIDC tokens and raw group names are not persisted.",
        "operationId": "startOIDCLogin",
        "parameters": [
          {"name": "provider_config_id", "in": "query", "required": false, "schema": {"type": "string"}, "description": "Optional opaque provider config id. Defaults to the configured default provider."},
          {"name": "tenant_id", "in": "query", "required": false, "schema": {"type": "string"}, "description": "Optional tenant id; must match the selected provider config."},
          {"name": "workspace_id", "in": "query", "required": false, "schema": {"type": "string"}, "description": "Optional workspace id; must match the selected provider config."},
          {"name": "return_to", "in": "query", "required": false, "schema": {"type": "string"}, "description": "Optional same-origin return path after callback. Absolute URLs and protocol-relative paths are ignored."}
        ],
        "responses": {
          "302": {
            "description": "Redirect to the configured OIDC provider authorization endpoint.",
            "headers": {
              "Location": {"description": "Provider authorization URL with state and nonce.", "schema": {"type": "string"}}
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "401": {"$ref": "#/components/responses/Unauthorized"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/oidc/callback": {
      "get": {
        "tags": ["auth"],
        "summary": "Complete a dashboard OIDC login",
        "description": "Completes the backend Authorization Code callback, verifies issuer metadata/JWKS through the configured provider, validates state, nonce, redirect URI proof, and subject claims, then maps hashed external groups to Eshu role grants before issuing the same hash-only browser-session cookies used by explicit session creation. Unmapped groups or revoked grant mappings deny login and create no session.",
        "operationId": "completeOIDCLogin",
        "parameters": [
          {"name": "state", "in": "query", "required": true, "schema": {"type": "string"}},
          {"name": "code", "in": "query", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {
          "201": {
            "description": "OIDC login completed and browser session created. Returned when no return path was stored.",
            "headers": {
              "Set-Cookie": {
                "description": "Sets __Host-eshu_session and __Host-eshu_csrf with the same browser-session security attributes.",
                "schema": {"type": "string"}
              }
            },
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/BrowserSessionResponse"}
              }
            }
          },
          "303": {
            "description": "OIDC login completed and browser session cookies were set before redirecting to the stored same-origin return path.",
            "headers": {
              "Location": {"description": "Stored same-origin return path.", "schema": {"type": "string"}},
              "Set-Cookie": {"description": "Sets __Host-eshu_session and __Host-eshu_csrf.", "schema": {"type": "string"}}
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "401": {"$ref": "#/components/responses/Unauthorized"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
`
