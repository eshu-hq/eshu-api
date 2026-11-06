// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package cli holds the ported OpenAPI path fragments for the CLI
// support family: the pure-function POSTs under /api/v0/answer-quality,
// /api/v0/investigation, /api/v0/report, and /api/v0/service
// (Score, Render, Capture, Validate, Compose — D2 Leaf B Native-now
// wave).
//
// Each file holds exactly one exported string constant. This package
// imports nothing; openapi/spec.go imports this package, never the other
// way around. The fragment files are byte-identical ports of the pinned
// core sources (see scripts/check-parity.sh); keep them in lockstep
// with the handlers and contracts/leafb-native-now.md.
package cli
