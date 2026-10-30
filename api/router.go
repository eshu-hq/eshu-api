// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/url"
)

// Server holds the dependencies of the dispatch surface. It answers
// health and readiness itself and proxies the authenticated /api/*
// subtree to the configured core origin (see dispatch.go). The server
// carries no credential: the key gates requests and crosses to the core
// only in the Authorization header, never in logs or health output.
type Server struct {
	coreBaseURL string
	apiKey      string
	core        *url.URL
	httpClient  *http.Client
}

// NewServer builds a Server that dispatches to coreBaseURL and gates
// /api/* behind apiKey. An empty apiKey fails closed: every /api/*
// request is denied until a key is configured. An empty or unparsable
// origin fails closed too: dispatch answers the local 503 until a valid
// origin is configured.
func NewServer(coreBaseURL, apiKey string) *Server {
	return &Server{
		coreBaseURL: coreBaseURL,
		apiKey:      apiKey,
		core:        parseCoreOrigin(coreBaseURL),
		httpClient:  dispatchClient(),
	}
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
// origin. It takes no credential; it discloses only the configured core
// origin, never the API key.
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

// routes registers the dispatch surface. Health and readiness are
// public; the /api/* subtree is auth-gated and dispatched to the core
// origin byte-for-byte (see dispatch.go).
func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.Handle("/api/", RequireAuth(s.apiKey, http.HandlerFunc(s.dispatch)))
	return mux
}

// Handler returns the production middleware chain over the dispatch
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
