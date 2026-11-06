// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/url"

	"github.com/eshu-hq/eshu-api/openapi"
)

// Server holds the dependencies of the dispatch surface. It answers
// health and readiness itself and proxies the authenticated /api/*
// subtree to the configured core origin (see dispatch.go). The server
// carries no credential: the key gates requests and crosses to the core
// only in the Authorization header, never in logs or health output.
// Telemetry (see telemetry.go) records per-family request, dispatch,
// authorizer, and audit-drop signals plus the structured request log.
type Server struct {
	coreBaseURL string
	apiKey      string
	core        *url.URL
	httpClient  *http.Client
	telemetry   *Telemetry
}

// NewServer builds a Server that dispatches to coreBaseURL and gates
// /api/* behind apiKey. An empty apiKey fails closed: every /api/*
// request is denied until a key is configured. An empty or unparsable
// origin fails closed too: dispatch answers the local 503 until a valid
// origin is configured. Telemetry defaults to a log-silent registry;
// use WithTelemetry to attach a logging one.
func NewServer(coreBaseURL, apiKey string) *Server {
	return &Server{
		coreBaseURL: coreBaseURL,
		apiKey:      apiKey,
		core:        parseCoreOrigin(coreBaseURL),
		httpClient:  dispatchClient(),
		telemetry:   NewTelemetry(nil),
	}
}

// WithTelemetry attaches tel as this server's signal registry and
// returns the server for chaining. The binary wires its structured
// logger here; tests attach capturing or discard registries.
func (s *Server) WithTelemetry(tel *Telemetry) *Server {
	s.telemetry = tel
	return s
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
// origin, never the API key. Every admission port has its own explicit
// key; admission_contracts stays as the legacy suppression alias.
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":                                       "ok",
		"service":                                      "eshu-api",
		"core_read_contracts":                          CoreReadContractsVersion,
		"admission_contracts":                          SuppressionAdmissionContractVersion,
		"suppression_admission_contracts":              SuppressionAdmissionContractVersion,
		"terminal_failure_admission_contracts":         TerminalFailureAdmissionContractVersion,
		"replay_ledger_admission_contracts":            ReplayLedgerAdmissionContractVersion,
		"sign_in_policy_admission_contracts":           SignInPolicyAdmissionContractVersion,
		"retirement_admission_contracts":               RetirementAdmissionContractVersion,
		"admin_identity_admission_contracts":           AdminIdentityAdmissionContractVersion,
		"setup_admission_contracts":                    SetupAdmissionContractVersion,
		"provider_config_mutation_admission_contracts": ProviderConfigMutationAdmissionContractVersion,
		"provider_config_read_admission_contracts":     ProviderConfigReadAdmissionContractVersion,
		"oidc_login_admission_contracts":               OIDCLoginAdmissionContractVersion,
		"github_login_admission_contracts":             GitHubLoginAdmissionContractVersion,
		"saml_login_admission_contracts":               SAMLLoginAdmissionContractVersion,
		"core":                                         s.coreBaseURL,
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

// routes registers the dispatch surface. Health, readiness, and the
// metrics exposition are public; the /api/* subtree is auth-gated and
// dispatched to the core origin byte-for-byte (see dispatch.go). Every
// dispatch handler runs inside the telemetry middleware (outside the
// auth gate, so gate denials count per family too); probes never do.
func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /metrics", s.metrics)
	// Native-dispatch pilot: the suppression upsert has its explicit
	// per-family route with the AllScopes authorizer. It is registered
	// ahead of the generic proxy; every other family still crosses the
	// byte-relay below.
	mux.Handle(suppressionRoute, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.suppressionWriteThrough))))
	// The trailing-slash spelling is the same upsert (core mounts the
	// exact path and decides; dest keeps both spellings on the native
	// route so the family is never split across dispatch paths).
	mux.Handle(suppressionRoute+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.suppressionWriteThrough))))
	// Native dispatch, SignInPolicyMutations (tier C-a): the admin
	// sign-in policy PATCH has its explicit per-family route with the
	// AllScopes authorizer, ahead of the generic proxy. The trailing
	// slash stays on the native route for the same never-split reason;
	// the family's GETs and every other family still cross the
	// byte-relay below.
	mux.Handle(signInPolicyRoute, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.signInPolicyWriteThrough))))
	mux.Handle(signInPolicyRoute+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.signInPolicyWriteThrough))))
	// Native dispatch, AdminRetirement (tier C-a): the operator
	// retirement trigger has its explicit per-family route with the
	// AllScopes authorizer, ahead of the generic proxy. The trailing
	// slash stays on the native route for the same never-split reason;
	// every other family still crosses the byte-relay below.
	mux.Handle(retirementRoute, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.retirementWriteThrough))))
	mux.Handle(retirementRoute+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.retirementWriteThrough))))
	// Native spec (Decision 5(c)): the sign-in-policy and retirement
	// fragments landed in
	// openapi/, so the assembled document is served from compiled
	// constants ahead of the generic proxy — no core fetch, served even
	// with no core origin configured. It stays inside RequireAuth like
	// every dest /api/* route (core serves it public; dest adds no new
	// public surface here). The document serves exactly the landed
	// families; the rest still relay from core only via their own routes,
	// not via this endpoint.
	mux.Handle("GET /api/v0/openapi.json", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(openapi.ServeSpec))))
	mux.Handle("/api/", s.instrumented(viaRelay, RequireAuth(s.apiKey, http.HandlerFunc(s.dispatch))))
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
