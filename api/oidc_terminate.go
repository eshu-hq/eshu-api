// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

// OIDC-pair native termination (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#72, oidc-pair lane, second of the sequential
// termination waves; lands only after the signinpolicy-group merge per
// the D9 order). GET /api/v0/auth/oidc/login and
// GET /api/v0/auth/oidc/callback terminate dest-side as exact-path
// native-public routes (outside every gate, ahead of the generic
// proxy): each makes exactly one anonymous core fetch over D0-proven
// transport, and dest synthesizes the answer from the port result —
// redirect status+Location byte-verbatim, Set-Cookie
// name/value/attrs/expiry verbatim, error envelopes verbatim — with no
// dest retry on either leg (both GETs are census mutations: login
// start creates server-side state, callback consumes it exactly
// once). GitHub and SAML stay on their landed write-through lanes;
// this file never names them.
//
// Opaque bytes (binding): the credential query values (state, code,
// provider IDs, return_to) cross as opaque bytes — this file parses
// none of them (no Query, no ParseForm, no FormValue), logs none of
// them, and records no RequestURI, Location, or Set-Cookie value in
// logs, metrics, or spans (the middleware logs method, family, via,
// status, and latencies only). The callback mutation is never
// retried and never serialized dest-side: a parallel duplicate
// callback makes one fetch per delivery and core's ConsumeState
// judges exactly-once (one session, loser core error).
//
// Login-state and session stores stay core-side: this file takes no
// table or store handle anywhere (no sql, store, table, or DB
// reference outside prose), mints no session, keeps no replay cache,
// appends no audit event, and runs no finalize — the state creation,
// the issuer/JWKS verification, the group-to-role mapping, the
// session mint, and the allow/deny audit append all run inside the
// core handlers that serve every fetch.
//
// Rollback is revert-to-relay: re-point the two exact-path native
// registrations in routes() at d9PublicWriteThrough and delete this
// file with its tests; no data migrates and no other family moves.

const (
	// oidcLoginPath and oidcCallbackPath are the exact terminated
	// paths, mirrored from d9Routes (pinned equal by test): only
	// these two spellings terminate. Trailing-slash twins, deeper
	// subtrees, and non-GET spellings stay on the generic relay
	// behind RequireAuth.
	oidcLoginPath    = "/api/v0/auth/oidc/login"
	oidcCallbackPath = "/api/v0/auth/oidc/callback"
)

// oidcTermMaxBody bounds the mapped core answer: OIDC answers are a
// redirect (empty body) or a small session/error JSON, never near a
// megabyte. An unreadable or oversize body fails closed to the
// permanent local 503 instead of relaying a truncation.
const oidcTermMaxBody = 1 << 20

// oidcPortRequest is the dest-side seam behind the terminated OIDC
// GETs: the exact core route path, the opaque caller query
// (r.URL.RawQuery, forwarded byte-identical, never parsed), the
// caller headers (forwarded opaquely; dest invents none — the dest
// shared key is never injected on these public routes, and a caller
// key that happens to arrive crosses as an ordinary header,
// untouched and unjudged, exactly like the dispatch lane before it),
// and the guaranteed correlation ID.
type oidcPortRequest struct {
	path        string
	query       string
	header      http.Header
	correlation string
}

// oidcPortResponse is the terminated answer: the core status, the
// core response headers minus per-hop framing, and the core body
// bytes for synthesis.
type oidcPortResponse struct {
	status int
	header http.Header
	body   []byte
}

