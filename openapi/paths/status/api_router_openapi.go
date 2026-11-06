// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package status

// OpenAPISpec documents the served-spec route itself:
// GET /api/v0/openapi.json (getOpenAPISpec, D2 Leaf B Native-now
// wave). Core ships the whole status surface in one file, so no
// whole-file byte-identical port can serve this entry only; this
// subset keeps the line-verbatim property instead, gated by
// scripts/check-parity.sh.
//
// Provenance: each payload line below is verbatim from the pinned core
// file go/internal/query/openapi/paths/status/routes.go at the wave pin
// (see docs/provenance.md). Keep it in lockstep with the handler.
const OpenAPISpec = `
    "/api/v0/openapi.json": {
      "get": {
        "tags": ["health"],
        "summary": "OpenAPI specification",
        "description": "Returns the OpenAPI 3.0 specification for this API.",
        "operationId": "getOpenAPISpec",
        "responses": {
          "200": {
            "description": "OpenAPI specification",
            "content": {
              "application/json": {
                "schema": {"type": "object"}
              }
            }
          }
        }
      }
    },
`
