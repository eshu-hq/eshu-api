// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package infrastructure holds the ported OpenAPI path fragments for
// the Infra family: the ecosystem, infra-relationship, and
// relationship-catalog reads (Routes) and the infra resource
// count/inventory aggregates (ResourceAggregate — D4 Native-next
// wave).
//
// Each file holds exactly one exported string constant. This package
// imports nothing; openapi/spec.go imports this package, never the other
// way around. The fragment files are byte-identical ports of the pinned
// core sources (see scripts/check-parity.sh); keep them in lockstep
// with the handlers and contracts/d4-native-next.md.
package infrastructure
