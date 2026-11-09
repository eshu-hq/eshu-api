// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

// SAML-triple native termination (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#72, saml-triple lane, last of the sequential
// termination waves; lands only after the github-pair merge per the D9
// order). GET /api/v0/auth/saml/providers/{provider_id}/metadata,
// GET .../login, and POST .../acs terminate dest-side as exact-path
// native-public routes (outside every gate, ahead of the generic
// proxy): each makes exactly one anonymous core fetch over D0-proven
// transport, and dest synthesizes the answer from the port result —
// metadata XML bytes verbatim, login redirect status plus Location
// byte-verbatim (SigAlg and Signature intact), ACS 201/303 with
// Set-Cookie name/value/attrs/expiry verbatim, error envelopes
// verbatim — with no dest retry on any leg. The metadata and login
// legs are census reads that create server-side state; the ACS POST
// is the census mutation that spends the request exactly once, so a
// second dest attempt would double-spend it. OIDC and GitHub stay on
// their own landed termination lanes; this file never names them.
//
// Opaque bytes (binding): the provider path value, the return_to
// query value, and the IdP-posted form fields cross as opaque bytes —
// this file decodes none of them (no query decoding, no form
// decoding, no path variable extraction: the request path crosses
// whole via EscapedPath, and the ACS form crosses as bounded bytes,
// never decoded), logs none of them, and records no RequestURI,
// Location, or Set-Cookie value in
// logs, metrics, or spans (the middleware logs method, family, via,
// status, and latencies only). The ACS mutation is never retried and
// never serialized dest-side: a parallel duplicate ACS makes one
// fetch per delivery and core's consume-plus-reservation chain judges
// exactly-once (one session, loser core error).
//
// Login-state and session stores stay core-side: this file takes no
// table or store handle anywhere (no sql, store, table, or DB
// reference outside prose), mints no session, keeps no replay cache,
// holds no provider material (no store lookup, no verifier, no key
// material of any kind — the SP key material never leaves core),
// appends no audit
// event, and runs no finalize — the request creation and RelayState
// hashing, the signed-assertion verification, the replay reservation,
// the group-to-role mapping, the session mint, and the allow/deny
// audit append all run inside the core handlers that serve every
// fetch.
//
// Rollback is revert-to-relay: re-point the three exact-path native
// registrations in routes() at d9SAMLPublicWriteThrough (restore the
// deleted method from this lane's diff) and delete this file with its
// tests; no data migrates and no other family moves.

// samlTermMaxBody bounds the mapped core answer and the forwarded ACS
// form: metadata answers are small XML, login answers are redirects
// (empty body), and ACS answers are a small session/error JSON; core
// itself caps the ACS form at one mebibyte, so dest mirrors that cap
// instead of truncating. An unreadable or oversize body fails closed
// to the permanent local 503 instead of relaying a truncation — with
// zero upstream contact on the oversize ACS shape.
const samlTermMaxBody = 1 << 20

// samlPortRequest is the dest-side seam behind the terminated SAML
// triple: the exact core route path (the caller EscapedPath,
// forwarded whole and never split into segments), the caller method
// (GET, HEAD on the GET legs, POST on ACS — forwarded for core to
// judge; dest adds no HEAD branch), the opaque caller query
// (r.URL.RawQuery, forwarded byte-identical, never parsed),
// the caller headers (forwarded opaquely; dest invents none — the
// dest shared key is never injected on these public routes, and a
// caller key that happens to arrive crosses as an ordinary header,
// untouched and unjudged, exactly like the dispatch lane before it),
// the ACS form bytes (POST only; bounded, never decoded), and the
// guaranteed correlation ID.
type samlPortRequest struct {
	method      string
	path        string
	query       string
	header      http.Header
	body        []byte
	correlation string
}

// samlPortResponse is the terminated answer: the core status, the
// core response headers minus per-hop framing, and the core body
// bytes for synthesis.
type samlPortResponse struct {
	status int
	header http.Header
	body   []byte
}

