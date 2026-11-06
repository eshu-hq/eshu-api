// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package auth

// AdminInvitations documents the tenant-scoped admin invitations list
// GET whose path key is shared with the LocalIdentity create POST.
// The create POST stays on the generic relay (D8 lane: identity
// writes persist through core-owned session/token stores with no
// admission pin yet), so this fragment ports only the GET entry —
// never the mutation key's POST. Per the core OpenAPI convention the
// served document shows the landed method only; D8 merges the POST
// when that lane ports it. The entry references only the shared
// responses (Forbidden, InternalError, ServiceUnavailable), so the
// lane adds no schemas.
//
// Provenance: each payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/auth/routes.go
// at commit 17feac86e0 (sha256
// 709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a,
// the D2 wave pin — routes.go is byte-identical there and at the D1
// pin). Core ships the lane entry and the relay-only POST in one
// path entry, so no whole-file byte-identical port can serve this lane
// only; this subset keeps the line-verbatim property instead, gated by
// scripts/check-parity.sh.
const AdminInvitations = `
    "/api/v0/auth/local/invitations": {
      "get": {
        "tags": ["auth"],
        "summary": "List the tenant's invitations",
        "description": "All-scopes admin route that lists invitations within the caller's own tenant/workspace: invite id, role id, status, and lifecycle timestamps. Never returns the invite code, invitee handle, or inviter identity (all stored only as hashes).",
        "operationId": "listAdminInvitations",
        "x-scoped-token-support": true,
        "responses": {
          "200": {
            "description": "The tenant's invitations.",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "invitations": {
                      "type": "array",
                      "items": {
                        "type": "object",
                        "properties": {
                          "invite_id": {"type": "string"},
                          "role_id": {"type": "string"},
                          "status": {"type": "string"},
                          "expires_at": {"type": "string", "format": "date-time"},
                          "accepted_at": {"type": "string", "format": "date-time"},
                          "revoked_at": {"type": "string", "format": "date-time"},
                          "created_at": {"type": "string", "format": "date-time"},
                          "updated_at": {"type": "string", "format": "date-time"},
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
          "403": {"$ref": "#/components/responses/Forbidden"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
`
