// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Setup native-dispatch routes (tier C-c1): the four pre-auth
// first-run wizard routes. They are registered explicitly OUTSIDE
// RequireAuth, ahead of the generic /api/ proxy, because a fresh
// deployment has no session, bearer, or prior credential to present.
// Route shapes are confirmed against the in-tree SetupHandler.Mount
// and go/cmd/api/CUTOVER_OWNERSHIP.md (Setup family, wireAPI) at the
// provenance pin: the state check plus the claim, admin, and MFA
// wizard steps under /api/v0/auth/....
const (
	setupStateRoute = "GET /api/v0/auth/setup-state"
	setupClaimRoute = "POST /api/v0/auth/setup/claim"
	setupAdminRoute = "POST /api/v0/auth/setup/admin"
	setupMFARoute   = "POST /api/v0/auth/setup/mfa"
)

// setupRoutes lists every native-dispatch route of the Setup family
// in registration order.
var setupRoutes = []string{
	setupStateRoute,
	setupClaimRoute,
	setupAdminRoute,
	setupMFARoute,
}

// setupWriteThrough is the tier C-c1 native dispatch for the setup
// wizard: each request crosses an explicit public per-family route
// instead of the generic byte-relay, with no dest-side gate. The
// shape is HTTP write-through — method, path, query, headers, and
// body forward to the same core route, and core bytes relay back
// unmodified — because every trust decision stays core-side: core's
// AuthMiddleware bypass (publicHTTPPaths) plus the per-request
// SetupStore re-checks (VerifyBootstrapCredential proof and the
// permanent SetupNeeded seal that answers 410 once any identity
// exists) still run on every forwarded request.
//
// Dest adds no validation and keeps no setup state: no SetupNeeded
// gate, cache, or synthesized 410. The credential bodies
// (bootstrap username/password, new_password, MFA proof) cross as
// opaque bytes — never parsed, logged, or telemetered — the dest
// shared key is never injected as Authorization on these routes, the
// core's Set-Cookie and one-time recovery-code answers relay
// unmodified, and a forwarded POST is never retried.
//
// Governance audit stays core-side on forwarded requests: the
// in-tree setup handlers emit audit (setup_claimed,
// setup_admin_password_set, setup_completed and every denied cause)
// from inside the core handlers that still run on every forwarded
// request, so write-through keeps every audit event core-side with
// no gap. Dest emits none. A family whose handlers emit audit must
// never be re-implemented dest-side without a versioned audit-append
// contract; this lane re-implements nothing.
func (s *Server) setupWriteThrough(w http.ResponseWriter, r *http.Request) {
	if s.core == nil {
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	forwardToCore(s, w, r, viaNative)
}
