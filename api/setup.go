// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// Setup native-dispatch routes (D10: tier-C POSTs plus a terminated
// GET): the four pre-auth first-run wizard routes. They are
// registered explicitly OUTSIDE RequireAuth, ahead of the generic
// /api/ proxy, because a fresh deployment has no session, bearer, or
// prior credential to present. Route shapes are confirmed against
// the in-tree SetupHandler.Mount and go/cmd/api/CUTOVER_OWNERSHIP.md
// (Setup family, wireAPI) at the provenance pin: the state check
// plus the claim, admin, and MFA wizard steps under /api/v0/auth/....
// The state GET terminates via the SetupNeeded port (see
// setup_terminate.go); the const stays here so the carve-out reads
// as the same four routes.
const (
	setupStateRoute = "GET /api/v0/auth/setup-state"
	setupClaimRoute = "POST /api/v0/auth/setup/claim"
	setupAdminRoute = "POST /api/v0/auth/setup/admin"
	setupMFARoute   = "POST /api/v0/auth/setup/mfa"
)

// setupMutationRoutes lists the three tier-C write-through routes of
// the Setup family in registration order. The setup-state GET is not
// listed here: it terminates via the SetupNeeded port (see
// setup_terminate.go). The carve-out stays exactly four public routes
// (state plus these three, each with its trailing-slash twin).
var setupMutationRoutes = []string{
	setupClaimRoute,
	setupAdminRoute,
	setupMFARoute,
}

// setupWriteThrough is the W7-writers terminated writer for the
// three setup wizard credential POSTs: each request crosses an
// explicit public per-family route instead of the generic
// byte-relay, with no dest-side gate. The shape is HTTP
// write-through — method, path, query, headers, and body forward to
// the same core route, and core bytes relay back unmodified with
// exactly one forward attempt and no retry — because every trust
// decision stays core-side: core's AuthMiddleware bypass
// (publicHTTPPaths) plus the per-request SetupStore re-checks
// (VerifyBootstrapCredential proof and the permanent SetupNeeded seal
// that answers 410 once any identity exists) still run on every
// forwarded request. Re-implementing these POSTs dest-side would
// require parsing credential bodies, which is forbidden on this lane:
// they forward, and the setup-state GET is the only route that
// terminates via a port (the SetupNeeded port, never touching a
// credential).
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
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	forwardToCore(s, w, r, viaNative)
}
