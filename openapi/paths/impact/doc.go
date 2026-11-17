// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package impact holds the ported OpenAPI path fragments for the
// change- and blast-radius-impact routes: the exported Routes,
// Contract, Rest, and Exposure, which openapi.Spec concatenates, plus
// the unexported deploymentConfigInfluence, which only this package
// reads.
//
// deploymentConfigInfluence is folded into Routes by a source-level
// `+` in routes.go rather than joined again in openapi/spec.go; do
// not add a second, separate concatenation of it there or the route
// doubles in the published document. Routes and
// deploymentConfigInfluence both splice K8sResourceLimits from this
// package's k8s_resource_limits.go; Routes also splices
// ImageRegistryTruthLimits and the shared schema helpers.
//
// This package holds route-fragment data only, no handler logic. It
// MUST NOT import the parent openapi package: openapi/spec.go imports
// every paths/<family> leaf, so the reverse import is a cycle. Every
// payload line is verbatim from its pinned core file under
// go/internal/query/openapi/paths/impact/ (see
// scripts/check-parity.sh).
package impact
