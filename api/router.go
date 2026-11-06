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
		"recovery_admission_contracts":                 RecoveryAdmissionContractVersion,
		"sign_in_policy_admission_contracts":           SignInPolicyAdmissionContractVersion,
		"retirement_admission_contracts":               RetirementAdmissionContractVersion,
		"admin_identity_admission_contracts":           AdminIdentityAdmissionContractVersion,
		"setup_admission_contracts":                    SetupAdmissionContractVersion,
		"provider_config_mutation_admission_contracts": ProviderConfigMutationAdmissionContractVersion,
		"provider_config_read_admission_contracts":     ProviderConfigReadAdmissionContractVersion,
		"oidc_login_admission_contracts":               OIDCLoginAdmissionContractVersion,
		"github_login_admission_contracts":             GitHubLoginAdmissionContractVersion,
		"saml_login_admission_contracts":               SAMLLoginAdmissionContractVersion,
		"admin_read_admission_contracts":               AdminReadAdmissionContractVersion,
		"admin_identity_read_admission_contracts":      AdminIdentityReadAdmissionContractVersion,
		"governance_audit_read_admission_contracts":    GovernanceAuditReadAdmissionContractVersion,
		"terminal_failure_read_admission_contracts":    TerminalFailureReadAdmissionContractVersion,
		"core": s.coreBaseURL,
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
// dispatched to the core origin byte-for-byte (see dispatch.go), with
// exactly one exception: the four tier C-c1 setup wizard routes, which
// are public pre-auth at the core and register outside RequireAuth
// below. Every dispatch handler runs inside the telemetry middleware
// (outside the auth gate, so gate denials count per family too);
// probes never do.
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
	// Native dispatch, AdminTerminalFailure (tier C-a): the operator
	// collector-failure acknowledgement POST has its explicit per-family
	// route with the AllScopes authorizer, ahead of the generic proxy.
	// The trailing slash stays on the native route for the same
	// never-split reason; the family's query reads and every other
	// family still cross the byte-relay below.
	mux.Handle(terminalFailureRoute, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.terminalFailureWriteThrough))))
	mux.Handle(terminalFailureRoute+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.terminalFailureWriteThrough))))
	// Native dispatch, AdminIdentityMutations (tier C-b): the five
	// tenant-scoped admin identity mutation routes have their explicit
	// per-family routes with the AllScopes authorizer, ahead of the
	// generic proxy. Each trailing-slash spelling stays on the native
	// route for the same never-split reason; the family's reads and
	// every other family still cross the byte-relay below.
	for _, route := range adminIdentityRoutes {
		mux.Handle(route, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.adminIdentityWriteThrough))))
		mux.Handle(route+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.adminIdentityWriteThrough))))
	}
	// Native dispatch, Setup (tier C-c1): the four pre-auth first-run
	// wizard routes are the strict arbiter-approved carve-out (C-c1
	// APPROVED on eshu-hq/eshu#9669): registered explicitly OUTSIDE
	// RequireAuth, ahead of the generic proxy, on the same native-public
	// handler. Each trailing-slash spelling stays on the native route
	// for the same never-split reason. No other /api/* route leaves
	// RequireAuth in this lane: the carve-out is exactly these four.
	for _, route := range setupRoutes {
		mux.Handle(route, s.instrumented(viaNative, http.HandlerFunc(s.setupWriteThrough)))
		mux.Handle(route+"/", s.instrumented(viaNative, http.HandlerFunc(s.setupWriteThrough)))
	}
	// Native dispatch, AdminProviderConfigMutations (tier C-b): the six
	// admin provider-config mutation routes have their explicit
	// per-family routes with the AllScopes authorizer, ahead of the
	// generic proxy. Each trailing-slash spelling stays on the native
	// route for the same never-split reason; the family's reads and
	// every other family still cross the byte-relay below. The create
	// and update POSTs share path keys with the family list/detail
	// GETs, but Go matches method and path together, so the POSTs ride
	// the native route while those GETs stay relayed.
	for _, route := range providerConfigRoutes {
		mux.Handle(route, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.providerConfigWriteThrough))))
		mux.Handle(route+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.providerConfigWriteThrough))))
	}
	// Native dispatch, AdminRecoveryMutations (tier C-c2): the four
	// admin recovery mutation routes have their explicit per-family
	// routes with the AllScopes authorizer, ahead of the generic proxy.
	// Each trailing-slash spelling stays on the native route for the
	// same never-split reason; the relay-only four (backfill,
	// dead-letter, reindex, skip) and the family's reads stay on the
	// generic relay — they have no admission pin and nobody infers them
	// into this lane.
	for _, route := range recoveryRoutes {
		mux.Handle(route, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.recoveryWriteThrough))))
		mux.Handle(route+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.recoveryWriteThrough))))
	}
	// D2 Leaf B Native-now termination (Refs eshu-hq/eshu#9669): the
	// 12 wave-gated reads register explicitly inside the wave gate,
	// ahead of the generic proxy. A valid shared key terminates on the
	// native lane; a foreign credential class (scoped token, OIDC
	// bearer, session cookie) relays explicitly on the relay lane so
	// core judges it; credential-less callers get the standing 401.
	// Each trailing-slash spelling stays on the wave route for the same
	// never-split reason. The family's mutations and every other family
	// still cross their existing lanes.
	for _, entry := range leafbWave {
		if entry.Serve != leafbServeGated {
			continue
		}
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.leafbWaveGate(http.HandlerFunc(s.serveLeafBTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
	// D3 admin/identity/recovery reads termination (Refs
	// eshu-hq/eshu#9669): the 16 D1-pinned read routes register
	// explicitly inside the D3 wave gate, ahead of the generic proxy.
	// A valid shared key terminates on the native lane; a foreign
	// credential class (scoped token, OIDC bearer, session cookie)
	// relays explicitly on the relay lane so core judges it;
	// credential-less callers — and junk-cookie carriers, per the D2
	// follow-up lesson — get the standing 401. Each trailing-slash
	// spelling stays on the wave route for the same never-split
	// reason. The 3 unpinned Admin GETs (delta-union completeness,
	// reindex progress, tuning-report) and every other family still
	// cross their existing lanes.
	for _, entry := range d3Wave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.d3ReadGate(http.HandlerFunc(s.serveD3Terminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
	// D2 wave-public carve-out (core-public parity): the four routes
	// core serves pre-auth register explicitly OUTSIDE any gate, ahead
	// of the generic proxy. Docs and redoc serve compiled shells with
	// no core fetch; the login-picker data reads forward whatever
	// credential arrived (or none) and relay core bytes on the native
	// lane. Each trailing-slash spelling stays on the carve-out for
	// the same never-split reason (the static shells answer the slash
	// spelling dest-side while core 404s it — disclosed in
	// contracts/leafb-native-now.md). No other /api/* route leaves the
	// gate in this lane: the carve-out is exactly these four.
	for _, entry := range leafbWave {
		if entry.Serve != leafbServePublic {
			continue
		}
		var public http.Handler
		switch entry.Path {
		case "/api/v0/docs":
			public = s.instrumented(viaNative, serveLeafBDocUI(leafbSwaggerUIShell))
		case "/api/v0/redoc":
			public = s.instrumented(viaNative, serveLeafBDocUI(leafbReDocShell))
		default:
			public = s.instrumented(viaNative, http.HandlerFunc(s.serveLeafBPublicRead))
		}
		pattern := entry.Method + " " + entry.Path
		mux.Handle(pattern, public)
		mux.Handle(pattern+"/", public)
	}
	// Native spec (Decision 5(c)): the sign-in-policy, retirement,
	// terminal-failure, admin-identity, and provider-config fragments
	// landed in openapi/, so the assembled document is served from
	// compiled constants ahead of the generic proxy — no core fetch,
	// served even with no core origin configured.
	// It stays inside RequireAuth like every dest /api/* route (core
	// serves it public; dest adds no new public surface here). The
	// document serves exactly the landed families; the rest still relay
	// from core only via their own routes, not via this endpoint.
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
