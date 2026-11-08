// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package status

// AdminTuningReport documents the static shared-projection tuning
// report GET (GET /api/v0/admin/shared-projection/tuning-report).
// The report has no reader — tuning is managed internally by the Go
// projector pipeline — so the contract is the exact served shape and
// its fixed values (SharedProjectionTuningReportContractVersion,
// pinned status not_applicable and the frozen detail text), served
// from the pinned constants by the core handler. The entry shares the
// core source file with the D3 AdminReads entries but keeps its own
// path key: it ports only this lane entry, never a key the relay
// still serves. The entry references only the shared responses it
// names (none beyond the 200 description), so the lane adds no
// schemas.
//
// Provenance: each payload line below is verbatim from the pinned
// core file go/internal/query/openapi/paths/status/admin.go at commit
// 4a587431fdf748c0a231b60708131fbaa1f41064 (sha256
// 26c7669744ebb6b29a18bb9d95c7186e24785421afb8d733d9d9a3a246a78eb9,
// the Gap 02 merge — admin.go is byte-identical there and at the
// recovery-lane pin). Core ships the lane entry and the relay-only
// routes in one file, so no whole-file byte-identical port can serve
// this lane only; this subset keeps the line-verbatim property
// instead, gated by scripts/check-parity.sh.
const AdminTuningReport = `
    "/api/v0/admin/shared-projection/tuning-report": {
      "get": {
        "tags": ["admin"],
        "summary": "Get shared projection tuning guidance",
        "responses": {
          "200": {"description": "Shared projection tuning report"}
        }
      }
    },
`
