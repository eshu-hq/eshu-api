// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package auth

// AdminInvitations documents the tenant-scoped admin invitations
// list GET plus the D8 LocalIdentity create POST sharing its path key
// (Refs eshu-hq/eshu#9669). D8 merges the POST here: the served
// document carries both landed methods for the key, and the key stays
// unique to this fragment. The entries reference only the shared
// responses (BadRequest, Forbidden, InternalError, ServiceUnavailable)
// plus the LocalIdentityInvitationRequest schema the D8 lane ports, so
// no other schema moves with them.
//
// Provenance: each payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/auth/routes.go
// at commit 91f2df28dbe9ec4e5fb8c5df59a76475a304d84 (sha256
// 709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a,
// byte-identical there and at the D2 wave pin). Core ships both entries
// in one path entry, so no whole-file byte-identical port can serve
// this lane only; this subset keeps the line-verbatim property
// instead, gated by scripts/check-parity.sh.
const AdminInvitations = `
    "/api/v0/auth/local/invitations": {
      "post": {
        "tags": ["auth"],
        "summary": "Create a local identity invitation",
        "description": "Creates a hash-only invitation for assignment-required signup. This route requires all-scopes admin authentication; broad open self-signup is intentionally not supported.",
        "operationId": "createLocalIdentityInvitation",
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"$ref": "#/components/schemas/LocalIdentityInvitationRequest"}}}
        },
        "responses": {
          "201": {"description": "Invitation created. The invite_code is returned once and only the hash is persisted."},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      },
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
