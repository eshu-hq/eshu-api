// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// logRequest emits the single structured line per dispatch request.
// The credential never appears: only method, closed labels, status,
// latencies, outcome, and the correlation ID.
func (t *Telemetry) logRequest(r *http.Request, family, via, correlationID string, rec *dispatchRecord, status int, duration time.Duration) {
	if t.logger == nil {
		return
	}
	outcome, coreLatency, retryAfter := "", time.Duration(0), false
	failure := ""
	if rec != nil {
		outcome, coreLatency, retryAfter = rec.outcome, rec.coreLatency, rec.retryAfter
		failure = rec.failure
	}
	t.logger.Info("api request",
		"method", r.Method,
		"family", family,
		"via", via,
		"status", status,
		"status_class", statusClass(status),
		"outcome", outcome,
		"failure", failure,
		"duration_ms", duration.Milliseconds(),
		"core_latency_ms", coreLatency.Milliseconds(),
		"retry_after", retryAfter,
		"correlation_id", correlationID,
	)
}

// dispatchRecordKey carries the per-request dispatch record through
// the request context from the instrumenting middleware to the
// dispatch step, so one structured log line can hold the lane, the
// outcome, and both latencies.
type dispatchRecordKey struct{}

// dispatchRecord is the per-request dispatch note the middleware reads
// after next returns. outcome stays empty when no dispatch step ran
// (the auth gate denied first). failure carries the closed relay
// failure reason (connection_refused, timeout, other, unconfigured);
// it stays empty on success and on lanes that terminate without a
// relay failure. A mid-copy abort files timeout or other beside the
// already-filed core outcome, since the core status went out first.
type dispatchRecord struct {
	via         string
	outcome     string
	failure     string
	coreLatency time.Duration
	retryAfter  bool
}

// dispatchNote returns the record the middleware attached, or nil when
// the handler ran outside the instrumented chain (direct unit calls).
func dispatchNote(r *http.Request) *dispatchRecord {
	rec, _ := r.Context().Value(dispatchRecordKey{}).(*dispatchRecord)
	return rec
}

// statusWriter captures the status next writes so the middleware can
// file it without touching the relayed bytes.
type statusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader records the status and forwards it untouched.
func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Flush forwards a flush to the underlying writer when it supports
// it. The statusWriter embeds only http.ResponseWriter, whose method
// set lacks Flush and Hijack, so without these forwarders the relay's
// mid-body flush and abort would silently no-op behind this writer.
func (w *statusWriter) Flush() {
	if fw, ok := w.ResponseWriter.(http.Flusher); ok {
		fw.Flush()
	}
}

// Hijack forwards a hijack to the underlying writer so the relay can
// abort a truncated mid-body response on HTTP/1.x connections.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.New("api: underlying writer does not support hijacking")
}

// instrumented wraps one dispatch handler with the telemetry chain:
// echo-or-mint the correlation ID on the response, file the request
// row and duration, file the dispatch row when a dispatch step ran,
// and emit the single structured log line. 401s from the auth gate
// inside next still cross this middleware, so gate denials count per
// family too. Probes never pass through here.
func (s *Server) instrumented(via string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tel := s.telemetry
		if tel == nil {
			next.ServeHTTP(w, r)
			return
		}
		family := classifyFamily(r.URL.Path)
		correlationID := CorrelationID(r)
		w.Header().Set(correlationHeader, correlationID)
		rec := &dispatchRecord{via: via}
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r.WithContext(context.WithValue(r.Context(), dispatchRecordKey{}, rec)))
		duration := time.Since(start)
		tel.observeRequest(family, statusClass(sw.status), duration)
		// The lane comes from the record, not the wrap: a gate inside
		// next (the D2 wave gate) may flip the record lane when the
		// request relays explicitly instead of terminating. The wrap
		// value seeds the record at creation, so every path without a
		// gate still files the wrap lane unchanged.
		if rec.outcome != "" {
			tel.observeDispatch(family, rec.via, rec.outcome, rec.coreLatency)
		}
		tel.logRequest(r, family, rec.via, correlationID, rec, sw.status, duration)
	})
}
