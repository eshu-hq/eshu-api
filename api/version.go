// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

// CoreReadContractsVersion names the core-owned read-contract set this
// server consumes (Refs eshu-hq/eshu#8872): the GraphQuery and ContentStore
// ports plus the Postgres ReadStore surface. It must move in lockstep with
// querycontract.CoreReadContractsVersion in eshu; the vendored pin lives
// in contracts/core-read-contracts.md and the source pin in
// docs/provenance.md. A breaking change to any named port fails the
// version test until this constant moves in the same diff.
const CoreReadContractsVersion = "v0"

// SuppressionAdmissionContractVersion names the versioned admission port
// for the single bounded in-API mutation (the operator suppression
// upsert). It must move in lockstep with the in-tree
// SuppressionAdmissionContractVersion; the vendored pin lives in
// contracts/suppression-admission.md. Route dispatch over this port lands
// with core dispatch, not in the skeleton.
const SuppressionAdmissionContractVersion = "v0"
