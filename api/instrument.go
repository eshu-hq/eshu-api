// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
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
	if rec != nil {
		outcome, coreLatency, retryAfter = rec.outcome, rec.coreLatency, rec.retryAfter
	}
	t.logger.Info("api request",
		"method", r.Method,
		"family", family,
		"via", via,
		"status", status,
		"status_class", statusClass(status),
		"outcome", outcome,
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
// (the auth gate denied first).
type dispatchRecord struct {
	via         string
	outcome     string
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
		if rec.outcome != "" {
			tel.observeDispatch(family, via, rec.outcome, rec.coreLatency)
		}
		tel.logRequest(r, family, via, correlationID, rec, sw.status, duration)
	})
}