// samlLoginPort is the dest-side seam behind the terminated SAML
// triple: the FetchSAMLMetadata call the terminated metadata GET
// admits through, the StartSAMLLogin call the terminated login GET
// admits through, and the CompleteSAMLLogin call the terminated ACS
// POST admits through, over D0-proven transport. The login and ACS
// names mirror the OIDC/GitHub Start/Complete convention; the
// metadata call deliberately does not mirror the store lookup name
// (that method returns the SP key material that must never leave
// core, so the check-parity.sh negative grep fails any sighting of it
// in this file). The shapes stay dest-owned and opaque (no core
// import) because dest transports caller bytes, it does not
// re-implement admission. Production satisfies it with a
// core-fetching adapter (one anonymous fetch per leg, no retry);
// tests drive the same handlers against stub cores.
type samlLoginPort interface {
	// FetchSAMLMetadata admits the terminated metadata GET: one fetch
	// of the canonical core metadata route with the opaque caller
	// path. Exactly-once is vacuous here (a read), but the single
	// attempt still holds: a retried metadata read is a second
	// server-side state creation.
	FetchSAMLMetadata(ctx context.Context, req samlPortRequest) (samlPortResponse, error)
	// StartSAMLLogin admits the terminated login GET: one fetch of
	// the canonical core login route with the opaque caller query.
	StartSAMLLogin(ctx context.Context, req samlPortRequest) (samlPortResponse, error)
	// CompleteSAMLLogin admits the terminated ACS POST: one fetch of
	// the canonical core ACS route with the opaque caller form.
	// Exactly-once stays core-side (consume plus replay reservation);
	// dest makes no second attempt and holds no lock.
	CompleteSAMLLogin(ctx context.Context, req samlPortRequest) (samlPortResponse, error)
}

// coreSAMLPort is the production samlLoginPort: it fetches the
// canonical core SAML routes over D0-proven transport. The adapter is
// request-scoped (built per call in the handlers, never shared), so
// nothing is cached: a second call would fetch again (callers make
// exactly one).
type coreSAMLPort struct {
	server *Server
}

// coreSAMLPort satisfies samlLoginPort: the terminated legs serve from
// this seam, so a re-signed dest port fails the build the same way a
// re-signed core method fails the frozen test.
var _ samlLoginPort = (*coreSAMLPort)(nil)

// errSAMLTermFetch marks any port-fetch failure the terminated legs
// map to the permanent local 503. It carries no core detail and no
// credential: a store error, an oversize body, or an unreachable core
// never reaches the response body.
var errSAMLTermFetch = errSAMLTermFetchType{}

type errSAMLTermFetchType struct{}

func (errSAMLTermFetchType) Error() string { return "eshu-api: saml fetch failed" }

// FetchSAMLMetadata implements samlLoginPort over D0-proven
// transport: one anonymous fetch of the canonical core metadata route.
func (p *coreSAMLPort) FetchSAMLMetadata(ctx context.Context, req samlPortRequest) (samlPortResponse, error) {
	return p.server.fetchSAMLCore(ctx, req)
}

// StartSAMLLogin implements samlLoginPort over D0-proven transport:
// one anonymous fetch of the canonical core login route.
func (p *coreSAMLPort) StartSAMLLogin(ctx context.Context, req samlPortRequest) (samlPortResponse, error) {
	return p.server.fetchSAMLCore(ctx, req)
}

// CompleteSAMLLogin implements samlLoginPort over D0-proven
// transport: one anonymous fetch of the canonical core ACS route. No
// retry, no lock: exactly-once is core's consume-plus-reservation
// chain to judge.
func (p *coreSAMLPort) CompleteSAMLLogin(ctx context.Context, req samlPortRequest) (samlPortResponse, error) {
	return p.server.fetchSAMLCore(ctx, req)
}

// fetchSAMLCore performs the single port fetch: the caller method to
// the canonical core SAML route (the caller EscapedPath, whole and
// never segmented), the caller raw query string, the cloned caller
// headers, the guaranteed correlation ID, and W3C trace context when
// the caller sent it. No Authorization header is ever invented — a
// caller key that happens to arrive crosses as an ordinary header,
// untouched and unjudged — and no path value, query value, or form
// field is parsed, logged, or telemetered. The ACS form crosses as
// bounded bytes (read by the handler, never decoded). Dest makes
// exactly one upstream attempt on every shape, follows no redirect
// (the shared dispatch client answers ErrUseLastResponse), and maps
// an unreadable or oversize body to a fetch failure: the caller fails
// closed downstream.
func (s *Server) fetchSAMLCore(ctx context.Context, req samlPortRequest) (samlPortResponse, error) {
	target := strings.TrimRight(s.coreBaseURL, "/") + req.path
	if req.query != "" {
		target += "?" + req.query
	}
	var body io.Reader
	if req.body != nil {
		body = bytes.NewReader(req.body)
	}
	upstream, err := http.NewRequestWithContext(ctx, req.method, target, body)
	if err != nil {
		return samlPortResponse{}, errSAMLTermFetch
	}
	upstream.Header = req.header.Clone()
	stripHopHeaders(upstream.Header)
	if upstream.Header.Get(correlationHeader) == "" {
		upstream.Header.Set(correlationHeader, req.correlation)
	}
	upstream.Host = s.core.Host

	resp, err := s.httpClient.Do(upstream)
	if err != nil {
		return samlPortResponse{}, errSAMLTermFetch
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, samlTermMaxBody+1))
	if err != nil {
		return samlPortResponse{}, errSAMLTermFetch
	}
	if len(raw) > samlTermMaxBody {
		return samlPortResponse{}, errSAMLTermFetch
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
	return samlPortResponse{status: resp.StatusCode, header: mapped, body: raw}, nil
}

