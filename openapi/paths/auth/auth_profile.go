// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package auth

// AuthProfile documents the D8 profile read (Refs eshu-hq/eshu#9669):
// the caller-identity profile GET (tier-C dispatch clone, not
// termination). Each payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/auth/routes.go
// at commit aa58debdd8102042f6c7e8b8e2e2944805889ef3 (sha256
// 709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a,
// byte-identical there and at the D2 wave and D8 stale pins). Same subset treatment
// as the BrowserSessions fragment, gated by scripts/check-parity.sh.
const AuthProfile = `
    "/api/v0/auth/profile": {
      "get": {
        "tags": ["auth"],
        "summary": "Read the caller's identity profile",
        "description": "Returns the authenticated caller's own profile: identity provider config id (absent for local identity), active tenant/workspace, role ids, permitted permission features, MFA status, and memberships. Cookie- or bearer-authenticated; never returns secrets, credential handles, or other subjects' data.",
        "operationId": "getAuthProfile",
        "x-scoped-token-support": true,
        "responses": {
          "200": {
            "description": "Caller profile.",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "external_provider_config_id": {"type": "string"},
                    "active_tenant_id": {"type": "string"},
                    "active_workspace_id": {"type": "string"},
                    "role_ids": {"type": "array", "items": {"type": "string"}},
                    "allowed_permission_features": {"type": "array", "items": {"type": "string"}},
                    "permission_catalog_enforced": {"type": "boolean"},
                    "mfa": {
                      "type": "object",
                      "properties": {
                        "has_active_mfa": {"type": "boolean"},
                        "factor_kind": {"type": "string"}
                      }
                    },
                    "memberships": {
                      "type": "array",
                      "items": {
                        "type": "object",
                        "properties": {
                          "tenant_id": {"type": "string"},
                          "workspace_id": {"type": "string"}
                        }
                      }
                    }
                  }
                }
              }
            }
          },
          "401": {"$ref": "#/components/responses/Unauthorized"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
`
