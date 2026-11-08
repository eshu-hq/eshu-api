// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package auth

// SessionList documents the D8 session-list read (Refs
// eshu-hq/eshu#9669): the metadata-only caller-sessions GET (tier-C
// dispatch clone, not termination). Each payload line below is verbatim
// from the pinned core file go/internal/query/openapi/paths/auth/routes.go
// at commit aa58debdd8102042f6c7e8b8e2e2944805889ef3 (sha256
// 709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a,
// byte-identical there and at the D2 wave and D8 stale pins). Same subset treatment
// as the BrowserSessions fragment, gated by scripts/check-parity.sh.
const SessionList = `
    "/api/v0/auth/sessions": {
      "get": {
        "tags": ["auth"],
        "summary": "List the caller's own browser sessions",
        "description": "Returns metadata for the authenticated caller's own active browser sessions: issued/last-seen/expiry timestamps, tenant/workspace, and which row is the current session. Never returns the session hash or secret, and never returns other subjects' sessions.",
        "operationId": "listAuthSessions",
        "x-browser-session-only": true,
        "parameters": [
          {
            "name": "limit",
            "in": "query",
            "description": "Maximum number of sessions to return (default 20, max 100). Invalid or missing values use the default.",
            "schema": {"type": "integer", "minimum": 1, "maximum": 100, "default": 20}
          },
          {
            "name": "offset",
            "in": "query",
            "description": "Number of sessions to skip before the first result (default 0).",
            "schema": {"type": "integer", "minimum": 0, "default": 0}
          }
        ],
        "responses": {
          "200": {
            "description": "The caller's browser sessions.",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "sessions": {
                      "type": "array",
                      "items": {
                        "type": "object",
                        "properties": {
                          "issued_at": {"type": "string", "format": "date-time"},
                          "last_seen_at": {"type": "string", "format": "date-time"},
                          "idle_expires_at": {"type": "string", "format": "date-time"},
                          "absolute_expires_at": {"type": "string", "format": "date-time"},
                          "tenant_id": {"type": "string"},
                          "workspace_id": {"type": "string"},
                          "current": {"type": "boolean"},
                          "revoked_at": {"type": "string", "format": "date-time"}
                        }
                      }
                    },
                    "prev": {
                      "type": "string",
                      "description": "Relative URL to the previous page of results. Present only when offset > 0."
                    },
                    "next": {
                      "type": "string",
                      "description": "Relative URL to the next page of results. Present only when more results exist."
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
