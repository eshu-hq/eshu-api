// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package evidence

// DeployableUnitPacket documents the deployable-unit investigation
// packet GET. The supply-chain, drift, and services packets sharing
// the pinned core file stay on the generic relay until their lanes
// port them, so no whole-file port can serve this lane only.
//
// Provenance: every payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/evidence/investigations.go
// at commit 17feac86e0 (sha256
// df0e51563c025a4c6571a04704ab9abce4b9abcfb3ba304d624cb0,
// the D2 wave pin). This subset keeps the line-verbatim property
// instead of a whole-file port (gated by scripts/check-parity.sh).
const DeployableUnitPacket = `
    "/api/v0/investigations/deployable-unit/packet": {
      "get": {
        "tags": ["investigations"],
        "summary": "Export deployable-unit investigation packet",
        "description": "Returns an investigation_evidence_packet.v2 artifact for deployable-unit admission truth, using reducer-owned admission-decision readback. A 503 with Retry-After means a transient condition that is safe to retry: the admission decision read hit a stale or timed-out PostgreSQL reader; a 503 without Retry-After means a configuration state: the Postgres admission decision read model is not configured on this API instance. No 5xx body carries backend error text. A reader-verdict 503 names capability admission_decisions.list in its error envelope.",
        "operationId": "getDeployableUnitPacket",
        "x-scoped-token-support": true,
        "parameters": [
          {"name": "scope_id", "in": "query", "required": true, "schema": {"type": "string"}, "description": "Ingestion scope id that bounds the admission decision read."},
          {"name": "generation_id", "in": "query", "required": true, "schema": {"type": "string"}, "description": "Scope generation id that bounds the admission decision read."},
          {"name": "repository_id", "in": "query", "required": false, "schema": {"type": "string"}, "description": "Optional repository anchor used to narrow deployable-unit decisions."},
          {"name": "repo_id", "in": "query", "required": false, "schema": {"type": "string"}, "description": "Alias for repository_id."},
          {"name": "max_source_facts", "in": "query", "required": false, "schema": {"type": "integer", "minimum": 1}, "description": "Optional lower cap for the packet source_facts layer."}
        ],
        "responses": {
          "200": {"description": "Investigation evidence packet", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/InvestigationEvidencePacket"}}}},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "501": {"$ref": "#/components/responses/NotImplemented"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"},
          "500": {"$ref": "#/components/responses/InternalError"}
        }
      }
    },
`