// samlTerminatedHandler selects the terminated handler for one SAML
// route: the metadata leg, the login start leg, or for the remaining
// d9SAMLRoutes entry the ACS leg. Lives here (not in routes()) so the
// shared router stays under the file-size cap.
func (s *Server) samlTerminatedHandler(route string) func(http.ResponseWriter, *http.Request) {
	switch route {
	case d9SAMLMetadata:
		return s.serveSAMLMetadataTerminated
	case d9SAMLLogin:
		return s.serveSAMLLoginTerminated
	default:
		return s.serveSAMLACSTerminated
	}
}

// serveSAMLMetadataTerminated terminates one SAML metadata GET for
// any caller: the route is public pre-auth, so there is no gate and
// no authorizer judgment to file. The metadata XML comes from the
// FetchSAMLMetadata port; dest synthesizes the 200 with the core
// Content-Type and body byte-verbatim. Every port failure is the
// permanent local 503 backend_unavailable — unconfigured origin,
// unreachable core, unreadable or oversize body — with exactly one
// upstream attempt and never a Retry-After. A core-marked retryable
// 503 keeps the Retry-After the core sent: mapped answers cross
// untouched and dest invents no headers.
func (s *Server) serveSAMLMetadataTerminated(w http.ResponseWriter, r *http.Request) {
	port := &coreSAMLPort{server: s}
	s.serveSAMLTerminated(w, r, nil, port.FetchSAMLMetadata)
}

// serveSAMLLoginTerminated terminates one SAML login GET for any
// caller: same public posture as the metadata leg. The redirect comes
// from the StartSAMLLogin port; dest synthesizes the 302 with the
// core Location byte-verbatim (SigAlg and Signature intact). Port
// failures close to the permanent local 503 with exactly one upstream
// attempt and never a Retry-After.
func (s *Server) serveSAMLLoginTerminated(w http.ResponseWriter, r *http.Request) {
	port := &coreSAMLPort{server: s}
	s.serveSAMLTerminated(w, r, nil, port.StartSAMLLogin)
}

// serveSAMLACSTerminated terminates one SAML ACS POST for any caller:
// same public posture as the GET legs. The IdP-posted form crosses as
// bounded bytes (read here, never decoded): an unreadable or oversize
// form fails closed to the permanent local 503 with zero upstream
// contact. The session answer (201 JSON plus both Set-Cookie session
// cookies, or the 303 with cookies plus Location) comes from the
// CompleteSAMLLogin port and synthesizes byte-verbatim; denied ACS
// deliveries (400 missing fields, 401 spent or unknown request, 403
// denied principal) create no session because core created none. Port
// failures close to the permanent local 503 with exactly one upstream
// attempt and never a Retry-After.
func (s *Server) serveSAMLACSTerminated(w http.ResponseWriter, r *http.Request) {
	form, err := io.ReadAll(io.LimitReader(r.Body, samlTermMaxBody+1))
	if err != nil || len(form) > samlTermMaxBody {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core fetch failed; no core routes are served")
		return
	}
	port := &coreSAMLPort{server: s}
	s.serveSAMLTerminated(w, r, form, port.CompleteSAMLLogin)
}

// serveSAMLTerminated is the shared terminated-leg engine: fail closed
// with no origin, make exactly one port call, file the native outcome
// with the core latency, and synthesize the mapped answer. The caller
// path crosses whole (EscapedPath, never segmented), the caller method
// crosses for core to judge, and the ACS form (nil on the GET legs)
// crosses as bounded bytes.
func (s *Server) serveSAMLTerminated(w http.ResponseWriter, r *http.Request, form []byte, call func(context.Context, samlPortRequest) (samlPortResponse, error)) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	req := samlPortRequest{method: r.Method, path: r.URL.EscapedPath(), query: r.URL.RawQuery, header: r.Header, body: form, correlation: CorrelationID(r)}
	coreStart := time.Now()
	got, err := call(r.Context(), req)
	coreLatency := time.Since(coreStart)
	if err != nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, coreLatency, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core fetch failed; no core routes are served")
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
