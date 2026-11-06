// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package status holds the ported OpenAPI path fragments for the
// admin-retirement family: POST /api/v0/admin/repository-retirements
// (RepositoryRetirement).
//
// Each file holds exactly one exported string constant. This package
// imports nothing; openapi/spec.go imports this package, never the other
// way around. The fragment file is a byte-identical port of the pinned
// core source (see scripts/check-parity.sh); keep it in lockstep with
// the handler and contracts/retirement-admission.md.
package status
