// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"net/http"
	"strings"

	"github.com/eshu-hq/eshu-api/openapi/paths/auth"
	"github.com/eshu-hq/eshu-api/openapi/paths/status"
)

// SpecVersion names the native-spec contract: v0 serves exactly the
// families whose OpenAPI fragments have landed in this repository
// (sign-in-policy first, retirement second, terminal-failure third,
// admin-identity mutations fourth, admin-identity reads fifth). It
// advances as family lanes port their
// fragments; the served family set is pinned by the Spec tests.
const SpecVersion = "v0"

// specPrefix opens the served document: the protocol version, the
// destination identity, the served prefix mounts, and the tags the landed
// fragments use. Family path entries concatenate after the open "paths"
// object; the shared components block closes it (see Spec).
const specPrefix = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Eshu API",
    "description": "Native-dispatch satellite surface. This document serves exactly the families whose OpenAPI fragments have landed in this repository (sign-in-policy, retirement, terminal-failure, admin-identity mutations, admin-identity reads); every other family is still relayed from core until its lane ports its fragment.",
    "version": "` + SpecVersion + `",
    "contact": {
      "name": "Eshu"
    }
  },
  "servers": [
    {
      "url": "/api/v0",
      "description": "API v0 prefix"
    },
    {
      "url": "/api/v1",
      "description": "API v1 prefix"
    }
  ],
  "tags": [
    {"name": "auth", "description": "Browser session and CSRF-safe dashboard authentication routes"},
    {"name": "admin", "description": "Operator administration routes"}
  ],
  "paths": {
`

// familyPaths renders the landed path fragments and closes the open
// "paths" object. Every fragment file ends its last entry with a
// separator comma for the next fragment in concatenation order (the core
// convention), so only the final fragment in landing order has its
// trailing comma trimmed before the close. A fragment that stops ending
// in a comma breaks the served JSON and the Spec tests fail closed.
func familyPaths() string {
	return strings.TrimSpace(auth.SignInPolicy) + "\n" +
		strings.TrimSpace(status.RepositoryRetirement) + "\n" +
		strings.TrimSpace(status.CollectorTerminalFailureAcknowledgement) + "\n" +
		strings.TrimSpace(auth.AdminMutations) + "\n" +
		strings.TrimSuffix(strings.TrimSpace(auth.AdminReads), ",") + "\n"
}

// Spec returns the dest-assembled OpenAPI 3.0 document for the landed
// families: the spec prefix, every ported family path fragment in landing
// order, then the minimal shared components block. The concatenation
// order is the compatibility surface the destination owns.
func Spec() string {
	return specPrefix +
		familyPaths() +
		"  },\n" +
		components +
		"\n"
}

// ServeSpec writes the assembled specification as JSON. Headers match the
// core mount at the provenance pin (go/internal/query/openapi/spec.go):
// JSON with charset, open CORS.
func ServeSpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(Spec()))
}
