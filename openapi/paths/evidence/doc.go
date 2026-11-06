// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package evidence holds the ported OpenAPI path fragments for the
// Documentation, Evidence, and InvestigationWorkflows families: the
// documentation facts/findings GETs (Documentation,
// DocumentationFindingAggregate), the evidence drilldown GETs and
// citation POST (EvidenceRoutes, a line-verbatim subset of the pinned
// core file), the evidence bundle GET (Bundle), the deployable-unit
// packet GET (DeployableUnitPacket, a line-verbatim subset carrying
// only the deployable-unit entry), and the investigation-workflow list
// and resolve routes (InvestigationWorkflows — D4 Native-next wave).
//
// Each file holds exactly one exported string constant. This package
// imports nothing; openapi/spec.go imports this package, never the other
// way around. The whole-file fragment files are byte-identical ports of
// the pinned core sources and the subsets keep the line-verbatim
// property (see scripts/check-parity.sh); keep them in lockstep with
// the handlers and contracts/d4-native-next.md.
package evidence
