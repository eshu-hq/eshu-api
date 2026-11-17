// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package dead holds the ported OpenAPI path fragments for the
// dead-code routes: a per-repository scan (Scan), a cross-repository
// classification (CrossRepo), and a single-repository investigation
// (Investigation). Each is its own file, following the
// one-constant-per-file shape the rest of openapi/paths/ uses.
// openapi.Spec concatenates each of them.
//
// This package MUST NOT import openapi or any other
// openapi/paths/<leaf> package. Every payload line is verbatim from
// its pinned core file under go/internal/query/openapi/paths/code/dead/
// (see scripts/check-parity.sh).
package dead
