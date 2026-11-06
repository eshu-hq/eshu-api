// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package status

// AdminRecovery documents the admin recovery mutation endpoints whose
// path keys are unique to the tier C-c2 lane: replay, reopen, refinalize,
// and recover-generations. The relay-only four (backfill, dead-letter,
// reindex, skip) share the core source file but keep their own path keys
// and stay on the generic relay with no admission pin, so this fragment
// ports only the four lane entries — never a key the relay still serves.
// The refinalize and recover-generations entries reference three shared
// schemas (SkippedScopesReport, DeltaActiveScopesReport,
// ReindexRequestsWrittenReport) that no relay-only route references, so
// the lane ports exactly those schemas alongside the entries.
//
// Provenance: each payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/status/admin.go
// at commit 7c60583996d9a3ccc46288f40a38f614a4f28ea8 (sha256
// 26c7669744ebb6b29a18bb9d95c7186e24785421afb8d733d9d9a3a246a78eb9).
// Core ships the lane entries and the relay-only routes in one file, so no
// whole-file byte-identical port can serve this lane only; this subset
// keeps the line-verbatim property instead, gated by
// scripts/check-parity.sh.
const AdminRecovery = `
    "/api/v0/admin/refinalize": {
      "post": {
        "tags": ["admin"],
        "summary": "Refinalize scopes",
        "description": "A 503 with Retry-After means the request checkpoint step failed, a transient condition that is safe to retry; a 503 without Retry-After means a configuration state: the recovery handler or the checkpoint source is not configured on this API instance. No 503 body carries backend error text.",
        "responses": {
          "200": {
            "description": "Refinalize request accepted. Alongside status, enqueued, scope_ids, and the five dedup counters it carries skipped_scopes, the scopes named in the request that were considered but not re-enqueued, by reason, delta_active_scopes, the re-enqueued scopes whose generation is a delta and whose graph stays incomplete until a full generation activates, and reindex_requests_written, the per-repository reindex watermarks written for them in the same transaction.",
            "content": {"application/json": {"schema": {"type": "object", "properties": {"skipped_scopes": {"$ref": "#/components/schemas/SkippedScopesReport"}, "delta_active_scopes": {"$ref": "#/components/schemas/DeltaActiveScopesReport"}, "reindex_requests_written": {"$ref": "#/components/schemas/ReindexRequestsWrittenReport"}}}}}
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "409": {"description": "Recovery could not acquire the reducer claim fence within its bounded retry window; response contains blocked/claim_fence_busy. Retry after queue contention subsides."},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/admin/recover-generations": {
      "post": {
        "tags": ["admin"],
        "summary": "Recover wedged generations",
        "description": "Operator escape hatch for generations that wedge active without advancing past canonical-nodes-committed, and the disaster-recovery entry point for rebuilding the graph from preserved Postgres facts. Durably re-enqueues projector work through the same Go work queue refinalize uses (re-driving reduce -> readiness -> projection over existing facts, no re-clone) and records the action in the admin_replay_requests ledger. Send either scope_ids or all_scopes, never both. all_scopes re-enqueues every recoverable scope, which is what a graph rebuild after a Postgres restore or a graph-backend swap needs: each active scope through its active generation, and each failed scope with no active generation through its newest failed generation (a named failed scope in scope_ids is recovered the same way). Scopes it cannot re-enqueue are reported in skipped_scopes, by reason, so a partial rebuild is visible. Requires an explicit reason and idempotency_key and an admin (all-scopes) token. Duplicate delivery of the same idempotency_key returns the prior outcome (duplicate=true) instead of re-enqueuing; a key reused across the two modes conflicts. In the same transaction it clears the dedup state for exactly those generations - succeeded reducer work items, completed shared projection intents, graph projection phase rows, active relationship generations, and shared projection acceptance rows - because all five outlive a graph wipe and would otherwise tell the pipeline the work is already done, leaving the rebuild stuck at source-local structure. A 503 with Retry-After means the request checkpoint step failed, a transient condition that is safe to retry; a 503 without Retry-After means a configuration state: the recovery handler, the admin store, or the checkpoint source is not configured on this API instance. No 503 body carries backend error text.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["reason", "idempotency_key"],
                "properties": {
                  "scope_ids": {"type": "array", "items": {"type": "string"}, "description": "Scopes whose wedged generations should be re-driven: an active scope through its active generation, a failed scope through its newest failed generation. Required unless all_scopes is true."},
                  "all_scopes": {"type": "boolean", "default": false, "description": "Re-enqueue every recoverable scope (active scopes and failed scopes with a failed generation), for rebuilding the graph from preserved facts when no scope list is available. Cannot be combined with scope_ids."},
                  "reason": {"type": "string", "description": "Why the recovery is safe."},
                  "idempotency_key": {"type": "string", "description": "Makes the recovery safe under retries and concurrent delivery."}
                },
                "oneOf": [
                  {"required": ["scope_ids"], "properties": {"scope_ids": {"minItems": 1}, "all_scopes": {"enum": [false]}}},
                  {"required": ["all_scopes"], "properties": {"all_scopes": {"enum": [true]}, "scope_ids": {"maxItems": 0}}}
                ]
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "A recovery this call performed, or the prior outcome replayed for an idempotency_key that already completed. The two shapes differ: only the first reports the dedup state cleared.",
            "content": {
              "application/json": {
                "schema": {
                  "oneOf": [
                    {
                      "type": "object",
                      "description": "Recovery performed by this call. Alongside status, enqueued, and scope_ids it reports the dedup state cleared so the re-projection rebuilds the whole graph rather than only its source-local layer. After a graph wipe all five counters should be non-zero; five zeros mean the rebuild will restore source-local structure and nothing else.",
                      "required": ["status", "enqueued", "scope_ids", "reducer_work_deleted", "shared_intents_reopened", "readiness_phases_cleared", "generations_retired", "shared_projection_acceptance_cleared", "skipped_scopes", "delta_active_scopes", "reindex_requests_written", "idempotency_key", "duplicate"],
                      "properties": {
                        "status": {"type": "string", "enum": ["recovered"]},
                        "enqueued": {"type": "integer", "description": "Scope generations re-enqueued for projection."},
                        "scope_ids": {"type": "array", "items": {"type": "string"}, "description": "Scopes actually re-enqueued."},
                        "reducer_work_deleted": {"type": "integer", "description": "Succeeded reducer work items removed so the re-projection's enqueue is not deduplicated away."},
                        "shared_intents_reopened": {"type": "integer", "description": "Shared projection intents whose completed_at was cleared so the partition workers drain them again."},
                        "readiness_phases_cleared": {"type": "integer", "description": "Graph projection phase rows removed, because they outlive a graph wipe and would otherwise assert canonical nodes are committed for an empty graph."},
                        "generations_retired": {"type": "integer", "description": "Active relationship generations superseded so the re-projection never consumes the prior wave's resolved rows as current truth."},
                        "shared_projection_acceptance_cleared": {"type": "integer", "description": "Shared projection acceptance rows deleted so the repo_dependency lane cannot project edges for a refinalized generation from its pre-wipe watermark; the re-projection's intent commits re-advance acceptance."},
                        "skipped_scopes": {"$ref": "#/components/schemas/SkippedScopesReport"},
                        "delta_active_scopes": {"$ref": "#/components/schemas/DeltaActiveScopesReport"},
                        "reindex_requests_written": {"$ref": "#/components/schemas/ReindexRequestsWrittenReport"},
                        "idempotency_key": {"type": "string"},
                        "duplicate": {"type": "boolean", "enum": [false]}
                      }
                    },
                    {
                      "type": "object",
                      "description": "Idempotent replay: this key already completed, and nothing was re-enqueued. The five dedup counters, skipped_scopes, delta_active_scopes, and reindex_requests_written are absent because the admin_replay_requests ledger does not persist them, so a retry issued after the original response was lost cannot report what that recovery cleared. Read the counters from the original response, or from the projector queue and shared-intent backlog directly.",
                      "required": ["status", "enqueued", "scope_ids", "idempotency_key", "duplicate"],
                      "properties": {
                        "status": {"type": "string", "enum": ["recovered"]},
                        "enqueued": {"type": "integer", "description": "Scope generations the original recovery re-enqueued."},
                        "scope_ids": {"type": "array", "items": {"type": "string"}, "description": "Scopes the original recovery re-enqueued."},
                        "idempotency_key": {"type": "string"},
                        "duplicate": {"type": "boolean", "enum": [true]}
                      }
                    }
                  ]
                }
              }
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"description": "Recovery requires an admin (all-scopes) token"},
          "409": {"description": "Idempotency key already in progress or reused with different scope_ids, or recovery could not acquire the reducer claim fence within its bounded retry window. On claim-fence exhaustion, the response contains blocked/claim_fence_busy, the idempotency key remains in_progress, and retry requires a fresh key after contention subsides."},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/admin/replay": {
      "post": {
        "tags": ["admin"],
        "summary": "Replay failed work items",
        "description": "Safely replays terminal work items. Requires an explicit reason and idempotency_key, an admin (all-scopes) token, and refuses unsafe failure classes (input_invalid, unsafe_payload, and the manual-review dead-letter triage classes projection_bug and resource_exhausted) unless force is set, whether they arrive as the failure_class selector or as explicit work_item_ids. A 200 with replayed_count 0 means nothing matched. Duplicate delivery of the same idempotency_key returns the prior outcome instead of replaying again. A 503 with Retry-After means the request checkpoint step failed, a transient condition that is safe to retry; a 503 without Retry-After means a configuration state: the admin store or the checkpoint source is not configured on this API instance. No 503 body carries backend error text.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["reason", "idempotency_key"],
                "properties": {
                  "work_item_ids": {"type": "array", "items": {"type": "string"}},
                  "scope_id": {"type": "string"},
                  "stage": {"type": "string"},
                  "failure_class": {"type": "string"},
                  "operator_note": {"type": "string"},
                  "reason": {"type": "string", "description": "Why the replay is safe."},
                  "idempotency_key": {"type": "string", "description": "Makes the replay safe under retries and concurrent delivery."},
                  "force": {"type": "boolean", "description": "Replay unsafe failure classes after addressing the cause."},
                  "limit": {"type": "integer"}
                }
              }
            }
          }
        },
        "responses": {
          "200": {"description": "Replay request results (duplicate=true when an idempotent prior outcome is returned)"},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"description": "Replay requires an admin (all-scopes) token"},
          "409": {"description": "Idempotency key already in progress or reused with different parameters"},
          "422": {
            "description": "Refused without force: the failure_class selector, or at least one explicit work_item_ids row, is in an unsafe or manual-review failure class. Also refused, even with force: at least one explicit work_item_ids row is a projector row whose scope generation is superseded (failure_class projector_replay_generation_superseded, #7130). A request naming such ids is refused whole, nothing is replayed, the idempotency_key is not consumed, and refused_work_items lists only the offending ids.",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "required": ["status", "reason", "detail"],
                  "properties": {
                    "status": {"type": "string", "enum": ["refused"]},
                    "reason": {"type": "string"},
                    "detail": {"type": "string"},
                    "failure_class": {"type": "string", "description": "Present when the failure_class selector itself was refused."},
                    "refused_work_items": {
                      "type": "array",
                      "description": "Present when explicit work_item_ids were refused; sorted by work_item_id.",
                      "items": {
                        "type": "object",
                        "required": ["work_item_id", "failure_class", "reason"],
                        "properties": {
                          "work_item_id": {"type": "string"},
                          "failure_class": {"type": "string"},
                          "generation_id": {"type": "string", "description": "Present for a superseded-generation refusal: the superseded generation the row belongs to."},
                          "reason": {"type": "string", "description": "Operator guidance for the class."}
                        }
                      }
                    }
                  }
                }
              }
            }
          },
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/admin/reopen": {
      "post": {
        "tags": ["admin"],
        "summary": "Reopen completed work items",
        "description": "Reopens completed reducer or shared-projection work for one domain and scope, the admin surface for the #7285 hand-SQL repair. Requires an explicit domain, scope_id, reason and idempotency_key, and an admin (all-scopes) token. Only repo_dependency (one completed intent row per acceptance unit, at the accepted source run), workload_materialization and submodule_pin (succeeded reducer rows) reopen; every other domain is refused. The store resolves the scope's active generation at run time and locks with SKIP LOCKED under a lock_timeout. A 200 with reopened_total_count 0 means nothing matched. Duplicate delivery of the same idempotency_key returns the prior outcome (duplicate=true, totals without the reducer/intent split) instead of reopening again. A scope_id matching more than one scope fails closed with 409, naming the matched scopes. A 503 with Retry-After means the request checkpoint step failed, a transient condition that is safe to retry; a 503 without Retry-After means a configuration state: the admin store or the checkpoint source is not configured on this API instance. No 503 body carries backend error text.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["domain", "scope_id", "reason", "idempotency_key"],
                "properties": {
                  "domain": {"type": "string", "enum": ["repo_dependency", "workload_materialization", "submodule_pin"], "description": "Which completed work to reopen."},
                  "scope_id": {"type": "string", "description": "Ingestion scope id or source key."},
                  "reason": {"type": "string", "description": "Why the reopen is safe."},
                  "idempotency_key": {"type": "string", "description": "Makes the reopen safe under retries and concurrent delivery."},
                  "limit": {"type": "integer", "description": "Maximum rows to reopen. Defaults to 1000; larger values are clamped to 1000."}
                }
              }
            }
          }
        },
        "responses": {
          "200": {"description": "Reopen request results (duplicate=true when an idempotent prior outcome is returned)"},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "404": {"$ref": "#/components/responses/NotFound"},
          "409": {"$ref": "#/components/responses/Conflict"},
          "422": {
            "description": "Refused before the idempotency claim, so the key is not consumed: the domain is not one of the three reopenable domains, or the scope has no active generation to reopen work for. Nothing is reopened either way. On a resolve race after the claim the same refused body is returned and the key stays in progress.",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "required": ["status", "reason", "detail"],
                  "properties": {
                    "status": {"type": "string", "enum": ["refused"]},
                    "domain": {"type": "string", "description": "Present when the domain itself was refused."},
                    "reason": {"type": "string"},
                    "detail": {"type": "string"}
                  }
                }
              }
            }
          },
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
`
