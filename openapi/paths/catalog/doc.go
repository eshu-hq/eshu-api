// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package catalog holds the ported OpenAPI path fragments for the
// capabilities family: the capability-matrix probe surface GET under
// /api/v0/capabilities (Capabilities, D2 Leaf B Native-now wave); and
// for the Playbooks family: the query-playbook list and resolve routes
// (Playbooks, D4 Native-next wave).
//
// Each file holds exactly one exported string constant. This package
// imports nothing; openapi/spec.go imports this package, never the other
// way around. The fragment files are byte-identical ports of the pinned
// core sources (see scripts/check-parity.sh); keep them in lockstep
// with the handlers and contracts/leafb-native-now.md (D2) and
// contracts/d4-native-next.md (D4).
package catalog
