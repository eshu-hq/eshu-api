// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package iac holds the ported OpenAPI path fragments for the IaC-drift
// and replatforming routes: Routes (the five management POSTs),
// Resources (the candidate-page GET), TerraformConfigStateDrift,
// Replatforming, ReplatformingOwnership, ReplatformingSelectors, and
// ReplatformingRollups — each an exported JSON string constant that
// openapi.Spec concatenates into the published document.
//
// The replatforming_*.go fragments live here, not in a separate
// package, because every replatforming route is served by the same
// IaC handler family. This package holds route-fragment data only, no
// handler logic. It MUST NOT import the parent openapi package:
// openapi/spec.go imports every paths/<family> leaf, so the reverse
// import is a cycle. Every payload line is verbatim from its pinned
// core file under go/internal/query/openapi/paths/iac/ (see
// scripts/check-parity.sh).
package iac
