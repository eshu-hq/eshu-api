// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package repository holds the ported OpenAPI path fragments for the
// PackageRegistry and Repositories families: the package-registry
// correlations, versions, dependencies, and packages reads
// (PackageRegistry, PackageRegistryAggregate) and the catalog,
// repository list, language, content, context, and tree reads
// (RepositoryRoutes, a line-verbatim subset of the pinned core file —
// the mux-health entry stays dest-side and the story entry stays out
// while its route relays), plus the branches, stats/coverage, and freshness reads
// (Branches, StatsAndCoverage, Freshness — D4 Native-next wave).
//
// Each file holds exactly one exported string constant. This package
// imports nothing; openapi/spec.go imports this package, never the other
// way around. The whole-file fragment files are byte-identical ports of
// the pinned core sources and the subset keeps the line-verbatim
// property (see scripts/check-parity.sh); keep them in lockstep with
// the handlers and contracts/d4-native-next.md.
package repository
