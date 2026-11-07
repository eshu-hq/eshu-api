// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package auth

// GitHubLogin documents the D9 GitHub-login tier-C clone lane: the login
// start and callback GETs under /api/v0/auth/github (Refs
// eshu-hq/eshu#9669, part of #6707; second of the three sequential
// family lanes: OIDC, then GitHub, then SAML). Core ships the whole
// auth surface in one file, so no whole-file byte-identical port can
// serve these routes only; this subset keeps the line-verbatim
// property instead, gated by scripts/check-parity.sh.
//
// Provenance: each payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/auth/routes.go at the D9 GitHub
// wave pin 8c2324693320c8f69ef3e8078c795956b549d7ad (see
// docs/provenance.md). Keep it in lockstep with the handler and
// contracts/d9-github-login.md. The entries reference only the
// BrowserSessionResponse schema the D8 lane already ported — no new
// schemas. Tier-C dispatch only, not termination.
const GitHubLogin = `
    "/api/v0/auth/github/login": {
      "get": {
        "tags": ["auth"],
        "summary": "Start a dashboard GitHub login (issue #5166, F-5)",
        "description": "Starts the backend GitHub Authorization Code (plain OAuth2) flow for dashboard users. Unlike /api/v0/auth/oidc/login, this is NOT OpenID Connect: github.com publishes no discovery document, issues no ID token, and has no JWKS — identity is resolved entirely by calling the GitHub REST API with the exchanged access token (see internal/githublogin's doc.go). The server stores only a SHA-256 hash of the state parameter (no nonce: plain OAuth2 has none) and a hash of the redirect URI, then redirects the browser to the configured GitHub (or GitHub Enterprise Server) authorization endpoint.",
        "operationId": "startGitHubLogin",
        "parameters": [
          {"name": "provider_config_id", "in": "query", "required": false, "schema": {"type": "string"}, "description": "Optional opaque provider config id. Defaults to the configured default provider."},
          {"name": "tenant_id", "in": "query", "required": false, "schema": {"type": "string"}, "description": "Optional tenant id; must match the selected provider config."},
          {"name": "workspace_id", "in": "query", "required": false, "schema": {"type": "string"}, "description": "Optional workspace id; must match the selected provider config."},
          {"name": "return_to", "in": "query", "required": false, "schema": {"type": "string"}, "description": "Optional same-origin return path after callback. Absolute URLs and protocol-relative paths are ignored."}
        ],
        "responses": {
          "302": {
            "description": "Redirect to the configured GitHub authorization endpoint.",
            "headers": {
              "Location": {"description": "Provider authorization URL with state.", "schema": {"type": "string"}}
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "401": {"$ref": "#/components/responses/Unauthorized"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/github/callback": {
      "get": {
        "tags": ["auth"],
        "summary": "Complete a dashboard GitHub login (issue #5166, F-5)",
        "description": "Completes the GitHub Authorization Code callback: validates the one-time state, exchanges the code for an access token, resolves the caller's verified primary email, active GitHub org memberships, and team memberships from the GitHub REST API, denies login (with an audited server-side reason; no session created) when the caller has no verified email or no active membership in the provider's configured allowed_orgs, then maps hashed team handles (\"org/team-slug\") to Eshu role grants through the SAME group-to-role resolver an OIDC login's group claims use, before issuing the same hash-only browser-session cookies used by explicit session creation.",
        "operationId": "completeGitHubLogin",
        "parameters": [
          {"name": "state", "in": "query", "required": true, "schema": {"type": "string"}},
          {"name": "code", "in": "query", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {
          "201": {
            "description": "GitHub login completed and browser session created. Returned when no return path was stored.",
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
            "description": "GitHub login completed and browser session cookies were set before redirecting to the stored same-origin return path.",
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
