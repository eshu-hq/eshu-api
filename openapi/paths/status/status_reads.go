// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package status

// StatusReads documents the operator status read endpoints whose path
// keys are unique to the D6 lane: the pipeline, ingesters, collectors,
// and ingesters-by-name GETs in both their legacy (/api/v0/...) and
// /api/v0/status/... spellings (satellite-design Decision 5(c) on
// eshu-hq/eshu#9669). The still-relayed Status aspects share the core
// source file but keep their own path keys on the generic relay with
// no admission pin, so this fragment ports only the seven snapshot
// lane entries — never a key the relay still serves (governance via
// the GovernanceAudit Summary, answer-narration via the
// NarrationPosture override, hosted-readiness via Neo4j counts, and
// the mux-level openapi.json self entry). The phase-2 extra-port legs
// (index-status and status/index via StatusExtraReads, operations via
// Operations) carry their own fragments behind StatusExtraReadAdmission
// v0. The entries reference only the shared responses (BadRequest,
// Forbidden, NotFound, InternalError, ServiceUnavailable) and the
// ActiveWorkSource, CoordinatorSource, and TerraformStateSource
// schemas, so the lane adds no other schemas.
//
// Provenance: each payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/status/routes.go
// at commit 6b66e6b23efe226970b4571c955aed1dc42f987a (sha256
// 24a57b35c675cb33ad5011343a0aeb68d49e9d05d7645f397de2d88e8e5afa61).
// Core ships the lane entries and the relay-only routes in one file, so
// no whole-file byte-identical port can serve this lane only; this
// subset keeps the line-verbatim property instead, gated by
// scripts/check-parity.sh.
const StatusReads = `
    "/api/v0/status/pipeline": {
      "get": {
        "tags": ["status"],
        "summary": "Get pipeline status",
        "description": "Returns the full pipeline status report. A 503 with Retry-After means a transient condition that is safe to retry: the status snapshot read hit a stale or timed-out PostgreSQL reader; a 503 without Retry-After means a configuration state: the status reader is not configured on this API instance. No 5xx body carries backend error text. A reader-verdict 503 error envelope names capability operations.pipeline_status.",
        "operationId": "getPipelineStatus",
        "responses": {
          "200": {
            "description": "Pipeline status",
            "content": {
              "application/json": {
                "schema": {"type": "object", "properties": {"active_work_source": {"$ref": "#/components/schemas/ActiveWorkSource"}, "coordinator_source": {"$ref": "#/components/schemas/CoordinatorSource"}, "terraform_state_source": {"$ref": "#/components/schemas/TerraformStateSource"}}}
              }
            }
          },
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/status/ingesters": {
      "get": {
        "tags": ["status"],
        "summary": "List ingesters",
        "description": "Returns known ingesters with basic health info.",
        "operationId": "listIngesters",
        "x-scoped-token-support": true,
        "responses": {
          "403": {"$ref": "#/components/responses/Forbidden"},
          "200": {
            "description": "List of ingesters",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "active_work_source": {"$ref": "#/components/schemas/ActiveWorkSource"},
                    "coordinator_source": {"$ref": "#/components/schemas/CoordinatorSource"},
                    "ingesters": {"type": "array", "items": {"type": "object"}},
                    "count": {"type": "integer"}
                  }
                }
              }
            }
          },
          "500": {"$ref": "#/components/responses/InternalError"}
        }
      }
    },
    "/api/v0/status/collectors": {
      "get": {
        "tags": ["status"],
        "summary": "List collectors",
        "description": "Returns collector runtime status classified by workflow coordinator registration, direct status evidence, and persisted source or reducer fact evidence. An unresolved terminal fetch failure marks the instance health degraded and adds last_failure_class, last_failure_operation, last_failure_terminal, and last_failure_at. An operator can acknowledge a failure (POST /api/v0/admin/collector-failures/acknowledge): the instance then reads promotion_state partial with an acknowledged_failure object instead of degraded, and its health reads at least partial (never healthy): a stale vulnerability source or a degraded failure on the same instance keeps the higher health. Scoped callers receive only the per-kind health aggregate; last_failure_* fields are withheld from them. A 503 with Retry-After means a transient condition that is safe to retry: the status snapshot read hit a stale or timed-out PostgreSQL reader; a 503 without Retry-After means a configuration state: the status reader is not configured on this API instance. No 5xx body carries backend error text. A reader-verdict 503 error envelope names capability operations.collector_status.",
        "operationId": "listCollectors",
        "x-scoped-token-support": true,
        "responses": {
          "403": {"$ref": "#/components/responses/Forbidden"},
          "200": {
            "description": "List of collector runtimes",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "version": {"type": "string"},
                    "collectors": {
                      "type": "array",
                      "items": {
                        "type": "object",
                        "properties": {
                          "instance_id": {"type": "string"},
                          "collector_kind": {"type": "string"},
                          "mode": {"type": "string"},
                          "runtime_mode": {"type": "string"},
                          "status_category": {
                            "type": "string",
                            "enum": ["coordinator_managed", "direct_mode", "profile_gated", "disabled", "unregistered"]
                          },
                          "coordinator_registered": {"type": "boolean"},
                          "enabled": {"type": "boolean"},
                          "claims_enabled": {"type": "boolean"},
                          "evidence_sources": {"type": "array", "items": {"type": "string"}},
                          "source_systems": {"type": "array", "items": {"type": "string"}},
                          "observation_count": {"type": "integer"},
                          "health": {"type": "string"},
                          "detail": {"type": "string"},
                          "last_failure_class": {"type": "string", "enum": ["analyzer_failed", "archived", "attempt_budget_exhausted", "auth_denied", "collect_failure", "commit_failed", "commit_failure", "cpu_limit_exceeded", "deleted", "expired_lease_budget_exhausted", "extension_defined", "fact_limit_exceeded", "file_limit_exceeded", "identity_mismatch", "input_limit_exceeded", "invalid_claim", "invalid_result", "launch_failure", "memory_limit_exceeded", "non_retryable", "not_found", "permission_denied", "permission_hidden", "rate_limited", "registry_auth_denied", "registry_canceled", "registry_metadata_too_large", "registry_not_found", "registry_rate_limited", "registry_retryable_failure", "registry_terminal_failure", "retryable", "source_state_record", "source_unavailable", "stale_fence", "status_record", "target_unavailable", "terminal", "timeout", "unsupported_permission", "unsupported_target", "waiting_on_git_generation", "unknown"], "description": "Bounded class of the newest unresolved terminal fetch failure; present only with last_failure_terminal. Each collector kind reports the common classes, the extension host classes, and its own; any other persisted value reads unknown, and a class an extension defines reads extension_defined, whether the kind is not first-party or the instance is a component-activated extension carrying a first-party kind."},
                          "last_failure_operation": {"type": "string", "description": "Bounded snake_case operation token of that failure, or unknown."},
                          "last_failure_terminal": {"type": "boolean"},
                          "last_failure_at": {"type": "string", "format": "date-time", "description": "When that failure became terminal; present only with last_failure_terminal."},
                          "acknowledged_failure": {
                            "type": "object",
                            "description": "An operator-acknowledged terminal failure (POST /api/v0/admin/collector-failures/acknowledge). Health reads at least partial, never healthy (a stale vulnerability source or a degraded failure on the same instance keeps the higher health); last_failure_* stays absent unless another scope has an unacknowledged failure. Bounded tokens and times only; the operator's reason is never returned here. Omitted when none.",
                            "properties": {
                              "failure_class": {"type": "string", "enum": ["analyzer_failed", "archived", "attempt_budget_exhausted", "auth_denied", "collect_failure", "commit_failed", "commit_failure", "cpu_limit_exceeded", "deleted", "expired_lease_budget_exhausted", "extension_defined", "fact_limit_exceeded", "file_limit_exceeded", "identity_mismatch", "input_limit_exceeded", "invalid_claim", "invalid_result", "launch_failure", "memory_limit_exceeded", "non_retryable", "not_found", "permission_denied", "permission_hidden", "rate_limited", "registry_auth_denied", "registry_canceled", "registry_metadata_too_large", "registry_not_found", "registry_rate_limited", "registry_retryable_failure", "registry_terminal_failure", "retryable", "source_state_record", "source_unavailable", "stale_fence", "status_record", "target_unavailable", "terminal", "timeout", "unsupported_permission", "unsupported_target", "waiting_on_git_generation", "unknown"]},
                              "failure_operation": {"type": "string"},
                              "failed_at": {"type": "string", "format": "date-time"},
                              "acknowledged_at": {"type": "string", "format": "date-time"}
                            }
                          }
                        }
                      }
                    },
                    "count": {"type": "integer"},
                    "classification_basis": {"type": "string"},
                    "updated_at": {"type": "string", "format": "date-time", "nullable": true}
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
    "/api/v0/status/ingesters/{ingester}": {
      "get": {
        "tags": ["status"],
        "summary": "Get ingester status",
        "description": "Returns detailed status for a specific ingester. A 503 with Retry-After means a transient condition that is safe to retry: the status snapshot read hit a stale or timed-out PostgreSQL reader; a 503 without Retry-After means a configuration state: the status reader is not configured on this API instance. No 5xx body carries backend error text. A reader-verdict 503 error envelope names capability operations.ingester_status.",
        "operationId": "getIngesterStatus",
        "x-scoped-token-support": true,
        "parameters": [
          {
            "name": "ingester",
            "in": "path",
            "required": true,
            "schema": {"type": "string"},
            "description": "Ingester name"
          }
        ],
        "responses": {
          "403": {"$ref": "#/components/responses/Forbidden"},
          "200": {
            "description": "Ingester status",
            "content": {
              "application/json": {
                "schema": {"type": "object", "properties": {"active_work_source": {"$ref": "#/components/schemas/ActiveWorkSource"}, "coordinator_source": {"$ref": "#/components/schemas/CoordinatorSource"}}}
              }
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "404": {"$ref": "#/components/responses/NotFound"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/ingesters": {
      "get": {
        "tags": ["status"],
        "summary": "List ingesters",
        "description": "Legacy compatibility alias for the Go-owned ingester status list.",
        "operationId": "listIngestersLegacy",
        "responses": {
          "200": {
            "description": "List of ingesters",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "active_work_source": {"$ref": "#/components/schemas/ActiveWorkSource"},
                    "coordinator_source": {"$ref": "#/components/schemas/CoordinatorSource"},
                    "ingesters": {"type": "array", "items": {"type": "object"}},
                    "count": {"type": "integer"}
                  }
                }
              }
            }
          },
          "500": {"$ref": "#/components/responses/InternalError"}
        }
      }
    },
    "/api/v0/collectors": {
      "get": {
        "tags": ["status"],
        "summary": "List collectors",
        "description": "Legacy compatibility alias for collector runtime status. A 503 with Retry-After means a transient condition that is safe to retry: the status snapshot read hit a stale or timed-out PostgreSQL reader; a 503 without Retry-After means a configuration state: the status reader is not configured on this API instance. No 5xx body carries backend error text. A reader-verdict 503 error envelope names capability operations.collector_status.",
        "operationId": "listCollectorsLegacy",
        "responses": {
          "200": {
            "description": "List of collector runtimes",
            "content": {
              "application/json": {
                "schema": {"type": "object"}
              }
            }
          },
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/ingesters/{ingester}": {
      "get": {
        "tags": ["status"],
        "summary": "Get ingester status",
        "description": "Legacy compatibility alias for the Go-owned ingester status detail route. A 503 with Retry-After means a transient condition that is safe to retry: the status snapshot read hit a stale or timed-out PostgreSQL reader; a 503 without Retry-After means a configuration state: the status reader is not configured on this API instance. No 5xx body carries backend error text. A reader-verdict 503 error envelope names capability operations.ingester_status.",
        "operationId": "getIngesterStatusLegacy",
        "parameters": [
          {
            "name": "ingester",
            "in": "path",
            "required": true,
            "schema": {"type": "string"},
            "description": "Ingester name"
          }
        ],
        "responses": {
          "200": {
            "description": "Ingester status",
            "content": {
              "application/json": {
                "schema": {"type": "object", "properties": {"active_work_source": {"$ref": "#/components/schemas/ActiveWorkSource"}, "coordinator_source": {"$ref": "#/components/schemas/CoordinatorSource"}}}
              }
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "404": {"$ref": "#/components/responses/NotFound"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
`