// oidcLoginPort is the dest-side seam behind the terminated OIDC
// pair: the StartOIDCLogin call the terminated login GET admits
// through and the CompleteOIDCLogin call the terminated callback GET
// admits through, over D0-proven transport. The names mirror the
// frozen OIDCLoginAdmission v0 method set so a re-signed core method
// fails the frozen test the same way; the shapes stay dest-owned and
// opaque (no core import) because dest transports caller bytes, it
// does not re-implement admission. Production satisfies it with a
// core-fetching adapter (one anonymous GET, no retry); tests drive
// the same handlers against stub cores.
type oidcLoginPort interface {
	// StartOIDCLogin admits the terminated login GET: one fetch of
	// the canonical core login route with the opaque caller query.
	StartOIDCLogin(ctx context.Context, req oidcPortRequest) (oidcPortResponse, error)
	// CompleteOIDCLogin admits the terminated callback GET: one
	// fetch of the canonical core callback route with the opaque
	// caller query. Exactly-once stays core-side (ConsumeState); dest
	// makes no second attempt and holds no lock.
	CompleteOIDCLogin(ctx context.Context, req oidcPortRequest) (oidcPortResponse, error)
}

// coreOIDCPort is the production oidcLoginPort: it fetches the
// canonical core OIDC routes over D0-proven transport. The adapter is
// request-scoped (built per call in the handlers, never shared), so
// nothing is cached: a second call would fetch again (callers make
// exactly one).
type coreOIDCPort struct {
	server *Server
}

// coreOIDCPort satisfies oidcLoginPort: the terminated GETs serve
// from this seam, so a re-signed dest port fails the build the same
// way a re-signed core method fails the frozen test.
var _ oidcLoginPort = (*coreOIDCPort)(nil)

// errOIDCTermFetch marks any port-fetch failure the terminated GETs
// map to the permanent local 503. It carries no core detail and no
// credential: a store error, an oversize body, or an unreachable core
// never reaches the response body.
var errOIDCTermFetch = errOIDCTermFetchType{}

type errOIDCTermFetchType struct{}

func (errOIDCTermFetchType) Error() string { return "eshu-api: oidc fetch failed" }

// StartOIDCLogin implements oidcLoginPort over D0-proven transport:
// one anonymous GET of the canonical core login route.
func (p *coreOIDCPort) StartOIDCLogin(ctx context.Context, req oidcPortRequest) (oidcPortResponse, error) {
	return p.server.fetchOIDCCore(ctx, req)
}

// CompleteOIDCLogin implements oidcLoginPort over D0-proven
// transport: one anonymous GET of the canonical core callback route.
// No retry, no lock: exactly-once is core's ConsumeState to judge.
func (p *coreOIDCPort) CompleteOIDCLogin(ctx context.Context, req oidcPortRequest) (oidcPortResponse, error) {
	return p.server.fetchOIDCCore(ctx, req)
}

// fetchOIDCCore performs the single port fetch: GET of the canonical
// core OIDC route with the caller's raw query string, the cloned
// caller headers, the guaranteed correlation ID, and W3C trace
// context when the caller sent it. No Authorization header is ever
// invented — a caller key that happens to arrive crosses as an
// ordinary header, untouched and unjudged — and no query value is
// parsed, logged, or telemetered. Dest makes exactly one upstream
// attempt on every shape, follows no redirect (the shared dispatch
// client answers ErrUseLastResponse), and maps an unreadable or
// oversize body to a fetch error: the caller fails closed
// downstream.
func (s *Server) fetchOIDCCore(ctx context.Context, req oidcPortRequest) (oidcPortResponse, error) {
	target := strings.TrimRight(s.coreBaseURL, "/") + req.path
	if req.query != "" {
		target += "?" + req.query
	}
	upstream, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return oidcPortResponse{}, errOIDCTermFetch
	}
	upstream.Header = req.header.Clone()
	stripHopHeaders(upstream.Header)
	if upstream.Header.Get(correlationHeader) == "" {
		upstream.Header.Set(correlationHeader, req.correlation)
	}
	upstream.Host = s.core.Host

	resp, err := s.httpClient.Do(upstream)
	if err != nil {
		return oidcPortResponse{}, errOIDCTermFetch
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, oidcTermMaxBody+1))
	if err != nil {
		return oidcPortResponse{}, errOIDCTermFetch
	}
	if len(raw) > oidcTermMaxBody {
		return oidcPortResponse{}, errOIDCTermFetch
	}
	mapped := make(http.Header, len(resp.Header))
	for key, values := range resp.Header {
		mapped.Del(key)
		for _, value := range values {
			mapped.Add(key, value)
		}
	}
	// Framing is per-hop: the body bytes cross untouched while Go sets
	// this hop's Content-Length itself, so a relayed length can never
	// disagree with the bytes written.
	mapped.Del("Content-Length")
	return oidcPortResponse{status: resp.StatusCode, header: mapped, body: raw}, nil
}

