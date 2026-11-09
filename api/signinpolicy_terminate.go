// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// Signinpolicy-group native termination (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#72, signinpolicy-group lane, first of the
// sequential termination waves). GET /api/v0/auth/sign-in-policy and
// GET /api/v0/auth/providers terminate dest-side as native-public
// routes (exact paths only, outside every gate, ahead of the generic
// proxy): the policy read crosses the frozen SignInPolicyReadAdmission
// v0 port, dest synthesizes the 200, and every failure closes to the
// permanent local 503 (never a relayed failure, never Retry-After).
// Dest takes no table or store handle anywhere in this file.
//
// The admin sign-in-policy GET and the PATCH on the same path stay
// exactly where they are (wave-gated relay and RequireAuth
// write-through): this lane widens nothing. Trailing-slash twins,
// deeper subtrees, and non-GET spellings of the two public paths stay
// on the generic relay behind RequireAuth, so keyless callers keep
// the standing 401 with zero core contact there.
//
// Rollback is revert-to-relay: delete the two exact-path native
// registrations in routes() and restore the default branch in the
// wave-public loop to forward the two data reads; no data migrates
// and no other family moves.

// signInPolicy is the dest-owned policy shape this lane synthesizes
// from: only the require_sso bit the public surface exposes. The
// admin detail shape stays core-side; dest never projects it.
type signInPolicy struct {
	RequireSSO bool `json:"require_sso"`
}

// signInPolicyReadPort is the dest-side seam behind the terminated
// public sign-in-policy GET (and the policy half of the terminated
// providers GET): the single GetSignInPolicy read of the frozen
// SignInPolicyReadAdmission v0 set, with the exact frozen signature so
// a re-signed dest port fails the build the same way a re-signed core
// method fails the frozen contract test. Production satisfies it with
// a core-fetching adapter over D0-proven transport (one anonymous
// GET, no retry); tests drive the same adapter against the stub core.
// Dest implements no other read here and keeps no policy state.
type signInPolicyReadPort interface {
	// GetSignInPolicy reports the tenant's sign-in policy read: the
	// require_sso bit the public surface exposes. It mirrors the
	// frozen core method so the port reads against the vendored
	// contract without a core import.
	GetSignInPolicy(ctx context.Context, tenantID string) (signInPolicy, error)
}

// coreSignInPolicyPort is the production signInPolicyReadPort: it reads
// the require_sso signal from the configured core origin with a single
// anonymous GET of the canonical core public sign-in-policy route. The
// adapter is request-scoped (built per call in the handlers, never
// shared), so nothing is cached: a second call would fetch again
// (callers make exactly one). The fetch is anonymous by construction
// — the route is public pre-auth at the core too — and performs no
// retry: one fetch, then the caller fails closed.
type coreSignInPolicyPort struct {
	server      *Server
	tenant      string
	query       string
	correlation string
	traceparent string
	tracestate  string
	accept      string
}

// coreSignInPolicyPort satisfies signInPolicyReadPort: the terminated
// GETs serve from this seam, so a re-signed dest port fails the build
// the same way a re-signed core method fails the frozen test.
var _ signInPolicyReadPort = (*coreSignInPolicyPort)(nil)

// GetSignInPolicy implements signInPolicyReadPort over D0-proven
// transport: one anonymous GET of the canonical core public route. An
// empty tenant never fetches: it returns the safe zero default
// (require_sso=false) with zero core contact, matching the core
// handler's own empty-tenant default instead of a cross-tenant scan.
func (p *coreSignInPolicyPort) GetSignInPolicy(ctx context.Context, tenantID string) (signInPolicy, error) {
	if tenantID == "" {
		return signInPolicy{RequireSSO: false}, nil
	}
	return p.server.fetchSignInPolicy(ctx, signInPolicyFetchParams{
		tenant:      tenantID,
		query:       p.query,
		correlation: p.correlation,
		traceparent: p.traceparent,
		tracestate:  p.tracestate,
		accept:      p.accept,
	})
}

// signInPolicyFetchParams carries the inbound request values the
// anonymous port fetch is allowed to see: the tenant, the query
// string, the guaranteed correlation ID, W3C trace context when the
// caller sent it, and the caller Accept for content negotiation.
// Nothing else crosses — in particular no Authorization header and no
// other caller metadata — so no credential can reach the fetch path
// and telemetry never observes one.
type signInPolicyFetchParams struct {
	tenant      string
	query       string
	correlation string
	traceparent string
	tracestate  string
	accept      string
}

// signInPolicyFetchResult is the terminated answer: the require_sso
// bit the public surface exposes.
type signInPolicyFetchResult = signInPolicy

// errSignInPolicyFetch marks any port-fetch failure the terminated
// GETs map to the permanent local 503. It carries no core detail: a
// store error never reaches the body.
var errSignInPolicyFetch = errSignInPolicyFetchType{}

type errSignInPolicyFetchType struct{}

