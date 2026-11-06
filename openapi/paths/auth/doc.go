// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package auth holds the ported OpenAPI path fragments for the
// sign-in-policy family: the public pre-auth GET and the admin GET/PATCH
// under /api/v0/auth/... (SignInPolicy); for the admin-identity
// mutation family: the invitation revoke, role-assignment revoke, and
// mapping delete routes whose path keys are unique to the mutation
// surface (AdminMutations); for the admin-identity reads family: the
// tenant-scoped admin reads whose path keys carry the family GETs
// (plus the grant and create POSTs that share those keys, per the core
// OpenAPI convention) (AdminReads); and for the admin provider-config
// mutation family: the revert, enable, disable, and test-connection
// routes whose path keys are unique to the mutation surface
// (AdminProviderConfigMutations).
//
// Each file holds exactly one exported string constant. This package
// imports nothing; openapi/spec.go imports this package, never the other
// way around. The fragment file is a byte-identical port of the pinned
// core source (see scripts/check-parity.sh); keep it in lockstep with the
// handlers and contracts/sign-in-admission.md.
package auth
