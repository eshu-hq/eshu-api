// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package status holds the ported OpenAPI path fragments for the
// admin-retirement family: POST /api/v0/admin/repository-retirements
// (RepositoryRetirement); and the terminal-failure family: the
// acknowledgement POST and its two query POSTs under
// /api/v0/admin/collector-failures/...
// (CollectorTerminalFailureAcknowledgement).
//
// Each file holds exactly one exported string constant. This package
// imports nothing; openapi/spec.go imports this package, never the other
// way around. Each fragment file is a byte-identical port of the pinned
// core source (see scripts/check-parity.sh); keep fragments in lockstep
// with their handlers and contracts/retirement-admission.md and
// contracts/terminal-failure-admission.md.
package status