func (errSignInPolicyFetchType) Error() string { return "eshu-api: sign-in-policy fetch failed" }

// fetchSignInPolicy performs the single port fetch: GET of the
// canonical core public sign-in-policy route with the caller's query
// string, the guaranteed correlation ID, and W3C trace context when
// supplied. No Authorization header is ever attached — the dest
// shared key is never injected and no caller credential crosses —
// and the twin spelling never crosses either: dest matched the exact
// route, so the core always sees the single canonical path. A non-200
// core answer, an unreadable body, or a missing require_sso key is a
// fetch error: the caller fails closed downstream. A core 200 with
// require_sso=false (the core fail-open on a store outage) decodes to
// false and stays a 200: dest preserves the core fail-open, it does
// not invent a 503 for it.
func (s *Server) fetchSignInPolicy(ctx context.Context, params signInPolicyFetchParams) (signInPolicyFetchResult, error) {
	target := strings.TrimRight(s.coreBaseURL, "/") + "/api/v0/auth/sign-in-policy"
	if params.query != "" {
		target += "?" + params.query
	}
	upstream, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return signInPolicy{}, errSignInPolicyFetch
	}
	upstream.Header.Set(correlationHeader, params.correlation)
	if params.accept != "" {
		upstream.Header.Set("Accept", params.accept)
	}
	if params.traceparent != "" {
		upstream.Header.Set("traceparent", params.traceparent)
	}
	if params.tracestate != "" {
		upstream.Header.Set("tracestate", params.tracestate)
	}
	upstream.Host = s.core.Host

	resp, err := s.httpClient.Do(upstream)
	if err != nil {
		return signInPolicy{}, errSignInPolicyFetch
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return signInPolicy{}, errSignInPolicyFetch
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return signInPolicy{}, errSignInPolicyFetch
	}
	var decoded struct {
		RequireSSO *bool `json:"require_sso"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return signInPolicy{}, errSignInPolicyFetch
	}
	if decoded.RequireSSO == nil {
		// No signal, no answer: defaulting an absent key to false
		// would report a fail-open hint on garbage.
		return signInPolicy{}, errSignInPolicyFetch
	}
	return signInPolicy{RequireSSO: *decoded.RequireSSO}, nil
}

// authProviderItem is the dest-owned provider entry this lane
// synthesizes: the opaque config id, the safe generic label, and the
// two coarse protocol-class hints. No domain, metadata URL, IdP name,
// or credential ever lands here.
type authProviderItem struct {
	ProviderConfigID string `json:"provider_config_id"`
	DisplayLabel     string `json:"display_label"`
	ProviderKind     string `json:"provider_kind"`
	IconHint         string `json:"icon_hint"`
}

// authPosture is the dest-owned posture shape the terminated
// providers GET synthesizes. Providers is always non-nil (possibly
// empty) so the wire encodes [] never null, matching core.
type authPosture struct {
	Providers                []authProviderItem `json:"providers"`
	LocalLoginOffered        bool               `json:"local_login_offered"`
	SelfServiceTokensOffered bool               `json:"self_service_tokens_offered"`
}

// fetchProvidersPosture performs the single providers-list fetch: GET
// of the canonical core providers route with the caller's query
// string, the guaranteed correlation ID, and W3C trace context when
// supplied. No Authorization header is ever attached and the twin
// spelling never crosses, exactly like the policy fetch. A non-200
// core answer (including the core 500 on a provider-list failure),
// an unreadable body, or a missing providers key is a fetch error:
// the caller fails closed downstream with the permanent local 503 —
// dest never relays a core 500 and never invents a Retry-After. The
// fetched local_login_offered rides along for validation but the
// synthesized value comes from the port (see serveProviders).
func (s *Server) fetchProvidersPosture(ctx context.Context, params signInPolicyFetchParams) (authPosture, error) {
	target := strings.TrimRight(s.coreBaseURL, "/") + "/api/v0/auth/providers"
	if params.query != "" {
		target += "?" + params.query
	}
	upstream, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return authPosture{}, errSignInPolicyFetch
	}
	upstream.Header.Set(correlationHeader, params.correlation)
	if params.accept != "" {
		upstream.Header.Set("Accept", params.accept)
	}
	if params.traceparent != "" {
		upstream.Header.Set("traceparent", params.traceparent)
	}
	if params.tracestate != "" {
		upstream.Header.Set("tracestate", params.tracestate)
	}
	upstream.Host = s.core.Host

	resp, err := s.httpClient.Do(upstream)
	if err != nil {
		return authPosture{}, errSignInPolicyFetch
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return authPosture{}, errSignInPolicyFetch
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return authPosture{}, errSignInPolicyFetch
	}
	var decoded struct {
		Providers *[]authProviderItem `json:"providers"`
		// LocalLoginOffered is validated present for parity but the
		// synthesized value comes from the port, not this field.
		LocalLoginOffered        *bool `json:"local_login_offered"`
		SelfServiceTokensOffered *bool `json:"self_service_tokens_offered"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return authPosture{}, errSignInPolicyFetch
	}
	if decoded.Providers == nil || decoded.LocalLoginOffered == nil || decoded.SelfServiceTokensOffered == nil {
		// No signal, no answer: defaulting absent keys would report
		// a posture on garbage.
		return authPosture{}, errSignInPolicyFetch
	}
	return authPosture{
		Providers:                *decoded.Providers,
		LocalLoginOffered:        *decoded.LocalLoginOffered,
		SelfServiceTokensOffered: *decoded.SelfServiceTokensOffered,
	}, nil
}

