// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package freshness holds the ported OpenAPI path fragments for the
// freshness family: the freshness causality read at
// /api/v0/status/freshness-causality (Causality, terminating natively
// since the D6 lane).
//
// The file holds exactly one exported string constant. This package
// imports nothing; openapi/spec.go imports this package, never the
// other way around. The fragment file is a byte-identical port of the
// pinned core source (see scripts/check-parity.sh); keep it in
// lockstep with the handler and contracts/d6-status-supplychain.md.
package freshness
