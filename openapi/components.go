// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

// components is the minimal shared block for the dest-served spec:
// exactly the schemas and responses the landed family fragments
// reference, nothing more. Entry bytes are ported from the pinned core
// commits 428790ab3ac0bff2eb28e50a26b3746a9b7dc8c5 (sign-in-policy lane)
// and ebaf4dfa0de672851bbec7a427e28fdb41344380 (retirement lane). The
// terminal-failure lane adds no entries: its fragment references only
// ErrorResponse plus the five responses above (gated in check-parity.sh):
//
//   - AdminSignInPolicy, AdminSignInPolicyUpdateRequest: schemas from
//     go/internal/query/openapi/components_sign_in_policy.go.
//   - ErrorResponse: schema from go/internal/query/openapi/components.go.
//   - BadRequest, Forbidden, Conflict, InternalError, ServiceUnavailable:
//     responses from go/internal/query/openapi/components_responses.go.
//
// A new family lane extends this block with the entries its fragment
// references; TestSpec_LocalRefsResolve fails closed on a dangling $ref.
const components = `  "components": {
    "schemas": {
      "AdminSignInPolicy": {
        "type": "object",
        "description": "The tenant sign-in policy. sso_admin_verified_at/sso_admin_verified_provider_config_id are only present once at least one admin has completed one SSO sign-in for this tenant; neither is ever a secret.",
        "properties": {
          "tenant_id": {"type": "string"},
          "require_sso": {"type": "boolean", "description": "Hides the local password form on the login page and rejects non-admin local login. Break-glass local admin sign-in stays reachable regardless."},
          "allow_local_user_creation": {"type": "boolean", "description": "false rejects new local-identity invitation creation. Default true."},
          "require_mfa_for_all_users": {"type": "boolean", "description": "Admins always require MFA regardless of this flag; true extends the requirement to every local-identity invitation acceptance."},
          "idle_timeout_seconds": {"type": "integer", "description": "0 means \"use the process default\" (DefaultBrowserSessionIdleTimeout)."},
          "absolute_timeout_seconds": {"type": "integer", "description": "0 means \"use the process default\" (DefaultBrowserSessionAbsoluteTimeout)."},
          "sso_admin_verified_at": {"type": "string", "format": "date-time"},
          "sso_admin_verified_provider_config_id": {"type": "string"},
          "policy_revision_hash": {"type": "string"},
          "updated_at": {"type": "string", "format": "date-time"}
        }
      },
      "AdminSignInPolicyUpdateRequest": {
        "type": "object",
        "description": "Partial update. Every field is optional; an absent field leaves the current value unchanged.",
        "properties": {
          "require_sso": {"type": "boolean"},
          "allow_local_user_creation": {"type": "boolean"},
          "require_mfa_for_all_users": {"type": "boolean"},
          "idle_timeout_seconds": {"type": "integer", "minimum": 0, "description": "0 means \"use the process default\"; any other value must be at least 60 seconds."},
          "absolute_timeout_seconds": {"type": "integer", "minimum": 0, "description": "0 means \"use the process default\"; any other value must be at least 60 seconds and not less than idle_timeout_seconds when both are set."}
        }
      },
      "ErrorResponse": {
        "type": "object",
        "properties": {
          "error": {"type": "string"},
          "detail": {"type": "string"},
          "error_code": {"type": "string"},
          "message": {"type": "string"},
          "correlation_id": {"type": "string"},
          "details": {"type": "object"}
        }
      }
    },
    "responses": {
      "BadRequest": {
        "description": "Bad request",
        "content": {
          "application/json": {
            "schema": {"$ref": "#/components/schemas/ErrorResponse"}
          }
        }
      },
      "Forbidden": {
        "description": "Permission denied",
        "content": {
          "application/json": {
            "schema": {"$ref": "#/components/schemas/ErrorResponse"}
          }
        }
      },
      "Conflict": {
        "description": "Ambiguous request or conflicting scope",
        "content": {
          "application/json": {
            "schema": {"$ref": "#/components/schemas/ErrorResponse"}
          }
        }
      },
      "InternalError": {
        "description": "Internal server error",
        "content": {
          "application/json": {
            "schema": {"$ref": "#/components/schemas/ErrorResponse"}
          }
        }
      },
      "ServiceUnavailable": {
        "description": "Service unavailable. A graph backend outage, or a PostgreSQL read replica that has not replayed to the writer checkpoint, or whose connection acquisition (pool wait or dial) or identity check timed out inside the replay window, answers with the stable backend_unavailable error code, a fixed message, and a Retry-After hint; those graph-read availability verdicts are transient and the request is safe to retry. Any other 503 backend_unavailable, such as a route that needs a graph backend the deployment did not configure, is a configuration state and carries no Retry-After. A PostgreSQL reader build that is not qualified for shared-snapshot export (#7957) answers a permanent backend_unavailable with error.details.reason=snapshot_export_unqualified and no Retry-After on the routes that would run a read on one shared snapshot; it is a configuration state and retrying does not help. A PostgreSQL reader failure that is not a timeout (authentication or TLS failure, connection refused, permission denied) answers 500.",
        "headers": {
          "Retry-After": {
            "description": "Seconds to wait before retrying. Present only on the transient graph-read availability 503 verdicts (graph unavailable, stale or timed-out PostgreSQL reader) and a failed checkpoint step's 503; absent from a permanent 503 backend_unavailable, including a checkpoint source that was never configured.",
            "schema": {"type": "integer", "minimum": 1}
          }
        },
        "content": {
          "application/json": {
            "schema": {"$ref": "#/components/schemas/ErrorResponse"}
          }
        }
      }
    }
  }
}`
