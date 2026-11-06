// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package chain holds the ported OpenAPI path fragment for the
// suppression family: the operator suppression upsert POST under
// /api/v0/supply-chain/impact/suppressions (SuppressionMutation, the
// single bounded SuppressionAdmission survivor row).
//
// The file holds exactly one exported string constant. This package
// imports nothing; openapi/spec.go imports this package, never the other
// way around. The fragment file is a line-verbatim subset port of the
// pinned core source (the payload lines are byte-identical; only the
// const wrapper is dest-owned to export the symbol — see
// scripts/check-parity.sh); keep it in lockstep with the handler and
// contracts/suppression-admission.md.
package chain