// oidcTerminatedHandler selects the terminated handler for one OIDC
// route: the login start leg, or for any other d9Routes entry the
// callback leg. Lives here (not in routes()) so the shared router
// stays under the file-size cap.
func (s *Server) oidcTerminatedHandler(route string) func(http.ResponseWriter, *http.Request) {
	if route == d9OIDCLogin {
		return s.serveOIDCLoginTerminated
	}
	return s.serveOIDCCallbackTerminated
}

// serveOIDCLoginTerminated terminates one OIDC login GET for any
// caller: the route is public pre-auth, so there is no gate and no
// authorizer judgment to file. The redirect comes from the
// StartOIDCLogin port; dest synthesizes the 302 with the core
// Location byte-verbatim. Every port failure is the permanent local
// 503 backend_unavailable — unconfigured origin, unreachable core,
// unreadable or oversize body — with exactly one upstream attempt and
// never a Retry-After. A core-marked retryable 503 keeps the
// Retry-After the core sent: mapped answers cross untouched and dest
// invents no headers.
func (s *Server) serveOIDCLoginTerminated(w http.ResponseWriter, r *http.Request) {
	port := &coreOIDCPort{server: s}
	s.serveOIDCTerminated(w, r, oidcLoginPath, port.StartOIDCLogin)
}

// serveOIDCCallbackTerminated terminates one OIDC callback GET for
// any caller: same public posture as the login leg. The session
// answer (201 JSON plus both Set-Cookie session cookies, or the 303
// with cookies plus Location) comes from the CompleteOIDCLogin port
// and synthesizes byte-verbatim; denied logins (403, unmapped groups
// or revoked grants) create no session because core created none.
// Port failures close to the permanent local 503 with exactly one
// upstream attempt and never a Retry-After.
func (s *Server) serveOIDCCallbackTerminated(w http.ResponseWriter, r *http.Request) {
	port := &coreOIDCPort{server: s}
	s.serveOIDCTerminated(w, r, oidcCallbackPath, port.CompleteOIDCLogin)
}

// serveOIDCTerminated is the shared terminated-GET engine: fail closed
// with no origin, make exactly one port call, file the native outcome
// with the core latency, and synthesize the mapped answer.
func (s *Server) serveOIDCTerminated(w http.ResponseWriter, r *http.Request, path string, call func(context.Context, oidcPortRequest) (oidcPortResponse, error)) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	req := oidcPortRequest{path: path, query: r.URL.RawQuery, header: r.Header, correlation: CorrelationID(r)}
	coreStart := time.Now()
	got, err := call(r.Context(), req)
	coreLatency := time.Since(coreStart)
	if err != nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, coreLatency, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	retryAfter := got.status == http.StatusServiceUnavailable && got.header.Get("Retry-After") != ""
	outcome := outcomeCoreOK
	switch {
	case retryAfter:
		outcome = outcomeCoreRetryable
	case got.status == http.StatusServiceUnavailable:
		outcome = outcomeCore503
	}
	s.dispatchOutcome(r, viaNative, outcome, coreLatency, retryAfter)
	for key, values := range got.header {
		w.Header().Del(key)
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(got.status)
	_, _ = w.Write(got.body)
}
