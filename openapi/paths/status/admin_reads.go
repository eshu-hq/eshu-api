// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package status

// AdminReads documents the operator admin read endpoints whose path
// keys are unique to the D3 lane: the six query POSTs (work-items,
// decisions, dead-letters, input-invalid-facts, changed-since
// poisoned-links, replay-events). The relay-only mutations and the 3
// Gap 02 Admin GETs (delta-union completeness, reindex progress,
// tuning-report) share the core source file but keep their own path
// keys — the GETs are pinned and served by the trio lane
// (delta_union_completeness.go, reindex_progress.go,
// admin_tuning_report.go) — so this fragment ports only the six lane
// entries, never a key another lane serves. The entries reference
// only the shared responses (BadRequest, Forbidden, InternalError,
// ServiceUnavailable), so the lane adds no schemas.
//
// Provenance: each payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/status/admin.go
// at commit 7c60583996d9a3ccc46288f40a38f614a4f28ea8 (sha256
// 26c7669744ebb6b29a18bb9d95c7186e24785421afb8d733d9d9a3a246a78eb9,
// the recovery-lane pin — admin.go is byte-identical there and at the
// D2 wave pin). Core ships the lane entries and the relay-only routes
// in one file, so no whole-file byte-identical port can serve this
// lane only; this subset keeps the line-verbatim property instead,
// gated by scripts/check-parity.sh.
const AdminReads = `
    "/api/v0/admin/work-items/query": {
      "post": {
        "tags": ["admin"],
        "summary": "Query fact work items",
        "description": "A 503 with Retry-After means a transient condition that is safe to retry: the read hit a stale or timed-out PostgreSQL reader, or the request checkpoint step failed; a 503 without Retry-After means a configuration state: the admin store or the checkpoint source is not configured on this API instance. No 503 body carries backend error text. The read runs with no route deadline, so it never answers 504. A reader-verdict 503 names capability operator.work_items.list in its error envelope.",
        "responses": {
          "200": {"description": "Admin work-item query results"},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/admin/decisions/query": {
      "post": {
        "tags": ["admin"],
        "summary": "Query projection decisions",
        "description": "A 503 with Retry-After means a transient condition that is safe to retry: the read hit a stale or timed-out PostgreSQL reader, or the request checkpoint step failed; a 503 without Retry-After means a configuration state: the admin store or the checkpoint source is not configured on this API instance. No 503 body carries backend error text. The read runs with no route deadline, so it never answers 504. A reader-verdict 503 names capability operator.decisions.list, or operator.decision_evidence.list for the include_evidence read in its error envelope.",
        "responses": {
          "200": {"description": "Projection decision query results"},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/admin/dead-letters/query": {
      "post": {
        "tags": ["admin"],
        "summary": "Query dead-letter work items",
        "description": "Returns a bounded deterministic page of durable fact_work_items dead letters. Requires limit and timeout_ms, supports failure_class, domain, scope_id, collector_kind, and updated_at window filters, and returns truncated=true when more rows matched than the requested limit. Scoped tokens are restricted to their granted component scopes. A 503 with Retry-After means a transient condition that is safe to retry: the read hit a stale or timed-out PostgreSQL reader, or the request checkpoint step failed; a 503 without Retry-After means a configuration state: the admin store or the checkpoint source is not configured on this API instance. No 503 body carries backend error text. A reader-verdict 503 names capability operator.dead_letters.list in its error envelope.",
        "x-scoped-token-support": true,
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["limit", "timeout_ms"],
                "properties": {
                  "failure_class": {"type": "string"},
                  "domain": {"type": "string"},
                  "scope_id": {"type": "string"},
                  "collector_kind": {"type": "string"},
                  "updated_after": {"type": "string", "format": "date-time"},
                  "updated_before": {"type": "string", "format": "date-time"},
                  "limit": {"type": "integer", "minimum": 1, "maximum": 500},
                  "timeout_ms": {"type": "integer", "minimum": 1, "maximum": 30000}
                }
              }
            }
          }
        },
        "responses": {
          "200": {"description": "Bounded dead-letter page with schema_version, limit, count, truncated, and items"},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "504": {"description": "Dead-letter query exceeded its timeout_ms budget while the read ran (fixed message dead-letter query timed out). If the budget expires during reader setup instead, the status depends on the reader topology: a single PostgreSQL reader answers the retryable 503 reader verdict; with reader fleet members, a budget that expires while the reader waits for a member slot or between member attempts answers this 504, and one that expires inside a member attempt answers the retryable 503. Keep timeout_ms well above reader setup latency."},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/admin/input-invalid-facts/query": {
      "post": {
        "tags": ["admin"],
        "summary": "Query durable input_invalid quarantine facts",
        "description": "Returns a bounded deterministic page of durable reducer_input_invalid_facts rows (issue #4630): facts the reducer quarantined during typed-payload decode because a required field was missing or null. Requires scope_id, generation_id, limit, and timeout_ms, supports optional domain and fact_kind filters, and returns truncated=true when more rows matched than the requested limit. Scoped tokens are restricted to their granted component scopes. A 503 with Retry-After means a transient condition that is safe to retry: the read hit a stale or timed-out PostgreSQL reader, or the request checkpoint step failed; a 503 without Retry-After means a configuration state: the admin store or the checkpoint source is not configured on this API instance. No 503 body carries backend error text. A reader-verdict 503 names capability operator.reducer_input_invalid_facts.list in its error envelope.",
        "x-scoped-token-support": true,
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["scope_id", "generation_id", "limit", "timeout_ms"],
                "properties": {
                  "scope_id": {"type": "string"},
                  "generation_id": {"type": "string"},
                  "domain": {"type": "string"},
                  "fact_kind": {"type": "string"},
                  "limit": {"type": "integer", "minimum": 1, "maximum": 500},
                  "timeout_ms": {"type": "integer", "minimum": 1, "maximum": 30000}
                }
              }
            }
          }
        },
        "responses": {
          "200": {"description": "Bounded input_invalid quarantine page with schema_version, limit, count, truncated, and items"},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "504": {"description": "Input-invalid-facts query exceeded its timeout_ms budget while the read ran (fixed message input-invalid-facts query timed out). If the budget expires during reader setup instead, the status depends on the reader topology: a single PostgreSQL reader answers the retryable 503 reader verdict; with reader fleet members, a budget that expires while the reader waits for a member slot or between member attempts answers this 504, and one that expires inside a member attempt answers the retryable 503. Keep timeout_ms well above reader setup latency."},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/admin/changed-since/poisoned-links/query": {
      "post": {
        "tags": ["admin"],
        "summary": "Query poisoned and retrying changed-since links",
        "description": "Returns a bounded deterministic page of durable changed_since_scope_cursor rows that are poisoned (the link hit its counting-failure limit, #7127 ruling 8.10) or retrying (a counted failure is pending on the scope's head activation). The changed-since link writer (go/internal/storage/postgres/freshness/links) is a runner over its own ledger, not a fact_work_items queue domain, so this state never appears in the dead-letter or reducer_input_invalid_facts surfaces (#7290). Requires limit and timeout_ms; supports optional status (poisoned|retrying), scope_id, and a forward keyset cursor (the last scope_id of a prior page). Returns truncated=true and next_cursor when more rows matched than the requested limit. Scoped tokens are restricted to their granted component scopes. A 503 with Retry-After means a transient condition that is safe to retry: the read hit a stale or timed-out PostgreSQL reader, or the request checkpoint step failed; a 503 without Retry-After means a configuration state: the admin store or the checkpoint source is not configured on this API instance. No 503 body carries backend error text. A reader-verdict 503 names capability operator.changed_since_poisoned_links.list in its error envelope.",
        "x-scoped-token-support": true,
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["limit", "timeout_ms"],
                "properties": {
                  "status": {"type": "string", "enum": ["poisoned", "retrying"]},
                  "scope_id": {"type": "string"},
                  "cursor": {"type": "string", "description": "The last scope_id of a prior page's items; returns rows with a greater scope_id."},
                  "limit": {"type": "integer", "minimum": 1, "maximum": 500},
                  "timeout_ms": {"type": "integer", "minimum": 1, "maximum": 30000}
                }
              }
            }
          }
        },
        "responses": {
          "200": {"description": "Bounded changed-since poisoned/retrying link page with schema_version, limit, count, truncated, next_cursor, and items"},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "504": {"description": "Changed-since poisoned-links query exceeded its timeout_ms budget while the read ran (fixed message changed-since poisoned-links query timed out). If the budget expires during reader setup instead, the status depends on the reader topology: a single PostgreSQL reader answers the retryable 503 reader verdict; with reader fleet members, a budget that expires while the reader waits for a member slot or between member attempts answers this 504, and one that expires inside a member attempt answers the retryable 503. Keep timeout_ms well above reader setup latency."},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/admin/replay-events/query": {
      "post": {
        "tags": ["admin"],
        "summary": "Query replay events",
        "description": "A 503 with Retry-After means a transient condition that is safe to retry: the read hit a stale or timed-out PostgreSQL reader, or the request checkpoint step failed; a 503 without Retry-After means a configuration state: the admin store or the checkpoint source is not configured on this API instance. No 503 body carries backend error text. The read runs with no route deadline, so it never answers 504. A reader-verdict 503 names capability operator.replay_events.list in its error envelope.",
        "responses": {
          "200": {"description": "Replay-event query results"},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
`
