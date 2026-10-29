// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// EnvelopeMIME selects the stable Eshu response envelope. It matches
// querycontract.EnvelopeMIMEType in eshu: a caller that sends it gets the
// envelope error shape, everyone else gets the plain error_code shape.
const EnvelopeMIME = "application/eshu.envelope+json"

// unauthenticatedCode is the stable machine-readable code every 401 from
// this server carries. It matches querycontract.ErrorCodeUnauthenticated
// ("unauthenticated") so the extracted path keeps the live wire value.
const unauthenticatedCode = "unauthenticated"

// CorrelationID returns the request's correlation ID: the
// X-Correlation-ID or X-Request-ID header when present, or a random
// 16-byte hex identifier otherwise (falling back to a nanosecond
// timestamp if the random read itself fails). Ported behavior from
// querycontract.DocumentationCorrelationID in eshu.
func CorrelationID(r *http.Request) string {
	for _, header := range []string{"X-Correlation-ID", "X-Request-ID"} {
		if value := strings.TrimSpace(r.Header.Get(header)); value != "" {
			return value
		}
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	}
	return hex.EncodeToString(raw[:])
}

// acceptsEnvelope reports whether a request negotiated the Eshu envelope.
func acceptsEnvelope(r *http.Request) bool {
	return r != nil && strings.Contains(r.Header.Get("Accept"), EnvelopeMIME)
}

// errorBody is the plain (non-envelope) error shape served to callers
// that did not negotiate the envelope. Keyed to match the live
// unauthenticated response: error_code, message, correlation_id.
type errorBody struct {
	ErrorCode     string `json:"error_code"`
	Message       string `json:"message"`
	CorrelationID string `json:"correlation_id"`
}

// envelopeError is the error section of the negotiated Eshu envelope.
type envelopeError struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// envelope is the negotiated Eshu response envelope for errors.
type envelope struct {
	Data  any            `json:"data"`
	Truth any            `json:"truth"`
	Error *envelopeError `json:"error"`
}

// writeJSON writes one JSON response with the shared content type.
func writeJSON(w http.ResponseWriter, status int, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

// WriteUnauthorized writes a 401 JSON error response. Its
// WWW-Authenticate header is the bare "Bearer" challenge. Callers that
// negotiated the envelope get the envelope error shape; everyone else
// gets the plain error_code shape. Ported behavior from
// querycontract.WriteUnauthorized in eshu.
func WriteUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	correlationID := CorrelationID(r)
	if acceptsEnvelope(r) {
		writeJSON(w, http.StatusUnauthorized, envelope{
			Error: &envelopeError{
				Code:          unauthenticatedCode,
				Message:       "authentication is required",
				CorrelationID: correlationID,
			},
		})
		return
	}
	writeJSON(w, http.StatusUnauthorized, errorBody{
		ErrorCode:     unauthenticatedCode,
		Message:       "authentication is required",
		CorrelationID: correlationID,
	})
}

// bearerKey extracts the shared-key bearer credential from r. It reports
// false when the Authorization header is absent or is not a Bearer [REDACTED]
// The key itself never leaves this function except by comparison.
func bearerKey(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	key := strings.TrimSpace(strings.TrimPrefix(value, prefix))
	if key == "" {
		return "", false
	}
	return key, true
}

// RequireAuth gates next behind the shared-key bearer credential. The
// health and readiness probes pass without a credential; every /api/*
// request without a valid Bearer [REDACTED] denied with WriteUnauthorized.
// An empty configured key fails closed: every /api/* request is denied.
//
// The skeleton carries the shared-key source only. Scoped-token and OIDC
// resolvers land with core dispatch; the gate stays closed until then.
func RequireAuth(apiKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		if apiKey == "" {
			WriteUnauthorized(w, r)
			return
		}
		got, ok := bearerKey(r)
		// Constant-time comparison: the key is a bearer secret, so even a
		// network-impractical timing signal is not worth emitting.
		if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(apiKey)) != 1 {
			WriteUnauthorized(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
