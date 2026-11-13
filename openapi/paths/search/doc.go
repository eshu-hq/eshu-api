// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package search holds the ported OpenAPI path fragments for the ask
// family, the Wave-2 Content point reads, and the W4 Content search:
// the bounded Tier-1 ask POST under /api/v0/ask (Ask, D2 Leaf B
// Native-now wave), the three Content point-read POSTs
// (ContentPointReads, Wave-2 Content lane, Refs eshu-hq/eshu#9669),
// and the four Content search POSTs (ContentSearch, W4 Content search
// lane, Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#111).
//
// Each fragment file holds exactly one exported string constant. This
// package imports nothing; openapi/spec.go imports this package, never
// the other way around. The Ask fragment is a byte-identical port of
// its pinned core source; the ContentPointReads and ContentSearch
// entries are line-verbatim subsets of their pinned core source (see
// scripts/check-parity.sh); keep them in lockstep with the handlers
// and contracts/leafb-native-now.md (Ask),
// contracts/content-point-reads-wave.md (ContentPointReads), or
// contracts/content-search-termination.md (ContentSearch).
package search
