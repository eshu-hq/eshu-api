// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package auth

// AdminProviderConfigReads documents the tenant-scoped admin
// provider-config reads: the list, detail, and revisions GETs under
// /api/v0/auth/admin/provider-configs (D2 Leaf B Native-now wave). The
// create and update POSTs share path keys with the list and detail
// GETs, so per the core OpenAPI convention they live in this reads
// fragment (the same convention as the AdminMutations/AdminReads
// split); this lane ports the reads entries alongside them so the
// served spec carries the full read surface.
//
// Provenance: each payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/auth/admin_provider_configs.go
// at commit 99381691b1ac49254e813e5a013cfb654d432491 (sha256
// afa45de81720f466b8b9f66949605564be21e9efa8ddb8c62e71600b127824ef).
// Core ships reads and mutations in one file, so no whole-file
// byte-identical port can serve reads only; this subset keeps the
// line-verbatim property instead, gated by scripts/check-parity.sh.
// Keep it in lockstep with the handler and
// contracts/leafb-native-now.md.
const AdminProviderConfigReads = `
    "/api/v0/auth/admin/provider-configs": {
      "get": {
        "tags": ["auth"],
        "summary": "List the tenant's DB-backed and env-registered identity provider configs",
        "description": "All-scopes admin route. Merges DB-backed provider configs with env/file-registered providers (env-file authoritative): a DB row whose provider_config_id matches an env-registered provider is returned with shadowed_by_environment=true and its sealed secret is never consulted for login. Never returns a secret — only has_secret, secret_fingerprint, and key_id.",
        "operationId": "listAdminProviderConfigs",
        "x-scoped-token-support": true,
        "responses": {
          "200": {
            "description": "The tenant's provider configs.",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "provider_configs": {
                      "type": "array",
                      "items": {"$ref": "#/components/schemas/AdminProviderConfig"}
                    },
                    "truncated": {"type": "boolean"}
                  }
                }
              }
            }
          },
          "403": {"$ref": "#/components/responses/Forbidden"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      },
      "post": {
        "tags": ["auth"],
        "summary": "Create a DB-backed identity provider config",
        "description": "All-scopes admin route that creates a new draft provider config with one active revision carrying the sealed secret. client_secret (oidc) or sp_private_key/sp_certificate (saml) are write-only: never echoed back, never logged. The provider cannot be enabled until it passes a test-connection call. Emits a governance audit event.",
        "operationId": "createAdminProviderConfig",
        "x-scoped-token-support": true,
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/AdminProviderConfigWriteRequest"}
            }
          }
        },
        "responses": {
          "200": {
            "description": "The created provider config's id and revision.",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/AdminProviderConfigWriteResult"}
              }
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "409": {"description": "A provider config already exists for this tenant, kind, and identity key."},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/admin/provider-configs/{provider_config_id}": {
      "get": {
        "tags": ["auth"],
        "summary": "Get one identity provider config",
        "description": "All-scopes admin route. Never returns a secret — only has_secret, secret_fingerprint, and key_id.",
        "operationId": "getAdminProviderConfig",
        "x-scoped-token-support": true,
        "parameters": [{"name": "provider_config_id", "in": "path", "required": true, "schema": {"type": "string"}}],
        "responses": {
          "200": {
            "description": "The provider config.",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/AdminProviderConfig"}
              }
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "404": {"$ref": "#/components/responses/NotFound"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      },
      "post": {
        "tags": ["auth"],
        "summary": "Update an identity provider config, creating a new active revision",
        "description": "All-scopes admin route. Every change creates a new audited revision superseding the current one. The full secret must be resupplied on every update — write-only secrets are never carried forward automatically (the AAD binds each sealed envelope to its own revision id specifically to prevent that). Emits a governance audit event.",
        "operationId": "updateAdminProviderConfig",
        "x-scoped-token-support": true,
        "parameters": [{"name": "provider_config_id", "in": "path", "required": true, "schema": {"type": "string"}}],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/AdminProviderConfigWriteRequest"}
            }
          }
        },
        "responses": {
          "200": {
            "description": "The updated provider config's new revision.",
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
    "/api/v0/auth/admin/provider-configs/{provider_config_id}/revisions": {
      "get": {
        "tags": ["auth"],
        "summary": "List a provider config's revision history",
        "description": "All-scopes admin route. Never returns a secret — only has_secret per revision.",
        "operationId": "listAdminProviderConfigRevisions",
        "x-scoped-token-support": true,
        "parameters": [{"name": "provider_config_id", "in": "path", "required": true, "schema": {"type": "string"}}],
        "responses": {
          "200": {
            "description": "The provider config's revision history, newest first.",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "revisions": {
                      "type": "array",
                      "items": {
                        "type": "object",
                        "properties": {
                          "revision_id": {"type": "string"},
                          "status": {"type": "string"},
                          "has_secret": {"type": "boolean"},
                          "created_at": {"type": "string", "format": "date-time"},
                          "activated_at": {"type": "string", "format": "date-time"},
                          "superseded_at": {"type": "string", "format": "date-time"}
                        }
                      }
                    }
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
