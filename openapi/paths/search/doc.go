// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package search holds the ported OpenAPI path fragment for the ask
// family: the bounded Tier-1 ask POST under /api/v0/ask (Ask, D2 Leaf B
// Native-now wave).
//
// The file holds exactly one exported string constant. This package
// imports nothing; openapi/spec.go imports this package, never the other
// way around. The fragment file is a byte-identical port of the pinned
// core source (see scripts/check-parity.sh); keep it in lockstep with
// the handler and contracts/leafb-native-now.md.
package search
