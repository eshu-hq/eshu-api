// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package search

// ContentPointReads is the OpenAPI path fragment documenting the
// Wave-2 Content point-read routes: `/api/v0/content/files/read`,
// `/api/v0/content/files/lines`, and `/api/v0/content/entities/read`
// (Refs eshu-hq/eshu#9669, D13 tracker eshu-hq/eshu#10491). openapi.Spec
// concatenates it into the published document; keep it in lockstep with
// the handlers and contracts/content-point-reads-wave.md.
//
// The three entries are a line-verbatim subset of the pinned core file
// go/internal/query/openapi/paths/search/content.go at the Wave-2 merge
// (see scripts/check-parity.sh): that file carries the lane entries
// mid-list with separator commas — not a paths-closing entry shaped
// like the Compare fragment — so the subset concatenates without
// re-shaping and no disclosed absence is recorded. The core file's
// search entries stay core-side with the still-relaying search legs.
const ContentPointReads = `
    "/api/v0/content/files/read": {
      "post": {
        "tags": ["content"],
        "summary": "Read file content",
        "description": "Reads full file content from the content store.",
        "operationId": "readFile",
        "x-scoped-token-support": true,
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["repo_id", "relative_path"],
                "properties": {
                  "repo_id": {"type": "string"},
                  "relative_path": {"type": "string"}
                }
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "File content",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/FileContent"}
              }
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "404": {"$ref": "#/components/responses/NotFound"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"},
          "500": {"$ref": "#/components/responses/InternalError"}
        }
      }
    },
    "/api/v0/content/files/lines": {
      "post": {
        "tags": ["content"],
        "summary": "Read file lines",
        "description": "Reads a line range from a file.",
        "operationId": "readFileLines",
        "x-scoped-token-support": true,
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["repo_id", "relative_path", "start_line", "end_line"],
                "properties": {
                  "repo_id": {"type": "string"},
                  "relative_path": {"type": "string"},
                  "start_line": {"type": "integer"},
                  "end_line": {"type": "integer"}
                }
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "File lines",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/FileContent"}
              }
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "404": {"$ref": "#/components/responses/NotFound"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"},
          "500": {"$ref": "#/components/responses/InternalError"}
        }
      }
    },
    "/api/v0/content/entities/read": {
      "post": {
        "tags": ["content"],
        "summary": "Read entity content",
        "description": "Reads entity source code from the content store.",
        "operationId": "readEntity",
        "x-scoped-token-support": true,
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["entity_id"],
                "properties": {
                  "entity_id": {"type": "string"}
                }
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Entity content",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/EntityContent"}
              }
            }
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "404": {"$ref": "#/components/responses/NotFound"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"},
          "500": {"$ref": "#/components/responses/InternalError"}
        }
      }
    },
`
