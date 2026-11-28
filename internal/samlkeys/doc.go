// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package samlkeys owns custody of SAML SP signing-key material for
// the G7 SAML flow (Refs eshu-hq/eshu-api#451, N6 prerequisite for
// the P5 SAML sign-in flows in
// docs/decisions/0002-eshu-api-owns-api-compute.md).
//
// Scope honesty (binding): this package owns custody only. No HTTP
// route terminates on it in this issue — api/d9saml.go and
// api/saml_terminate.go stay key-free by their check-parity and
// lane-hygiene gates — and nothing here is wired into cmd/ or any
// server constructor, so existing relay deploys that lack SAML
// secrets keep starting exactly as before. G7 wires it.
//
// Three invariants hold across every file here:
//
//  1. Secret references only. Key material enters pods through
//     mounted secret files; configuration names the file PATH, never
//     the PEM value. Loaders fail closed on empty paths, unreadable
//     files, unparseable material, key/cert mismatch, or an
//     out-of-validity certificate, and errors carry paths only —
//     never key or certificate bytes.
//  2. Memory only, no raw export. Material is parsed once with the
//     standard library and retained as a crypto.Signer plus its
//     *x509.Certificate. Raw PEM bytes are cleared after parsing,
//     nothing is written to disk or any store, and String/GoString
//     render metadata (provider, version, fingerprint, expiry) only.
//  3. Rotation without downtime. The store publishes immutable
//     per-provider snapshot sets {Current, Previous} behind an
//     atomic pointer: G7 captures one snapshot per AuthnRequest and
//     signs outside any lock, so in-flight signing finishes on its
//     captured snapshot after a reload. Metadata publishes both
//     certificates during the overlap and drops the previous one
//     explicitly once in-flight requests expire.
package samlkeys
