// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package status

// AdminD7 documents the admin-four mutation endpoints whose path keys
// are unique to the D7 tier-C clone lane: backfill, dead-letter,
// reindex, and skip. The C-c2 four (replay, reopen, refinalize,
// recover-generations) share the core source file but keep their own
// path keys on the recovery lane, and the query POSTs stay on the D3
// read lane, so this fragment ports only the four lane entries — never
// a key another lane serves. None of the four entries references a
// shared schema (only the versioned response $refs every lane already
// carries), so the lane ports no schemas alongside the entries.
//
// Provenance: each payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/status/admin.go
// at commit 19de7346d11a184bf88d8f307b5d7a9ced0a4657 (sha256
// 26c7669744ebb6b29a18bb9d95c7186e24785421afb8d733d9d9a3a246a78eb9).
// Core ships the lane entries and the other lanes' routes in one file,
// so no whole-file byte-identical port can serve this lane only; this
// subset keeps the line-verbatim property instead, gated by
// scripts/check-parity.sh.
const AdminD7 = `
    "/api/v0/admin/reindex": {
      "post": {
        "tags": ["admin"],
        "summary": "Request a fleet-wide or per-repository reindex",
        "description": "Records a reindex watermark, stamped by Postgres and never moved backward. With scope workspace (the default) it records the fleet watermark, returned as requested_at: on each sync cycle every git ingester shard forces a full re-parse (reconcile reason reindex_requested) of each repository it owns whose newest activated full generation was ingested before requested_at. With scope repository it resolves each repositories selector to exactly one git default-branch repository scope and records a watermark for each, returned per repository; the owning shard forces a full re-parse (reconcile reason repository_reindex_requested) and syncs requested repositories first. A repository-scoped request is all or nothing: any selector that matches no repository, matches several, or is not a git default-branch scope fails the request with 400 naming every such selector, and nothing is recorded. Both share the reconciliation sweep's per-cycle budget (ESHU_REPO_RECONCILE_MAX_PER_CYCLE) and its in-flight and retry-backoff throttle. A request is satisfied repository by repository as those full generations activate; it is never claimed and has no completion status. Webhook-only ingesters reach only the repositories they are triggered for, and filesystem source mode does not read either watermark. Send the request only after every git ingester runs the target binary: the watermark records when a full generation was ingested, not which parser produced it. Unknown body fields are rejected. A 503 with Retry-After means a transient condition that is safe to retry, and nothing was recorded: the repository catalog lookup hit a stale or timed-out PostgreSQL reader, or the request checkpoint step failed; a 503 without Retry-After means a configuration state: reindex requests, repository-scoped reindex requests, or the checkpoint source are not configured on this API instance. No 503 body carries backend error text. The lookup runs with no route deadline, so the route never answers 504. A reader-verdict 503 names capability operator.repository_catalog.lookup in its error envelope.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "additionalProperties": false,
                "properties": {
                  "ingester": {"type": "string", "enum": ["repository"], "default": "repository", "description": "Only the git repository ingesters honor reindex requests."},
                  "scope": {"type": "string", "enum": ["workspace", "repository"], "default": "workspace", "description": "workspace reindexes every repository the git ingesters own; repository reindexes only the repositories listed in repositories."},
                  "repositories": {"type": "array", "minItems": 1, "maxItems": 100, "items": {"type": "string", "minLength": 1}, "description": "Repository selectors (repository ID, scope ID, name, slug, or path) for scope repository. Required with scope repository and rejected with scope workspace."},
                  "force": {"type": "boolean", "default": true, "description": "Must be true or omitted: a reindex always forces a full re-parse, and false is rejected with 400."}
                }
              }
            }
          }
        },
        "responses": {
          "202": {
            "description": "Reindex watermark recorded.",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "required": ["status", "ingester", "scope", "force", "detail"],
                  "properties": {
                    "status": {"type": "string", "enum": ["accepted"]},
                    "ingester": {"type": "string", "enum": ["repository"]},
                    "scope": {"type": "string", "enum": ["workspace", "repository"]},
                    "force": {"type": "boolean", "enum": [true]},
                    "requested_at": {"type": "string", "format": "date-time", "description": "scope workspace only: the stored fleet reindex watermark in UTC. A request never lowers it."},
                    "repositories": {
                      "type": "array",
                      "description": "scope repository only: one entry per distinct repository scope, sorted by scope_id.",
                      "items": {
                        "type": "object",
                        "required": ["repository_id", "scope_id", "requested_at"],
                        "properties": {
                          "repository_id": {"type": "string"},
                          "scope_id": {"type": "string"},
                          "requested_at": {"type": "string", "format": "date-time", "description": "The stored per-repository reindex watermark in UTC. A request never lowers it."}
                        }
                      }
                    },
                    "detail": {"type": "string"}
                  },
                  "oneOf": [
                    {"required": ["requested_at"], "properties": {"scope": {"enum": ["workspace"]}}},
                    {"required": ["repositories"], "properties": {"scope": {"enum": ["repository"]}}}
                  ]
                }
              }
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/admin/dead-letter": {
      "post": {
        "tags": ["admin"],
        "summary": "Dead-letter work items",
        "description": "A 503 with Retry-After means the request checkpoint step failed, a transient condition that is safe to retry; a 503 without Retry-After means a configuration state: the admin store or the checkpoint source is not configured on this API instance. No 503 body carries backend error text.",
        "responses": {
          "200": {"description": "Dead-letter request results"},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/admin/skip": {
      "post": {
        "tags": ["admin"],
        "summary": "Skip repository work items",
        "description": "Dead-letters at most 100 pending, retrying, or failed work items for a repository or scope. Claimed, running, succeeded, superseded, and already dead-lettered items are left unchanged. The response count includes only rows transitioned by this call. The repository_id accepts a scope id or a source key; a selector matching more than one scope fails closed with 409, naming the matched scopes. A 503 with Retry-After means the request checkpoint step failed, a transient condition that is safe to retry; a 503 without Retry-After means a configuration state: the admin store or the checkpoint source is not configured on this API instance. No 503 body carries backend error text.",
        "responses": {
          "200": {"description": "Skip request results"},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "409": {"$ref": "#/components/responses/Conflict"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/admin/backfill": {
      "post": {
        "tags": ["admin"],
        "summary": "Request a backfill",
        "description": "A 503 with Retry-After means the request checkpoint step failed, a transient condition that is safe to retry; a 503 without Retry-After means a configuration state: the admin store or the checkpoint source is not configured on this API instance. No 503 body carries backend error text.",
        "responses": {
          "200": {"description": "Backfill request results"},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
`