// servePublicSignInPolicy terminates one public sign-in-policy GET
// for any caller: the route is public pre-auth, so there is no gate
// and no authorizer judgment to file. The require_sso signal comes
// from the signInPolicyReadPort; dest synthesizes the 200 from it
// with the core Cache-Control posture (public, max-age=30). Every
// other outcome is the permanent local 503 backend_unavailable —
// unconfigured origin, unreachable core, non-200 core answer,
// unreadable body, or missing require_sso key — with exactly one
// upstream attempt and never a Retry-After. Dest synthesizes no
// Location and no Set-Cookie on this route: the 200 carries neither,
// even when the core answer on the fetch carries them. An empty
// tenant answers 200 with require_sso:false and zero core contact.
func (s *Server) servePublicSignInPolicy(w http.ResponseWriter, r *http.Request) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	port := &coreSignInPolicyPort{
		server:      s,
		tenant:      r.URL.Query().Get("tenant_id"),
		query:       r.URL.RawQuery,
		correlation: CorrelationID(r),
		traceparent: r.Header.Get("traceparent"),
		tracestate:  r.Header.Get("tracestate"),
		accept:      r.Header.Get("Accept"),
	}
	fetchStart := time.Now()
	policy, err := port.GetSignInPolicy(r.Context(), port.tenant)
	if err != nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, time.Since(fetchStart), false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core is unavailable")
		return
	}
	s.dispatchOutcome(r, viaNative, outcomeCoreOK, time.Since(fetchStart), false)
	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, http.StatusOK, policy)
}

// serveProviders terminates one providers GET for any caller: the
// route is public pre-auth, so there is no gate and no authorizer
// judgment to file. The provider list comes from a single anonymous
// fetch of the canonical core providers route and the
// local_login_offered bit comes from the signInPolicyReadPort (the
// policy read the O6 pin covers); dest synthesizes the 200 from both
// with the core Cache-Control posture (public, max-age=60). List and
// port each run exactly once with no retry: two upstream hits on the
// success path, one (the list) when the list fails. Every failure —
// unconfigured origin, unreachable core, non-200 core answer on
// either fetch (including the core 500 on a provider-list failure),
// unreadable body, missing keys, or caller cancel — closes to the
// permanent local 503 backend_unavailable with never a Retry-After.
// Dest synthesizes no Location and no Set-Cookie on this route. An
// empty tenant answers the safe zero default (empty providers, local
// login offered, self-service tokens offered) with zero core contact.
func (s *Server) serveProviders(w http.ResponseWriter, r *http.Request) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	params := signInPolicyFetchParams{
		tenant:      r.URL.Query().Get("tenant_id"),
		query:       r.URL.RawQuery,
		correlation: CorrelationID(r),
		traceparent: r.Header.Get("traceparent"),
		tracestate:  r.Header.Get("tracestate"),
		accept:      r.Header.Get("Accept"),
	}
	fetchStart := time.Now()
	if params.tenant == "" {
		// No tenant can be resolved pre-auth without an explicit
		// tenant_id: answer the safe zero-configuration default
		// with zero core contact, matching the core handler
		// instead of a cross-tenant scan.
		s.dispatchOutcome(r, viaNative, outcomeCoreOK, time.Since(fetchStart), false)
		w.Header().Set("Cache-Control", "public, max-age=60")
		writeJSON(w, http.StatusOK, authPosture{
			Providers:                []authProviderItem{},
			LocalLoginOffered:        true,
			SelfServiceTokensOffered: true,
		})
		return
	}
	listed, err := s.fetchProvidersPosture(r.Context(), params)
	if err != nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, time.Since(fetchStart), false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core is unavailable")
		return
	}
	port := &coreSignInPolicyPort{
		server:      s,
		tenant:      params.tenant,
		query:       params.query,
		correlation: params.correlation,
		traceparent: params.traceparent,
		tracestate:  params.tracestate,
		accept:      params.accept,
	}
	policy, err := port.GetSignInPolicy(r.Context(), params.tenant)
	if err != nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, time.Since(fetchStart), false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core is unavailable")
		return
	}
	s.dispatchOutcome(r, viaNative, outcomeCoreOK, time.Since(fetchStart), false)
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, authPosture{
		Providers:                listed.Providers,
		LocalLoginOffered:        !policy.RequireSSO,
		SelfServiceTokensOffered: listed.SelfServiceTokensOffered,
	})
}
