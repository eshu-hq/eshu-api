// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package repository

import (
	"github.com/eshu-hq/eshu-api/openapi/schema"
)

// RepositoryStory documents the repository story GET. The entry
// interpolates the shared schema.EvidenceBoundaries helper, so it
// cannot ride the RepositoryRoutes whole-file subset: this file
// vendors the entry entry-shaped with the helper, keeping the
// line-verbatim property (gated by scripts/check-parity.sh).
const RepositoryStory = `
    "/api/v0/repositories/{repo_id}/story": {
      "get": {
        "tags": ["repositories"],
        "summary": "Get repository story",
        "description": "Returns a structured repository story with deployment, support, and bounded content-store coverage overviews. Missing coverage remains explicit and does not trigger whole-graph traversal. Scoped tokens receive the same shape; a repository outside the caller's grant 404s like sibling repository routes.",
        "operationId": "getRepositoryStory",
        "x-scoped-token-support": true,
        "parameters": [
          {"$ref": "#/components/parameters/RepoId"},
          {"name": "evidence_detail", "in": "query", "required": false, "schema": {"type": "string", "enum": ["full", "handles"], "default": "full"}, "description": "How the story row lists are shaped. full (the HTTP default when absent) returns every list in full. handles projects relationship_overview.relationships, deployment_evidence.artifacts, deployment_artifacts, delivery_paths, delivery_workflows, target_documentation.findings, documentation_overview.target_documentation.related_facts, target_support.evidence, support_overview.target_support.ambiguous_evidence, and framework_summary.frameworks to identity rows, drops the repeated relationship partition lists and the evidence_index regrouping with a pointer, keeps every count, narrative, limitation, and drilldown, and names each reduced family with its total in truth.omissions. Each total is the rows the story held for that family before the cut; the target documentation and target support reads are capped at 10 rows, so their totals are lower bounds past the cap. The MCP get_repo_story tool defaults to handles. Unknown values return 400 invalid_argument."}
        ],
        "responses": {
          "403": {"$ref": "#/components/responses/Forbidden"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"},
          "504": {"$ref": "#/components/responses/GatewayTimeout"},
          "200": {
            "description": "Repository narrative",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "repository": {"$ref": "#/components/schemas/RepositoryRef"},
                    "subject": {"type": "object"},
                    "story": {"type": "string"},
                    "story_sections": {"type": "array", "items": {"type": "object"}},
                    "semantic_overview": {"type": "object"},
                    "ci_cd_evidence": {"type": "object", "description": "Repository-scoped static workflow, live run, and artifact evidence. A file read that returns a sentinel row past the 5,000-file limit adds static_workflow_artifacts.candidate_pool_status=unknown_at_limit and missing_evidence class static_workflow_coverage_unknown. A repository with exactly 5,000 files is a complete scan and adds neither. Zero observed workflows on a truncated scan means state=unknown; positive counts describe the observed page. Uncapped serialization is preserved."},
                    "deployment_overview": {"type": "object"},
                    "gitops_overview": {"type": "object"},
                    "documentation_overview": {"type": "object"},
                    "support_overview": {"type": "object"},
                    "coverage_summary": {"type": "object", "description": "Bounded content-store coverage, sharing the stats coverage contract. last_error, when present, is one fixed string from the same closed set as the stats coverage.last_error, never backend error text."},
                    "limitations": {
                      "type": "array",
                      "description": "Explicit reasons the story is partial, e.g. infrastructure_read_degraded when the auxiliary infrastructure graph read failed, relationships_read_degraded when the outgoing dependencies read failed, infrastructure_truncated when a healthy infrastructure read landed past its bound, story_rows_truncated when the workload/platform/language narrative rows landed past their bound, or repository_semantic_read_truncated_at_5000 when the bounded semantic entity or file read found a row past its 5000-row cap, or deployment_artifacts_read_degraded when a content or graph read of the deployment-artifact stage failed and the artifacts hold the loads that succeeded and lack the artifacts of the load that failed, or deployment_artifacts_candidates_budget_reached means the request budget of 12,000 config-artifact candidates beyond the per-repository floor of 50 ran out, so at least one related repository had candidates inside its 5,000-file window that were not read and the config rows are a lower bound, or deployment_artifacts_related_file_list_truncated_at_5000 means at least one related repository has more than 5,000 files, so its candidates past the window are unknown; always present so the envelope shape is stable.",
                      "items": {"type": "string"}
                    },
                    "truncated": {"type": "boolean", "description": "True when the workload/platform/language narrative rows landed past repositoryStoryStringRowLimit and were capped, OR the auxiliary infrastructure read landed past its own bound, OR the bounded semantic entity or file read found a row past its 5000-row cap, OR a deployment-artifact candidate bound bit (deployment_artifacts_candidates_budget_reached or deployment_artifacts_related_file_list_truncated_at_5000 in limitations); always present so the envelope shape is stable."},
                    "drilldowns": {"type": "object"},
                    "answer_metadata": {"type": "object", "description": "Normalized additive answer metadata with schema_version, evidence_handles, missing_evidence, limitations, truncated, coverage, partial_reasons, and recommended_next_calls."},
                    "evidence_boundaries": ` + schema.EvidenceBoundaries + `
                  }
                }
              }
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "404": {"$ref": "#/components/responses/NotFound"},
          "500": {"$ref": "#/components/responses/InternalError"}
        }
      }
    },
`
