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

// health reports the served contract versions and the intended core origin.
// It takes no credential and never discloses the API key. Every admission
// port has its own explicit key; admission_contracts stays as the legacy suppression alias.
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":                                            "ok",
		"service":                                           "eshu-api",
		"core_read_contracts":                               CoreReadContractsVersion,
		"admission_contracts":                               SuppressionAdmissionContractVersion,
		"suppression_admission_contracts":                   SuppressionAdmissionContractVersion,
		"terminal_failure_admission_contracts":              TerminalFailureAdmissionContractVersion,
		"replay_ledger_admission_contracts":                 ReplayLedgerAdmissionContractVersion,
		"recovery_admission_contracts":                      RecoveryAdmissionContractVersion,
		"sign_in_policy_admission_contracts":                SignInPolicyAdmissionContractVersion,
		"retirement_admission_contracts":                    RetirementAdmissionContractVersion,
		"admin_identity_admission_contracts":                AdminIdentityAdmissionContractVersion,
		"setup_admission_contracts":                         SetupAdmissionContractVersion,
		"provider_config_mutation_admission_contracts":      ProviderConfigMutationAdmissionContractVersion,
		"provider_config_read_admission_contracts":          ProviderConfigReadAdmissionContractVersion,
		"oidc_login_admission_contracts":                    OIDCLoginAdmissionContractVersion,
		"github_login_admission_contracts":                  GitHubLoginAdmissionContractVersion,
		"saml_login_admission_contracts":                    SAMLLoginAdmissionContractVersion,
		"admin_read_admission_contracts":                    AdminReadAdmissionContractVersion,
		"admin_identity_read_admission_contracts":           AdminIdentityReadAdmissionContractVersion,
		"governance_audit_read_admission_contracts":         GovernanceAuditReadAdmissionContractVersion,
		"terminal_failure_read_admission_contracts":         TerminalFailureReadAdmissionContractVersion,
		"delta_union_completeness_read_admission_contracts": DeltaUnionCompletenessReadAdmissionContractVersion,
		"reindex_progress_read_admission_contracts":         ReindexProgressReadAdmissionContractVersion,
		"shared_projection_tuning_report_contracts":         SharedProjectionTuningReportContractVersion,
		"retire_unclaimed_admission_contracts":              RetireUnclaimedAdmissionContractVersion,
		"core":                                              s.coreBaseURL,
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

