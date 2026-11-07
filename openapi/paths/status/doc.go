// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package status holds the ported OpenAPI path fragments for the
// admin-retirement family: POST /api/v0/admin/repository-retirements
// (RepositoryRetirement); the terminal-failure family: the
// acknowledgement POST and its two query POSTs under
// /api/v0/admin/collector-failures/...
// (CollectorTerminalFailureAcknowledgement); and the admin-recovery
// family: the replay, reopen, refinalize, and recover-generations
// POSTs whose path keys are unique to the tier C-c2 lane
// (AdminRecovery, a line-verbatim subset of the pinned core file);
// and the operator admin read family: the six query POSTs whose path
// keys are unique to the D3 lane (AdminReads, a line-verbatim subset
// of the same pinned core file); and the admin-four family: the
// backfill, dead-letter, reindex, and skip POSTs whose path keys are
// unique to the D7 tier-C clone lane (AdminD7, a line-verbatim subset
// of the same pinned core file at the D7 merge).
//
// Each file holds exactly one exported string constant. This package
// imports nothing; openapi/spec.go imports this package, never the other
// way around. Each fragment file is a byte-identical port of the pinned
// core source (see scripts/check-parity.sh); keep fragments in lockstep
// with their handlers and contracts/retirement-admission.md,
// contracts/terminal-failure-admission.md,
// contracts/recovery-admission.md, and contracts/d7-admin-four.md.
package status
