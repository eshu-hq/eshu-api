// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"crypto/subtle"
	"net/http"
)

// D2 Leaf B Native-now termination engine (Refs eshu-hq/eshu#9669,
// part of #6707). The wave table lives in leafb_wave.go; this file
// owns the serving contract: the wave gate, the terminated reader,
// the public carve-out readers, and the doc-UI shells.
//
// Lane rule (arbiter D-wave ruling, decision 4): shared-key callers
// terminate natively; credential classes dest cannot resolve (scoped
// tokens, OIDC bearers, browser sessions) relay explicitly — core
// judges, core bytes come back untouched, dest invents no 401, 403,
// 404, or 500 for them; credential-less callers get the standing dest
// 401. The only dest-synthesized errors on the terminated path are
// the gate 401, the future-resolver family 403, and the permanent
// local 503 when no core origin is configured (never Retry-After).

// authorizeLeafBSharedKey reports whether an identity carrying
// allScopes may terminate a wave-gated read natively. The only
// identity dest can construct today is the shared-key holder, which
// core maps to AllScopes, so the wave gate passes true and core
// re-judges every forwarded call as backstop (tenant scope,
// permission catalog, governance mode — which is also where the live
// 403s and mode-dependent denials come from). The false branch is the
// hookup point for the day dest gains a scoped/session resolver; until
// then it stays a plain deny (no invented reason codes, no audit
// widening). One shared authorizer covers all 12 gated wave routes:
// they share the shared-key trust root until D8 resolves per-class
// handling, and the contract record (contracts/leafb-native-now.md)
// carries the scope-honesty label for each family.
func authorizeLeafBSharedKey(allScopes bool) bool {
	return allScopes
}

// leafbFamilyDenyMessage is the documented family-403 text the dest
// authorizer adds on the unreachable-until-D8 deny branch.
const leafbFamilyDenyMessage = "all-scopes operator authorization is required"

// serveLeafBTerminated terminates one wave-gated read for a
// shared-key caller: the family authorizer runs first, then the call
// crosses the versioned port transport (forwardToCore on the native
// lane) with the caller's negotiation headers and credential intact
// and core bytes come back unmodified.
func (s *Server) serveLeafBTerminated(w http.ResponseWriter, r *http.Request) {
	s.serveLeafBTerminatedAs(w, r, true)
}

// serveLeafBTerminatedAs is the testable core of the terminated
// reader: allScopes is the resolved caller scope. The wave gate always
// passes true today (the shared key maps to AllScopes in core); the
// false branch is the hookup point for the day dest gains a
// scoped/session resolver, and it stays a plain deny until then. Like
// dispatch, it fails closed with the permanent 503 when no core origin
// is configured, so a native route can never dereference an
// unconfigured origin.
func (s *Server) serveLeafBTerminatedAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	// The family authorizer judgment counts on both branches, so the
	// unreachable deny branch stays observable from dest signals. A deny
	// never dispatches: it files the denied outcome on the native lane.
	family := classifyFamily(r.URL.Path)
	if !authorizeLeafBSharedKey(allScopes) {
		if tel := s.telemetry; tel != nil {
			tel.observeAuthorizer(family, decisionDeny)
		}
		s.dispatchOutcome(r, viaNative, outcomeDenied, 0, false)
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			leafbFamilyDenyMessage)
		return
	}
	if tel := s.telemetry; tel != nil {
		tel.observeAuthorizer(family, decisionAllow)
	}
	forwardToCore(s, w, r, viaNative)
}

// serveLeafBPublicRead terminates one wave-public data read (the
// pre-auth login-picker routes). There is no gate: core serves these
// routes publicly, so dest forwards whatever credential arrived (or
// none) and relays core bytes — including the core Cache-Control
// posture — untouched on the native lane. Unconfigured origin still
// fails closed with the permanent local 503.
func (s *Server) serveLeafBPublicRead(w http.ResponseWriter, r *http.Request) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	forwardToCore(s, w, r, viaNative)
}

// leafbWaveGate fronts the 12 wave-gated routes. A valid shared key
// terminates (next); a request carrying any other credential — a
// non-key Bearer (scoped token, OIDC) or a session cookie — relays
// explicitly to the core on the relay lane, so core judges the class
// dest cannot resolve and dest invents no failure for it; a
// credential-less request gets the standing dest 401 with zero core
// contact. An empty configured key fails closed like RequireAuth.
func (s *Server) leafbWaveGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.apiKey == "" {
			WriteUnauthorized(w, r)
			return
		}
		if got, ok := bearerKey(r); ok {
			if subtle.ConstantTimeCompare([]byte(got), []byte(s.apiKey)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
			// A non-key Bearer is a foreign credential class (scoped
			// token, OIDC): relay explicitly so core judges it.
			s.leafbRelayExplicit(w, r)
			return
		}
		if r.Header.Get("Cookie") != "" {
			// A session cookie without a Bearer is a browser-session
			// caller: relay explicitly so core judges it.
			s.leafbRelayExplicit(w, r)
			return
		}
		WriteUnauthorized(w, r)
	})
}

// leafbRelayExplicit forwards one wave-gated call carrying an
// unresolvable credential class to the same core route and relays
// core bytes untouched on the relay lane. The instrumented middleware
// files the lane from the record, so the gate flips the record lane
// before forwarding; direct (uninstrumented) calls already carry the
// relay lane through forwardToCore.
func (s *Server) leafbRelayExplicit(w http.ResponseWriter, r *http.Request) {
	if rec := dispatchNote(r); rec != nil {
		rec.via = viaRelay
	}
	if s.core == nil {
		s.dispatchOutcome(r, viaRelay, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	forwardToCore(s, w, r, viaRelay)
}

// leafbSwaggerUIShell is the dest-served Swagger UI shell for
// GET /api/v0/docs: a line-verbatim subset port of the pinned core
// openapi/spec.go swaggerUIHTML const (the const line is dest-owned;
// the payload lines are gated byte-identical by scripts/check-parity.sh).
// Core serves this shell publicly with text/html; dest matches it.
const leafbSwaggerUIShell = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Eshu API - Swagger UI</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      window.ui = SwaggerUIBundle({
        url: "/api/v0/openapi.json",
        dom_id: "#swagger-ui",
        deepLinking: true
      });
    };
  </script>
</body>
</html>
`

// leafbReDocShell is the dest-served ReDoc shell for
// GET /api/v0/redoc: a line-verbatim subset port of the pinned core
// openapi/spec.go redocHTML const, same gate and headers as above.
const leafbReDocShell = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Eshu API - ReDoc</title>
</head>
<body>
  <redoc spec-url="/api/v0/openapi.json"></redoc>
  <script src="https://cdn.jsdelivr.net/npm/redoc@2/bundles/redoc.standalone.js"></script>
</body>
</html>
`

// serveLeafBDocUI writes one doc-UI shell with the core mount's
// headers (HTML with charset, 200, no CORS variation): JSON with
// charset is the spec route's shape, HTML is the shell routes' shape.
// The shells are compiled constants — no core fetch, so they serve
// even with no core origin configured.
func serveLeafBDocUI(shell string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(shell))
	}
}