// routes registers the dispatch surface. Health, readiness, and metrics
// are public; /api/* is auth-gated and dispatched to the core origin
// byte-for-byte (see dispatch.go), except the pre-auth carve-outs below.
// Every dispatch handler runs inside the telemetry middleware (outside
// the auth gate, so gate denials count per family too); probes never do.
func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /metrics", s.metrics)
	// Native-dispatch singletons (suppression, sign-in-policy,
	// retirement, terminal-failure): mounted in api/native_singles.go
	// (pure move to hold the file cap; behavior identical).
	s.registerNativeSingles(mux)
	// AdminRetireUnclaimed writer: explicit route + slash twin ahead of the relay, AllScopes-gated (see api/retireunclaimed.go).
	mux.Handle(retireUnclaimedRoute, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.retireUnclaimedWriteThrough))))
	mux.Handle(retireUnclaimedRoute+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.retireUnclaimedWriteThrough))))
	// Native dispatch, IaC reads (W3 IaC-6, see api/iac.go): the four
	// POST legs cleared on IaCReadAdmission v0 plus the dead POST leg
	// cleared on the supplemental IaCDeadContentAdmission v0 dual pin,
	// plus the resources candidate-page GET cleared on
	// IaCInventorySearchAdmission v0 (own slice, own pin, same
	// write-through handler).
	for _, route := range iacNativeRoutes {
		mux.Handle(route, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.iacWriteThrough))))
		mux.Handle(route+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.iacWriteThrough))))
	}
	for _, route := range iacResourcesRoutes {
		mux.Handle(route, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.iacWriteThrough))))
		mux.Handle(route+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.iacWriteThrough))))
	}
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
	// Native dispatch, Setup (D10 GET plus W7-writers trio): the four
	// pre-auth first-run wizard routes are the strict
	// arbiter-approved carve-out (C-c1 APPROVED on eshu-hq/eshu#9669,
	// graduated to D8-grade public-route handling on the D10 wave):
	// registered explicitly OUTSIDE RequireAuth, ahead of the generic
	// proxy. The setup-state GET terminates dest-side via the
	// SetupNeeded port (see setup_terminate.go); the three credential
	// POSTs terminate natively as writers on the same native-public
	// handler (see setup.go). Each trailing-slash spelling stays on
	// its route's handler for the same never-split reason. No other
	// /api/* route leaves RequireAuth in this lane: the carve-out is
	// exactly these four.
	mux.Handle(setupStateRoute, s.instrumented(viaNative, http.HandlerFunc(s.serveSetupState)))
	mux.Handle(setupStateRoute+"/", s.instrumented(viaNative, http.HandlerFunc(s.serveSetupState)))
	for _, route := range setupMutationRoutes {
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
	// same never-split reason; the admin-four (backfill, dead-letter,
	// reindex, skip) ride the D7 tier-C clone lane below and the
	// family's reads stay on the generic relay.
	// Native dispatch, AdminFourMutations (tier C, D7): the four
	// admin-four mutation routes have their explicit per-family routes
	// with the AllScopes authorizer, ahead of the generic proxy. Each
	// trailing-slash spelling stays on the native route for the same
	// never-split reason. See api/d7.go and contracts/d7-admin-four.md:
	// tier-C dispatch only, not termination.
	for _, route := range d7Routes {
		mux.Handle(route, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.d7WriteThrough))))
		mux.Handle(route+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.d7WriteThrough))))
	}
	// Native dispatch, SessionResolver tier-C clones (D8): the 18
	// authed session/token-lifecycle routes cross the D8
	// credential-class gate — a valid shared key rides the native
	// write-through with the family authorizer; a foreign credential
	// class (scoped token, OIDC bearer, session cookie) relays
	// explicitly on the relay lane so core judges it;
	// credential-less callers keep the standing 401. Each
	// trailing-slash spelling stays on the native route for the same
	// never-split reason. See api/d8.go and
	// contracts/d8-session-resolver.md: tier-C dispatch only, not
	// termination.
	for _, route := range d8GatedRoutes {
		gated := s.instrumented(viaNative, s.d8Gate(http.HandlerFunc(s.d8WriteThrough)))
		mux.Handle(route, gated)
		// The two d8SlashSkip entries skip the slash double: their
		// slash subtree would panic against the
		// invitations/{invite_id}/revoke wildcard sibling. Those
		// spellings stay on the generic relay.
		if !d8SlashSkip[route] {
			mux.Handle(route+"/", gated)
		}
	}
	// Native dispatch, SessionResolver public carve-out (D8, C-c1
	// grade): the 4 pre-auth public routes register exactly those
	// paths outside every gate, ahead of the generic proxy, on the
	// public write-through handler. No other D8 route leaves the gate
	// in this lane: the carve-out is exactly these four exact paths.
	// Credential bodies cross as opaque bytes; the dest shared key is
	// never injected here.
	for _, route := range d8PublicRoutes {
		public := s.instrumented(viaNative, http.HandlerFunc(s.d8PublicWriteThrough))
		// Exact paths only: a trailing-slash twin would be a
		// subtree pattern swallowing /login/* past the auth
		// bypass, and the carve-out is exactly these four paths.
		// The slash spellings stay on the generic relay (keyless
		// callers keep the standing 401 there; keyed callers get
		// core's answer for the spelled path).
		mux.Handle(route, public)
	}
	// Native termination, OIDC-login pair (see
	// api/oidc_terminate.go): the 2 pre-auth public GETs terminate
	// dest-side, registered exactly those paths outside every gate.
	// GitHub/SAML ride their own lanes; non-GET spellings keep the
	// standing 401; credentials cross as opaque bytes.
	for _, route := range d9Routes {
		terminated := s.instrumented(viaNative, http.HandlerFunc(s.oidcTerminatedHandler(route)))
		// Exact paths only: a trailing-slash twin would be a
		// subtree pattern swallowing past the auth bypass, and the
		// carve-out is exactly these two paths. The slash
		// spellings stay on the generic relay (keyless callers keep
		// the standing 401 there; keyed callers get core's answer
		// for the spelled path).
		mux.Handle(route, terminated)
	}
	// Native termination, GitHub-login pair (see
	// api/github_terminate.go): the 2 pre-auth public GETs terminate
	// dest-side, registered exactly those paths outside every gate.
	// OIDC stays on its own landed termination lane and SAML rides
	// its own landed lane; non-GET spellings keep the standing 401;
	// credentials cross as opaque bytes.
	for _, route := range d9GitHubRoutes {
		terminated := s.instrumented(viaNative, http.HandlerFunc(s.githubTerminatedHandler(route)))
		// Exact paths only: a trailing-slash twin would be a
		// subtree pattern swallowing past the auth bypass, and the
		// carve-out is exactly these two paths. The slash
		// spellings stay on the generic relay (keyless callers keep
		// the standing 401 there; keyed callers get core's answer
		// for the spelled path).
		mux.Handle(route, terminated)
	}
	// Native termination, SAML-login triple (see
	// api/saml_terminate.go): the 2 pre-auth public GETs plus the
	// pre-auth public ACS POST terminate dest-side, registered
	// exactly those paths outside every gate. OIDC and GitHub stay
	// on their own landed termination lanes; non-GET spellings of
	// the metadata/login paths (plus non-POST spellings of the ACS
	// path) keep the standing 401; credentials cross as opaque bytes.
	for _, route := range d9SAMLRoutes {
		terminated := s.instrumented(viaNative, http.HandlerFunc(s.samlTerminatedHandler(route)))
		// Exact paths only: a trailing-slash twin would be a
		// subtree pattern swallowing past the auth bypass, and the
		// carve-out is exactly these three paths. The slash
		// spellings stay on the generic relay (keyless callers keep
		// the standing 401 there; keyed callers get core's answer
		// for the spelled path).
		mux.Handle(route, terminated)
	}
	for _, route := range recoveryRoutes {
		mux.Handle(route, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.recoveryWriteThrough))))
		mux.Handle(route+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.recoveryWriteThrough))))
	}
	// W7-read admin sign-in-policy GET: family-owned termination
	// (see api/signinpolicy_admin_terminate.go), ahead of the LeafB loop.
	s.registerAdminSignInPolicyRead(mux)
	// D2 Leaf B Native-now termination (Refs eshu-hq/eshu#9669): the
	// wave-gated reads register explicitly inside the wave gate,
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
	// reason. The 3 Gap 02 Admin GETs (delta-union completeness,
	// reindex progress, tuning-report) ride the trio registration
	// below; every other family still crosses its existing lane.
	for _, entry := range d3Wave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.d3ReadGate(http.HandlerFunc(s.serveD3Terminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
	// D3 Gap 02 trio termination (Refs eshu-hq/eshu#9669, satellite
	// eshu-hq/eshu-api#70): the three Gap 02-pinned Admin GETs register
	// explicitly inside the same D3 wave gate, ahead of the generic
	// proxy — same shared-key authorizer, same explicit-relay rule for
	// foreign credential classes, same standing 401, same never-split
	// trailing-slash rule. See api/d3trio_wave.go and
	// contracts/d3-admin-reads.md.
	for _, entry := range d3TrioWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.d3ReadGate(http.HandlerFunc(s.serveD3Terminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
	// D4 Native-next reads termination (Refs eshu-hq/eshu#9669): the
	// 45 Native-next read routes register explicitly inside the D4 wave
	// gate, ahead of the generic proxy. The repositories story GET stays
	// on the generic relay (its fragment waits on the schema helper port). A valid shared key terminates
	// on the native lane; a foreign credential class (scoped token,
	// OIDC bearer, session cookie) relays explicitly on the relay lane
	// so core judges it; credential-less callers — and junk-cookie
	// carriers, per the D2 follow-up lesson — get the standing 401.
	// Each trailing-slash spelling stays on the wave route for the same
	// never-split reason. Compare, and every other family still cross
	// their existing lanes.
	for _, entry := range d4Wave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.d4ReadGate(http.HandlerFunc(s.serveD4Terminated)))
		mux.Handle(pattern, gated)
		// The four d4SlashSkip literals skip the slash double: their
		// slash subtree would panic against the wildcard sibling (see
		// d4_wave.go). Those spellings still terminate natively under
		// the parent list-subtree registration.
		if !d4SlashSkip[pattern] {
			mux.Handle(pattern+"/", gated)
		}
	}
	// D4 story relay leg (Refs eshu-hq/eshu#9669): the reverted story
	// GET rides the keyed generic-relay stack (RequireAuth + dispatch),
	// registered explicitly ahead of the generic proxy. The explicit
	// wildcard is load-bearing: the list route's slash twin above is a
	// subtree that would otherwise swallow the story path back under
	// the D4 gate, so only the explicit registration restores true
	// relay behavior. The trailing-slash twin relays with it for the
	// same never-split reason.
	for _, pattern := range d4RelayLegs {
		relayed := s.instrumented(viaRelay, RequireAuth(s.apiKey, http.HandlerFunc(s.dispatch)))
		mux.Handle(pattern, relayed)
		mux.Handle(pattern+"/", relayed)
	}
	s.registerCompareWave(mux)
	s.registerCodeGraphWave(mux)
	s.registerCodeFlowWave(mux)
	// W4 Code search termination (Refs eshu-hq/eshu#9669, satellite
	// eshu-hq/eshu-api#113): the 2 pinned code search POSTs register
	// explicitly inside their own wave gate, ahead of the generic
	// proxy — never merged into the W6a Code-flow table. See
	// api/code_search_wave.go.
	s.registerCodeSearchWave(mux)
	// Satellite Wave-4 Drift termination (Refs eshu-hq/eshu#9669,
	// satellite eshu-hq/eshu-api#117): the 2 pinned drift routes
	// register explicitly inside their own wave gate, ahead of the
	// generic proxy — never merged into the W8a table. See
	// api/drift_wave.go.
	s.registerDriftWave(mux)
	// D6 Status + SupplyChain reads termination (see api/d6_wave.go).
	s.registerD6Wave(mux)
	// W8a satellite termination (Refs eshu-hq/eshu#9669, satellite
	// eshu-hq/eshu-api#100): the 16 pinned W8a reads register
	// explicitly inside the W8a wave gate, ahead of the generic proxy.
	// A valid shared key terminates on the native lane; a foreign
	// credential class (scoped token, OIDC bearer, session cookie)
	// relays explicitly on the relay lane so core judges it;
	// credential-less callers — and junk-cookie carriers, per the D2
	// follow-up lesson — get the standing 401. Each trailing-slash
	// spelling stays on the wave route for the same never-split
	// reason. See api/w8a_wave.go and contracts/w8a-termination.md.
	s.registerW8aWave(mux)
	// Wave-1 Freshness reads termination (Refs eshu-hq/eshu#9669, D13
	// tracker eshu-hq/eshu#10491): the 3 pinned Freshness GETs register
	// explicitly inside the wave gate, ahead of the generic proxy. See
	// api/freshness_wave.go.
	s.registerFreshnessWave(mux)
	// Wave-2 Content point reads termination (Refs eshu-hq/eshu#9669,
	// D13 tracker eshu-hq/eshu#10491): the 3 pinned Content POSTs
	// register explicitly inside the wave gate, ahead of the generic
	// proxy. See api/content_wave.go.
	s.registerContentPointReads(mux)
	// W4 Content search termination (Refs eshu-hq/eshu#9669, satellite
	// eshu-hq/eshu-api#111): the 4 pinned search POSTs register
	// explicitly inside their own wave gate, ahead of the generic
	// proxy — never merged into the Wave-2 point-read table. See
	// api/content_search_wave.go.
	s.registerContentSearchWave(mux)
	// Signinpolicy-group native termination (Refs eshu-hq/eshu#9669,
	// satellite eshu-hq/eshu-api#72): the two public login-picker
	// reads terminate dest-side via the SignInPolicyReadAdmission v0
	// port (see api/signinpolicy_terminate.go), registered explicitly
	// OUTSIDE any gate, ahead of the generic proxy and ahead of the D2
	// wave-public loop below. Exact paths only: the trailing-slash
	// twins and deeper subtrees stay on the generic relay behind
	// RequireAuth (keyless callers keep the standing 401 there; keyed
	// callers get core's answer for the spelled path), and non-GET
	// spellings keep the standing 401. The public registration is
	// exactly these two; the admin sign-in-policy GET terminates on
	// its family lane (see api/signinpolicy_admin_terminate.go) and
	// the PATCH terminates behind its family writer gate (see
	// api/signin.go).
	mux.Handle("GET /api/v0/auth/sign-in-policy", s.instrumented(viaNative, http.HandlerFunc(s.servePublicSignInPolicy)))
	mux.Handle("GET /api/v0/auth/providers", s.instrumented(viaNative, http.HandlerFunc(s.serveProviders)))
	// D2 wave-public carve-out (core-public parity): the routes core
	// serves pre-auth register explicitly OUTSIDE any gate, ahead of
	// the generic proxy. Docs and redoc serve compiled shells with no
	// core fetch (each trailing-slash spelling stays on the carve-out:
	// the static shells answer the slash spelling dest-side while
	// core 404s it — disclosed in contracts/leafb-native-now.md). The
	// two login-picker data reads are served by the native synthesis
	// above (exact paths only, twins stay on the generic relay), so
	// the loop below serves only the doc shells now. No other /api/*
	// route leaves the gate in this lane.
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
			continue
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
