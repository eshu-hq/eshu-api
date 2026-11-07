// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package auth

// SAMLLogin documents the D9 SAML-login tier-C clone lane: the metadata
// and login GETs plus the ACS POST under /api/v0/auth/saml (Refs
// eshu-hq/eshu#9669, part of #6707; third of the three sequential
// family lanes: OIDC, then GitHub, then SAML). Core ships the whole
// auth surface in one file, so no whole-file byte-identical port can
// serve these routes only; this subset keeps the line-verbatim
// property instead, gated by scripts/check-parity.sh.
//
// Provenance: each payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/auth/routes.go at the D9 SAML
// wave pin 8c2324693320c8f69ef3e8078c795956b549d7ad (see
// docs/provenance.md). Keep it in lockstep with the handler and
// contracts/d9-saml-login.md. The entries reference only the
// BrowserSessionResponse schema the D8 lane already ported — no new
// schemas. Tier-C dispatch only, not termination.
const SAMLLogin = `
    "/api/v0/auth/saml/providers/{provider_id}/metadata": {
      "get": {
        "tags": ["auth"],
        "summary": "Read SAML service-provider metadata",
        "description": "public route that returns Eshu service-provider metadata for a configured SAML provider. The metadata advertises the provider-specific EntityID and POST ACS endpoint only; raw assertions, raw NameID values, group claims, private metadata URLs, provider secrets, and browser-session state are never returned.",
        "operationId": "getSAMLServiceProviderMetadata",
        "parameters": [
          {
            "name": "provider_id",
            "in": "path",
            "required": true,
            "schema": {"type": "string"},
            "description": "Opaque SAML provider configuration identifier."
          }
        ],
        "responses": {
          "200": {
            "description": "SAML service-provider metadata XML.",
            "content": {
              "application/samlmetadata+xml": {
                "schema": {"type": "string"}
              }
            }
          },
          "404": {"$ref": "#/components/responses/NotFound"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/saml/providers/{provider_id}/login": {
      "get": {
        "tags": ["auth"],
        "summary": "Start SAML login",
        "description": "public SP-initiated SAML login route. Eshu creates an AuthnRequest, generates a RelayState secret, stores only the RelayState SHA-256 hash with the request ID and short expiry, and redirects the browser to the IdP SSO endpoint. The raw RelayState is used only in the browser redirect and is required by the ACS route before any browser session can be created.",
        "operationId": "startSAMLLogin",
        "parameters": [
          {
            "name": "provider_id",
            "in": "path",
            "required": true,
            "schema": {"type": "string"},
            "description": "Opaque SAML provider configuration identifier."
          },
          {"name": "return_to", "in": "query", "required": false, "schema": {"type": "string"}, "description": "Optional same-origin return path after the ACS callback. Absolute URLs and protocol-relative paths are ignored."}
        ],
        "responses": {
          "302": {
            "description": "Redirect to the IdP SSO endpoint.",
            "headers": {
              "Location": {
                "description": "IdP SSO URL carrying SAMLRequest and RelayState.",
                "schema": {"type": "string"}
              }
            }
          },
          "404": {"$ref": "#/components/responses/NotFound"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/saml/providers/{provider_id}/acs": {
      "post": {
        "tags": ["auth"],
        "summary": "Complete SAML login at the assertion consumer service",
        "description": "public ACS route that accepts IdP POSTed RelayState and SAMLResponse form fields. Eshu consumes the RelayState hash, validates the signed assertion through the configured SAML provider, reserves a hash-only replay key before session creation, maps NameID and group claims through Eshu-owned memberships and grants, and then creates the normal __Host-eshu_session and __Host-eshu_csrf browser cookies. The server persists only SHA-256 hashes for session, CSRF, RelayState, subject, group-claim, and replay material; raw SAML assertions and raw claim values are not stored or returned.",
        "operationId": "postSAMLAssertionConsumerService",
        "parameters": [
          {
            "name": "provider_id",
            "in": "path",
            "required": true,
            "schema": {"type": "string"},
            "description": "Opaque SAML provider configuration identifier."
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/x-www-form-urlencoded": {
              "schema": {
                "type": "object",
                "required": ["RelayState", "SAMLResponse"],
                "properties": {
                  "RelayState": {"type": "string"},
                  "SAMLResponse": {"type": "string"}
                }
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": "SAML login accepted; Set-Cookie includes __Host-eshu_session and __Host-eshu_csrf. Returned when no return_to path was stored.",
            "headers": {
              "Set-Cookie": {
                "description": "__Host-eshu_session is HttpOnly/Secure/SameSite=Strict; __Host-eshu_csrf is readable by the browser client and must be echoed in X-Eshu-CSRF for unsafe requests.",
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
            "description": "SAML login accepted; browser redirected to the same-origin return_to path stored at login initiation. Set-Cookie is set before the redirect.",
            "headers": {
              "Location": {
                "description": "Same-origin path from the original return_to query parameter.",
                "schema": {"type": "string"}
              },
              "Set-Cookie": {
                "description": "__Host-eshu_session and __Host-eshu_csrf browser-session cookies.",
                "schema": {"type": "string"}
              }
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "401": {"$ref": "#/components/responses/Unauthorized"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
`
