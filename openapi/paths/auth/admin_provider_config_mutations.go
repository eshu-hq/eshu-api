// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package auth

// AdminProviderConfigMutations documents the tenant-scoped admin
// provider-config mutation endpoints that occupy their own path keys:
// the revert, enable, disable, and test-connection POSTs. The create
// (POST) and update (POST) operations share a path key with their
// read-list/detail GET, so they are merged into the reads fragment
// rather than declared here: an OpenAPI paths object cannot carry the
// same path key twice (the same convention as the admin-identity
// AdminMutations/AdminReads split). This fragment holds only the
// routes whose path key is unique to the mutation surface.
//
// Every route requires all-scope admin authentication, writes strictly
// within the caller's own tenant (derived from the session, never a
// request body), is idempotent under retry, and emits a governance
// audit event. No route accepts or returns a secret in plaintext —
// has_secret, secret_fingerprint, and key_id are the only
// secret-adjacent response fields.
//
// Provenance: each payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/auth/admin_provider_configs.go
// at commit 99381691b1ac49254e813e5a013cfb654d432491 (sha256
// afa45de81720f466b8b9f66949605564be21e9efa8ddb8c62e71600b127824ef).
// Core ships reads and mutations in one file, so no whole-file
// byte-identical port can serve mutations only; this subset keeps the
// line-verbatim property instead, gated by scripts/check-parity.sh,
// and the reads-split follow-up ports the remaining path keys.
const AdminProviderConfigMutations = `
    "/api/v0/auth/admin/provider-configs/{provider_config_id}/revert": {
      "post": {
        "tags": ["auth"],
        "summary": "Revert a provider config to a prior revision",
        "description": "All-scopes admin route. Activates a prior revision, restoring its sealed secret automatically (no secret is transmitted or re-entered). Idempotent: reverting to the already-active revision is a safe no-op. Emits a governance audit event.",
        "operationId": "revertAdminProviderConfig",
        "x-scoped-token-support": true,
        "parameters": [{"name": "provider_config_id", "in": "path", "required": true, "schema": {"type": "string"}}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["revision_id"],
                "properties": {"revision_id": {"type": "string"}}
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "The reactivated revision.",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/AdminProviderConfigWriteResult"}
              }
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "404": {"$ref": "#/components/responses/NotFound"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/admin/provider-configs/{provider_config_id}/enable": {
      "post": {
        "tags": ["auth"],
        "summary": "Enable a provider config",
        "description": "All-scopes admin route. Re-runs a test-connection for the current active revision synchronously and only transitions the provider to active if it passes; a draft provider without a passing test cannot be enabled. For a login-capable provider kind (oidc, saml, github), also rejects if the stored configuration is missing a field required to resolve the provider for login (redirect_url for oidc/github; service_provider_entity_id, service_provider_acs_url, or inline metadata_xml for saml) even though those fields are optional at create/test-connection time. Emits a governance audit event.",
        "operationId": "enableAdminProviderConfig",
        "x-scoped-token-support": true,
        "parameters": [{"name": "provider_config_id", "in": "path", "required": true, "schema": {"type": "string"}}],
        "responses": {
          "200": {
            "description": "The provider config's new status.",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/AdminProviderConfigWriteResult"}
              }
            }
          },
          "400": {"description": "The provider cannot be enabled: either the connection test did not pass, or the stored configuration is missing a field required to resolve the provider for login."},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "404": {"$ref": "#/components/responses/NotFound"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/admin/provider-configs/{provider_config_id}/disable": {
      "post": {
        "tags": ["auth"],
        "summary": "Disable a provider config",
        "description": "All-scopes admin route that transitions an active provider config back to draft. Idempotent. Emits a governance audit event.",
        "operationId": "disableAdminProviderConfig",
        "x-scoped-token-support": true,
        "parameters": [{"name": "provider_config_id", "in": "path", "required": true, "schema": {"type": "string"}}],
        "responses": {
          "200": {
            "description": "The provider config's new status.",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/AdminProviderConfigWriteResult"}
              }
            }
          },
          "403": {"$ref": "#/components/responses/Forbidden"},
          "404": {"$ref": "#/components/responses/NotFound"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/admin/provider-configs/{provider_config_id}/test-connection": {
      "post": {
        "tags": ["auth"],
        "summary": "Test a provider config's connection",
        "description": "All-scopes admin route that validates OIDC discovery/JWKS reachability (or SAML IdP metadata) and that the stored secret decrypts to well-formed material. Does not perform a live OAuth2 authorization-code round trip or a live SAML SSO exchange — see the oidclogin/samlauth TestConnection doc comments for the exact, bounded scope. Emits a governance audit event.",
        "operationId": "testAdminProviderConfigConnection",
        "x-scoped-token-support": true,
        "parameters": [{"name": "provider_config_id", "in": "path", "required": true, "schema": {"type": "string"}}],
        "responses": {
          "200": {
            "description": "The test-connection result.",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "provider_config_id": {"type": "string"},
                    "ok": {"type": "boolean"},
                    "detail": {"type": "string"}
                  }
                }
              }
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
`
