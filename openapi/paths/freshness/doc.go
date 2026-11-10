// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package freshness holds the ported OpenAPI path fragments for the
// freshness family: the freshness causality read at
// /api/v0/status/freshness-causality (Causality, terminating natively
// since the D6 lane), and the Wave-1 drilldown reads (ChangedSince,
// Generations, ServiceChangedSince, terminating natively since the
// Wave-1 Freshness lane, Refs eshu-hq/eshu#9669).
//
// Each fragment file holds exactly one exported string constant. This
// package imports nothing; openapi/spec.go imports this package, never
// the other way around. Each fragment file is a byte-identical port of
// its pinned core source (see scripts/check-parity.sh); keep them in
// lockstep with the handlers and contracts/d6-status-supplychain.md
// (causality) or contracts/freshness-wave.md (Wave-1 drilldowns).
package freshness
