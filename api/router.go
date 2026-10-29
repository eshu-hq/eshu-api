// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// backendUnavailableCode is the stable machine-readable code the
// skeleton answers on core routes it cannot serve yet. It reuses the
// versioned in-tree vocabulary value "backend_unavailable"; the message
// states the skeleton status so no caller mistakes it for a live backend
// outage. The 503 is permanent: it never carries Retry-After, matching
// the in-tree rule that a permanent backend_unavailable never tells a
// client to retry.
const backendUnavailableCode = "backend_unavailable"

// Server holds the dependencies of the pre-cutover serving surface. Core
// dispatch over the versioned read contracts lands next; until then the
// server answers health and readiness itself and the /api/* subtree
// through the auth gate only. The server carries no credential and makes
// no core calls.
type Server struct {
	coreBaseURL string
	apiKey      string
}

// NewServer builds a Server that advertises coreBaseURL as its intended
// core origin and gates /api/* behind apiKey. An empty apiKey fails
// closed: every /api/* request is denied until a key is configured.
func NewServer(coreBaseURL, apiKey string) *Server {
	return &Server{coreBaseURL: coreBaseURL, apiKey: apiKey}
}

// writeError writes a status-coded JSON error in the caller's negotiated
// shape: the envelope error section for envelope callers, the plain
// error_code shape otherwise.
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if acceptsEnvelope(r) {
		writeJSON(w, status, envelope{
			Error: &envelopeError{
				Code:          code,
				Message:       message,
				CorrelationID: CorrelationID(r),
			},
		})
		return
	}
	writeJSON(w, status, errorBody{
		ErrorCode:     code,
		Message:       message,
		CorrelationID: CorrelationID(r),
	})
}

// health reports the served contract versions and the intended core
// origin. It takes no credential and reveals none.
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":              "ok",
		"service":             "eshu-api",
		"core_read_contracts": CoreReadContractsVersion,
		"admission_contracts": SuppressionAdmissionContractVersion,
		"core":                s.coreBaseURL,
	})
}

// ready reports readiness in the same key shape the live probe uses:
// service, probe, status.
func (s *Server) ready(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "eshu-api",
		"probe":   "readyz",
		"status":  "ok",
	})
}

// unavailable answers every core route the skeleton cannot serve yet. It
// sits behind the auth gate, so reaching it proves the caller carried a
// valid credential; the 503 says the route has no core dispatch behind
// it, not that the caller's credential failed.
func (s *Server) unavailable(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
		"eshu-api skeleton: core dispatch is not wired yet; no core routes are served")
}

// routes registers the skeleton surface. Health and readiness are
// public; the /api/* subtree is auth-gated and, until core dispatch
// lands, served by the explicit 503 above.
func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.Handle("/api/", RequireAuth(s.apiKey, http.HandlerFunc(s.unavailable)))
	return mux
}

// Handler returns the production middleware chain over the skeleton
// routes. Layering is load-bearing and pinned by tests:
//
//	deprecation (outer: sees the original path, so /api/v1/* never
//	  carries deprecation headers) -> v1 alias rewrite -> auth gate
//	  (sees the rewritten /api/v0/* path) -> routes.
//
// The sunset date comes from V0SunsetFromEnv at wiring time.
func (s *Server) Handler(sunset string) http.Handler {
	return deprecationHeadersMiddleware(v1PrefixAliasMiddleware(s.routes()), sunset)
}
