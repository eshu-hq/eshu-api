// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package search holds the ported OpenAPI path fragments for the ask
// family and the Wave-2 Content point reads: the bounded Tier-1 ask
// POST under /api/v0/ask (Ask, D2 Leaf B Native-now wave) and the
// three Content point-read POSTs (ContentPointReads, Wave-2 Content
// lane, Refs eshu-hq/eshu#9669).
//
// Each fragment file holds exactly one exported string constant. This
// package imports nothing; openapi/spec.go imports this package, never
// the other way around. The Ask fragment is a byte-identical port of
// its pinned core source; the ContentPointReads entries are a
// line-verbatim subset of their pinned core source (see
// scripts/check-parity.sh); keep them in lockstep with the handlers
// and contracts/leafb-native-now.md (Ask) or
// contracts/content-point-reads-wave.md (ContentPointReads).
package search
