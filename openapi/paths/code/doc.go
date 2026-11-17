// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package code holds the ported OpenAPI path fragments for the
// code-search and static-analysis routes: the top-level code routes
// (Routes), the call/reference graph (Graph), data-flow tracing
// (Flow), code ownership (Owners), code quality signals (Quality),
// security-relevant call paths (Security), route-to-caller tracing
// (RouteToCaller), and symbol search (Symbols). Dead-code detection
// (Scan, CrossRepo, Investigation) lives in the dead subpackage.
//
// Each file holds exactly one exported string constant. This package
// imports the sibling schema package for the shared evidence-boundary
// helpers; openapi/spec.go imports this package, never the other way
// around. Every payload line is verbatim from its pinned core file
// under go/internal/query/openapi/paths/code/ (see
// scripts/check-parity.sh); keep it in lockstep with the handlers
// and contracts/code-*-termination.md.
package code
