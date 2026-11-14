// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
)

// Relay failure reasons: the closed vocabulary for details.reason on
// the synthesized local 503 and for the failure log field. Refused
// and timeout stay distinct so callers and operators can tell a dead
// core from a slow one; other covers the rest (canceled callers,
// unbuildable requests, mid-copy short closes); unconfigured marks
// the missing-origin fail-closed path.
const (
	reasonConnectionRefused = "connection_refused"
	reasonTimeout           = "timeout"
	reasonOther             = "other"
	reasonUnconfigured      = "unconfigured"
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
		noteRelayFailure(r, reasonUnconfigured)
		writeBackendUnavailable(w, r,
			"eshu-api: core origin is not configured; no core routes are served",
			reasonUnconfigured)
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

// writeBackendUnavailable writes the permanent 503 backend_unavailable
// in the caller's negotiated shape with the closed failure reason in
// details.reason (connection_refused, timeout, other, unconfigured).
// It never sets Retry-After: the local 503 is permanent. Relayed core
// bytes never pass through here.
func writeBackendUnavailable(w http.ResponseWriter, r *http.Request, message, reason string) {
	details := map[string]string{"reason": reason}
	if acceptsEnvelope(r) {
		writeJSON(w, http.StatusServiceUnavailable, envelope{
			Error: &envelopeError{
				Code:          backendUnavailableCode,
				Message:       message,
				CorrelationID: CorrelationID(r),
				Details:       details,
			},
		})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, errorBody{
		ErrorCode:     backendUnavailableCode,
		Message:       message,
		CorrelationID: CorrelationID(r),
		Details:       details,
	})
}

// noteRelayFailure records the closed failure reason on the
// middleware record so the structured log line tells refused apart
// from timeout. It leaves the outcome and metrics untouched.
func noteRelayFailure(r *http.Request, reason string) {
	if rec := dispatchNote(r); rec != nil {
		rec.failure = reason
	}
}

// dispatchFailureReason maps a pre-header dispatch error to the closed
// reason vocabulary: refused connections stay apart from timeouts so
// callers and operators can tell a dead core from a slow one.
// Anything else (canceled callers, unbuildable requests, mid-copy
// short closes) is other.
func dispatchFailureReason(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return reasonTimeout
		}
		err = urlErr.Err
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return reasonTimeout
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return reasonConnectionRefused
	}
	lowered := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lowered, "connection refused"):
		return reasonConnectionRefused
	case strings.Contains(lowered, "deadline exceeded"),
		strings.Contains(lowered, "timeout"),
		strings.Contains(lowered, "time out"):
		return reasonTimeout
	default:
		return reasonOther
	}
}

// copyBodyFlushing relays src to dst in chunks, flushing after every
// write when the writer allows it. Bytes cross untouched; flushing
// only changes when the caller sees them, never what it sees.
func copyBodyFlushing(dst http.ResponseWriter, src io.Reader) (int64, error) {
	fw, _ := dst.(http.Flusher)
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			written, writeErr := dst.Write(buf[:n])
			total += int64(written)
			if fw != nil {
				fw.Flush()
			}
			if writeErr != nil {
				return total, writeErr
			}
			if written != n {
				return total, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return total, nil
			}
			return total, readErr
		}
	}
}

// abortRelayCopy aborts a relayed response whose headers are already
// sent, so the caller sees a transport error instead of a clean but
// truncated end. It hijacks and closes the connection when the writer
// allows it (HTTP/1.x, the relay's serving path, promoted through the
// telemetry writer); otherwise it panics with http.ErrAbortHandler,
// which the server turns into an aborted connection.
func abortRelayCopy(w http.ResponseWriter) {
	if hj, ok := w.(http.Hijacker); ok {
		if conn, _, err := hj.Hijack(); err == nil {
			_ = conn.Close()
			return
		}
	}
	panic(http.ErrAbortHandler)
}

// forwardToCore proxies one call to the configured core origin and
// relays the core's answer byte-for-byte: status, headers, and body. It
// owns exactly one synthetic error: when the core cannot be reached it
// answers the permanent 503 backend_unavailable with the closed failure
// reason in details.reason. A core-marked retryable 503 keeps the
// Retry-After the core sent: relayed core bytes cross unmodified and
// this function never invents headers. A core that fails mid-body
// aborts the connection so the caller sees a transport error instead
// of a clean truncated end.
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
	fail := func(message, reason string) {
		s.dispatchOutcome(r, via, outcomeLocal503, 0, false)
		noteRelayFailure(r, reason)
		writeBackendUnavailable(w, r, message, reason)
	}
	target := strings.TrimRight(s.coreBaseURL, "/") + r.URL.RequestURI()
	upstream, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		fail("eshu-api: core is unavailable", reasonOther)
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
		reason := dispatchFailureReason(err)
		fail("eshu-api: core is unavailable", reason)
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
	// Flush the headers (and every body chunk below) promptly: a core
	// that stalls mid-body must leave the caller holding the already
	// written status and partial bytes, so the later abort reads as a
	// truncated body with a transport error instead of a failed call.
	if fw, ok := w.(http.Flusher); ok {
		fw.Flush()
	}
	if _, err := copyBodyFlushing(w, resp.Body); err != nil {
		// The status is already sent, so this cannot become a 503:
		// note the failure for the log line, log the copy error for
		// the operator, and abort the connection so the caller sees
		// a transport error instead of truncated JSON with a clean
		// end. The credential never appears in the log.
		reason := dispatchFailureReason(err)
		noteRelayFailure(r, reason)
		if tel := s.telemetry; tel != nil && tel.logger != nil {
			tel.logger.Error("relay body copy failed",
				"family", classifyFamily(r.URL.Path),
				"via", via,
				"status", resp.StatusCode,
				"outcome", outcome,
				"failure", reason,
				"correlation_id", CorrelationID(r),
				"error", err.Error(),
			)
		}
		abortRelayCopy(w)
	}
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
