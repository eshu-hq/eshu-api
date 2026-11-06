// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package auth

// AuthProviders documents the pre-auth login-picker discovery route:
// the tenant's derived sign-in posture GET under
// /api/v0/auth/providers (D2 Leaf B Native-now wave). Core ships the
// whole auth surface in one file, so no whole-file byte-identical port
// can serve this route only; this subset keeps the line-verbatim
// property instead, gated by scripts/check-parity.sh.
//
// Provenance: each payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/auth/routes.go at the wave pin
// (see docs/provenance.md). Keep it in lockstep with the handler and
// contracts/leafb-native-now.md.
const AuthProviders = `
    "/api/v0/auth/providers": {
      "get": {
        "tags": ["auth"],
        "summary": "Derive the tenant's pre-auth sign-in posture (issue #5165)",
        "description": "Public pre-auth endpoint scoped to a single tenant. Returns the tenant's derived AuthPosture: the configured OIDC/SAML providers available for interactive browser login, whether the local username/password form is offered, and whether self-service personal API tokens are offered — one discovery call for the console login picker instead of separate provider and sign-in-policy fetches. This is the single reusable posture derivation (query.DeriveAuthPosture) issue #5163 (F-2)'s MCP OAuth-discovery route also consumes to decide enablement. Provider entries expose only the opaque provider_config_id (required by the redirect endpoints), a safe generic display label, and a generic icon hint derived from the protocol class (never an IdP hostname, issuer, or operator-specific name). No secrets or private IdP configuration are ever returned. When tenant_id is absent or empty the safe zero-configuration default is returned (empty providers array, local login offered, self-service tokens offered) — the endpoint never performs a global cross-tenant scan. When no providers are configured for the tenant an empty array is returned. The response carries Cache-Control: public, max-age=60 — an admin enabling or disabling a provider is reflected on the tenant's NEXT (uncached) load, no restart required.",
        "operationId": "listAuthProviders",
        "security": [],
        "parameters": [
          {
            "name": "tenant_id",
            "in": "query",
            "required": false,
            "schema": {"type": "string"},
            "description": "Tenant to derive the sign-in posture for. When absent or empty the response is always the safe zero-configuration default — no global cross-tenant scan is performed."
          }
        ],
        "responses": {
          "200": {
            "description": "The tenant's derived sign-in posture. Empty providers array when tenant_id is absent or no providers are configured.",
            "headers": {
              "Cache-Control": {
                "description": "Always set to 'public, max-age=60'.",
                "schema": {"type": "string"}
              }
            },
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "required": ["providers", "local_login_offered", "self_service_tokens_offered"],
                  "properties": {
                    "providers": {
                      "type": "array",
                      "items": {
                        "type": "object",
                        "required": ["provider_config_id", "display_label", "provider_kind", "icon_hint"],
                        "properties": {
                          "provider_config_id": {
                            "type": "string",
                            "description": "Opaque operator-assigned provider config identifier. Required by the OIDC and SAML redirect endpoints."
                          },
                          "display_label": {
                            "type": "string",
                            "description": "Safe generic label for the login button. Always a generic protocol-class label (e.g. 'Single sign-on (OIDC)'). Never echoes a domain, org name, or IdP identifier."
                          },
                          "provider_kind": {
                            "type": "string",
                            "enum": ["oidc", "saml", "github"],
                            "description": "Protocol class: oidc, saml, or github. Used by the console to select the correct redirect helper. Unlike oidc/saml (a protocol class only), github additionally identifies the specific connector (issue #5166) since a GitHub provider IS github.com or a GitHub Enterprise Server instance by construction."
                          },
                          "icon_hint": {
                            "type": "string",
                            "enum": ["oidc", "saml", "github"],
                            "description": "Generic icon selector for the login button, derived from provider_kind. For oidc/saml this carries no IdP brand information — only the protocol class; github is the one exception (see provider_kind's description)."
                          }
                        }
                      }
                    },
                    "local_login_offered": {
                      "type": "boolean",
                      "description": "Whether the console should render the local username/password form. A UX hint only: false exactly when the tenant's sign-in policy has require_sso=true. The server's break-glass admin path (POST /api/v0/auth/local/login) always stays reachable regardless of this value."
                    },
                    "self_service_tokens_offered": {
                      "type": "boolean",
                      "description": "Whether an authenticated caller may self-issue a personal API token (issue #5164). Always true today — self-service token creation has no sign-in-policy gate."
                    }
                  }
                }
              }
            }
          },
          "500": {"$ref": "#/components/responses/InternalError"}
        }
      }
    },
`
