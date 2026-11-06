// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

// components is the minimal shared block for the dest-served spec:
// exactly the schemas and responses the landed family fragments
// reference, nothing more. Entry bytes are ported from the pinned core
// commits 428790ab3ac0bff2eb28e50a26b3746a9b7dc8c5 (sign-in-policy lane)
// and ebaf4dfa0de672851bbec7a427e28fdb41344380 (retirement,
// terminal-failure, and admin-identity lanes),
// 99381691b1ac49254e813e5a013cfb654d432491 (provider-config lane),
// and 7c60583996d9a3ccc46288f40a38f614a4f28ea8 (recovery lane).
// The terminal-failure lane adds no entries: its fragment references
// only ErrorResponse plus the five responses above (gated in
// check-parity.sh):
//
//   - AdminSignInPolicy, AdminSignInPolicyUpdateRequest: schemas from
//     go/internal/query/openapi/components_sign_in_policy.go.
//   - AdminAuditCountList: schema from
//     go/internal/query/openapi/components_local_identity.go (admin-identity
//     reads lane: the audit-summary fragment response counts by it).
//   - AdminProviderConfigWriteResult: schema from
//     go/internal/query/openapi/components_provider_configs.go.
//   - SkippedScopesReport: schema from
//     go/internal/query/openapi/components.go (recovery lane: the
//     refinalize and recover-generations entries reference it, and no
//     relay-only route does).
//   - DeltaActiveScopesReport, ReindexRequestsWrittenReport: schemas
//     from go/internal/query/openapi/components_recovery.go (same
//     lane, same two entries, no relay-only reference).
//   - ErrorResponse: schema from go/internal/query/openapi/components.go.
//   - BadRequest, NotFound, Forbidden, Conflict, InternalError,
//     ServiceUnavailable: responses from
//     go/internal/query/openapi/components_responses.go.
//
// A new family lane extends this block with the entries its fragment
// references; TestSpec_LocalRefsResolve fails closed on a dangling $ref.
const components = `  "components": {
    "parameters": {
      "ServiceName": {
        "name": "service_name",
        "in": "path",
        "required": true,
        "schema": {"type": "string"},
        "description": "Service name"
      }
    },
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
      "AdminProviderConfig": {
        "type": "object",
        "description": "Metadata-only admin view of one identity provider config. Never carries a secret — has_secret, secret_fingerprint (a non-reversible short hash of the sealed envelope's ciphertext), and key_id are the only secret-adjacent fields.",
        "properties": {
          "provider_config_id": {"type": "string"},
          "provider_kind": {"type": "string", "enum": ["external_oidc", "external_saml", "external_github"]},
          "status": {"type": "string", "enum": ["draft", "active"]},
          "active_revision_id": {"type": "string"},
          "configuration": {"type": "object", "description": "Non-secret settings: issuer/client_id/scopes/group_claim (oidc); metadata_url/entity_id/group_attribute/service_provider_entity_id/service_provider_acs_url (saml); client_id/base_url/api_base_url/scopes/allowed_orgs (github, issue #5166)."},
          "has_secret": {"type": "boolean"},
          "secret_fingerprint": {"type": "string"},
          "key_id": {"type": "string"},
          "shadowed_by_environment": {"type": "boolean", "description": "True when an env/file-registered provider already occupies this provider_config_id; the env config is authoritative and this row's sealed secret is never consulted for login."},
          "managed_by": {"type": "string", "enum": ["database", "environment"], "description": "\"database\" for a normal DB-backed row; \"environment\" for a row shadowed by (or synthesized from) an env/file-registered provider."},
          "created_at": {"type": "string", "format": "date-time"},
          "updated_at": {"type": "string", "format": "date-time"}
        }
      },
      "AdminProviderConfigWriteRequest": {
        "type": "object",
        "description": "Create/update request. Exactly one of the oidc, saml, or github field groups applies, selected by provider_kind. Secret fields (client_secret, sp_private_key, sp_certificate) are write-only and are never echoed back. The github group (issue #5166) reuses client_id/client_secret/scopes and adds base_url/api_base_url/allowed_orgs.",
        "required": ["provider_kind"],
        "properties": {
          "provider_kind": {"type": "string", "enum": ["oidc", "saml", "github"]},
          "provider_config_id": {"type": "string", "description": "Optional on create; when supplied, used verbatim (this is how a DB row is made to share an id with, and thereby be detected as shadowed by, an env-registered provider). Ignored on update."},
          "issuer": {"type": "string"},
          "client_id": {"type": "string"},
          "client_secret": {"type": "string", "description": "Write-only. Required on every create and update."},
          "scopes": {"type": "array", "items": {"type": "string"}},
          "group_claim": {"type": "string"},
          "redirect_url": {"type": "string"},
          "metadata_url": {"type": "string"},
          "metadata_xml": {"type": "string"},
          "entity_id": {"type": "string", "description": "Expected identity provider entity id, validated against the parsed IdP metadata."},
          "group_attribute": {"type": "string"},
          "service_provider_entity_id": {"type": "string", "description": "Eshu's own SP entity id advertised for this provider. Optional at create/test-connection time; required for the provider to resolve for login."},
          "service_provider_acs_url": {"type": "string", "description": "Eshu's own SP Assertion Consumer Service URL for this provider. Optional at create/test-connection time; required for the provider to resolve for login."},
          "sp_private_key": {"type": "string", "description": "Write-only PEM. Required on every create and update for saml."},
          "sp_certificate": {"type": "string", "description": "Write-only PEM. Required on every create and update for saml."},
          "base_url": {"type": "string", "description": "GitHub only (issue #5166). GitHub Enterprise Server host, e.g. https://github.example.com. Optional — defaults to https://github.com."},
          "api_base_url": {"type": "string", "description": "GitHub only (issue #5166). GitHub Enterprise Server REST API host, e.g. https://github.example.com/api/v3. Optional — defaults to https://api.github.com."},
          "allowed_orgs": {"type": "array", "items": {"type": "string"}, "description": "GitHub only (issue #5166). Required and non-empty: the GitHub organizations a user must have an active membership in to sign in. A GitHub OAuth App can authenticate any GitHub account, so this org allow-list is the connector's only tenant boundary."}
        }
      },
      "AdminProviderConfigWriteResult": {
        "type": "object",
        "properties": {
          "provider_config_id": {"type": "string"},
          "revision_id": {"type": "string"},
          "status": {"type": "string"},
          "changed": {"type": "boolean"}
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
      },
      "AdminAuditCountList": {
        "type": "array",
        "description": "Low-cardinality aggregate counts. Each entry is a name and an integer count; never an individual event identifier.",
        "items": {
          "type": "object",
          "properties": {
            "name": {"type": "string"},
            "count": {"type": "integer"}
          }
        }
      },
      "SkippedScopesReport": {
        "type": "object",
        "description": "Scopes a refinalize (POST /api/v0/admin/recover-generations, POST /api/v0/admin/refinalize) considered but did not re-enqueue, by reason. Always present in a performed recovery, with empty objects when nothing was skipped, so a partial graph rebuild is visible to the operator (#7116).",
        "required": ["total", "by_reason", "sample_scope_ids"],
        "properties": {
          "total": {"type": "integer", "description": "Scopes skipped across every reason."},
          "by_reason": {
            "type": "object",
            "description": "Exact skip count per reason. Reasons: no_recoverable_generation (failed scope with no non-superseded generation; only a fresh collection heals it), newest_generation_not_failed (failed scope whose newest non-superseded generation is pending and carries its own projector work), no_active_generation (scope neither active nor failed, for example a first generation still pending), unknown_scope (a named scope id with no ingestion_scopes row).",
            "additionalProperties": {"type": "integer"}
          },
          "sample_scope_ids": {
            "type": "object",
            "description": "Up to 10 skipped scope ids per reason, ascending, so an operator can find the scopes to look at without an unbounded response.",
            "additionalProperties": {"type": "array", "items": {"type": "string"}}
          }
        }
      },
      "DeltaActiveScopesReport": {
        "type": "object",
        "description": "Re-enqueued scopes whose re-projected generation is a delta (POST /api/v0/admin/recover-generations, POST /api/v0/admin/refinalize). A delta generation carries only the files that changed since its baseline, so re-projecting it onto an empty graph restores only those files: the graph for these scopes is incomplete until a full generation activates (#7797). Always present in a performed refinalize, with empty objects and an empty detail when no re-projected generation was a delta.",
        "required": ["total", "by_outcome", "sample_scope_ids", "detail"],
        "properties": {
          "total": {"type": "integer", "description": "Scopes re-projected through a delta generation across every outcome."},
          "by_outcome": {
            "type": "object",
            "description": "Exact count per outcome. Outcomes: reindex_requested (a git default-branch repository scope; the refinalize recorded a per-repository reindex watermark in the same transaction; a git ingester's scheduled selection forces a full re-parse when it next syncs the repository, at most ESHU_REPO_RECONCILE_MAX_PER_CYCLE per cycle and later while an in-flight or recently failed full holds the scope, and a webhook-only ingester forces it only when a webhook triggers that repository), reindex_unsupported (a git ref scope or another collector's scope; no reindex watermark can force it, so only that collector's next full generation repairs the graph).",
            "additionalProperties": {"type": "integer"}
          },
          "sample_scope_ids": {
            "type": "object",
            "description": "Up to 10 delta-active scope ids per outcome, ascending.",
            "additionalProperties": {"type": "array", "items": {"type": "string"}}
          },
          "detail": {"type": "string", "description": "Operator message stating that the graph is incomplete for these scopes until a full generation activates. Empty when total is 0."}
        }
      },
      "ReindexRequestsWrittenReport": {
        "type": "object",
        "description": "Per-repository reindex watermarks this refinalize wrote to repository_reindex_requests in its own transaction, one per delta-active git default-branch scope (#7797). The same rows POST /api/v0/admin/reindex with scope repository writes. A git ingester's scheduled selection honors a row when it next syncs the repository, at most ESHU_REPO_RECONCILE_MAX_PER_CYCLE per cycle and subject to the in-flight and retry-backoff throttle; a webhook-only ingester honors it only for a repository a webhook triggers. A row is satisfied when a full generation ingested at or after its requested_at activates.",
        "required": ["count", "scope_ids"],
        "properties": {
          "count": {"type": "integer", "description": "Exact number of scopes that got a reindex watermark."},
          "scope_ids": {"type": "array", "items": {"type": "string"}, "description": "Up to 10 of those scope ids, ascending. Never null."}
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
      "NotFound": {
        "description": "Resource not found",
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
      },
      "Unauthorized": {
        "description": "Authentication is required or the presented credential is not active",
        "content": {
          "application/json": {
            "schema": {"$ref": "#/components/schemas/ErrorResponse"}
          }
        }
      },
      "NotImplemented": {
        "description": "Capability is not available in the current runtime profile",
        "content": {
          "application/json": {
            "schema": {"$ref": "#/components/schemas/ErrorResponse"}
          }
        }
      }
    }
  }
}`
