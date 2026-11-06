// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// backendUnavailableCode is the stable machine-readable code this
// server answers when it cannot reach the core. It reuses the versioned
// in-tree vocabulary value "backend_unavailable"; the message states the
// dispatch status so no caller mistakes it for a live backend outage.
// The local 503 is permanent: it never carries Retry-After, matching the
// in-tree rule that a permanent backend_unavailable never tells a client
// to retry. A core-marked retryable 503 keeps the Retry-After the core
// sent: dispatch relays core bytes untouched and never invents headers.
const backendUnavailableCode = "backend_unavailable"

// dispatchTimeout bounds one proxied core call, including the body
// relay. It matches the versioned read-contract client bound
// (readclient.DefaultTimeout, pinned equal by test): a slower core
// answers 503/504 on its own side rather than holding an eshu-api
// worker past this bound.
const dispatchTimeout = 30 * time.Second

// hopHeaders are the hop-by-hop headers dispatch strips before
// forwarding. They describe this hop's transport, not the route, so
// they must never reach the core.
var hopHeaders = []string{ // static table, reviewed with the relay below
	"Connection",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
	"Keep-Alive",
}

// stripHopHeaders removes the hop-by-hop headers from the forwarded
// set, including any header the request's Connection token list names.
func stripHopHeaders(header http.Header) {
	tokens := strings.Split(header.Get("Connection"), ",")
	header.Del("Connection")
	for _, name := range tokens {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			header.Del(trimmed)
		}
	}
	for _, name := range hopHeaders {
		header.Del(name)
	}
}

// dispatch proxies one authenticated /api/* call to the configured core
// origin and relays the core's answer byte-for-byte: status, headers,
// and body. It is the whole core-route move at the HTTP layer, so every
// route family in go/cmd/api/CUTOVER_OWNERSHIP.md (reads and mutations,
// newRouter- and wireAPI-mounted) crosses here with its wire contract
// intact — status codes, envelope shapes, and error codes included. The
// v1 alias rewrite already ran ahead of the auth gate, so the core sees
// the rewritten /api/v0/* path; the v0 deprecation headers were set by
// the outer middleware and stay on the relayed response.
//
// Dispatch owns exactly one synthetic error: when the core cannot be
// reached or the origin is not configured, it answers the permanent 503
// backend_unavailable in the caller's negotiated shape. Every byte the
// core does answer crosses unmodified.
func (s *Server) dispatch(w http.ResponseWriter, r *http.Request) {
	if s.core == nil {
		s.dispatchOutcome(r, viaRelay, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	forwardToCore(s, w, r, viaRelay)
}

// dispatchOutcome files one dispatch step. Inside the instrumented
// chain it notes the outcome on the middleware record (which files the
// row and the log line after next returns); on a direct call with no
// record it files the row immediately. Either way the family comes
// from the request path and the labels stay in the closed vocabulary.
func (s *Server) dispatchOutcome(r *http.Request, via, outcome string, coreLatency time.Duration, retryAfter bool) {
	if rec := dispatchNote(r); rec != nil {
		rec.outcome = outcome
		rec.coreLatency = coreLatency
		rec.retryAfter = retryAfter
		return
	}
	if s.telemetry != nil {
		s.telemetry.observeDispatch(classifyFamily(r.URL.Path), via, outcome, coreLatency)
	}
}

// forwardToCore proxies one call to the configured core origin and
// relays the core's answer byte-for-byte: status, headers, and body. It
// owns exactly one synthetic error: when the core cannot be reached it
// answers the permanent 503 backend_unavailable. A core-marked retryable
// 503 keeps the Retry-After the core sent: relayed core bytes cross
// unmodified and this function never invents headers.
//
// Trace context crosses untouched: traceparent/tracestate are not
// hop-by-hop headers, so the clone carries them. The correlation ID is
// guaranteed upstream: the caller's value when supplied, the minted one
// otherwise. Dest invents no truth levels or error codes here.
//
// The dispatch step notes its outcome and core latency on the telemetry
// record the middleware attached (nil outside the instrumented chain,
// e.g. direct unit calls) without changing the relayed bytes.
func forwardToCore(s *Server, w http.ResponseWriter, r *http.Request, via string) {
	fail := func(message string) {
		s.dispatchOutcome(r, via, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode, message)
	}
	target := strings.TrimRight(s.coreBaseURL, "/") + r.URL.RequestURI()
	upstream, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		fail("eshu-api: core is unavailable")
		return
	}
	upstream.Header = r.Header.Clone()
	stripHopHeaders(upstream.Header)
	if upstream.Header.Get(correlationHeader) == "" {
		upstream.Header.Set(correlationHeader, CorrelationID(r))
	}
	upstream.Host = s.core.Host

	coreStart := time.Now()
	resp, err := s.httpClient.Do(upstream)
	if err != nil {
		fail("eshu-api: core is unavailable")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	relayed := w.Header()
	for key, values := range resp.Header {
		relayed.Del(key)
		for _, value := range values {
			relayed.Add(key, value)
		}
	}
	// Framing is per-hop: the body bytes cross untouched while Go sets
	// this hop's Content-Length itself, so a relayed length can never
	// disagree with the bytes written.
	relayed.Del("Content-Length")
	retryAfter := resp.StatusCode == http.StatusServiceUnavailable &&
		resp.Header.Get("Retry-After") != ""
	outcome := outcomeCoreOK
	switch {
	case retryAfter:
		outcome = outcomeCoreRetryable
	case resp.StatusCode == http.StatusServiceUnavailable:
		outcome = outcomeCore503
	}
	s.dispatchOutcome(r, via, outcome, time.Since(coreStart), retryAfter)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// parseCoreOrigin resolves the configured core origin to its host for
// outbound dispatch. An empty or unparsable origin resolves to nil and
// dispatch fails closed with the local 503.
func parseCoreOrigin(coreBaseURL string) *url.URL {
	if coreBaseURL == "" {
		return nil
	}
	parsed, err := url.Parse(coreBaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil
	}
	return parsed
}

// dispatchClient builds the outbound client: bounded by dispatchTimeout
// and never following the core's redirects, so a 3xx relays as a 3xx.
func dispatchClient() *http.Client {
	return &http.Client{
		Timeout: dispatchTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
