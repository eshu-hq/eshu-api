// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package cloud holds the ported OpenAPI path fragment for the Infra
// family: the cloud resources GET under /api/v0/cloud (Routes — D4
// Native-next wave).
//
// The file holds exactly one exported string constant. This package
// imports nothing; openapi/spec.go imports this package, never the other
// way around. The fragment file is a byte-identical port of the pinned
// core source (see scripts/check-parity.sh); keep it in lockstep with
// the handlers and contracts/d4-native-next.md.
package cloud
