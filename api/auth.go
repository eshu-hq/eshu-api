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

// Local-error reasons: the closed details.reason vocabulary for
// errors this server synthesizes itself. The relay-lane 503 reasons
// (connection_refused, timeout, other, unconfigured) live in
// dispatch.go and are shared, never redefined, so the probe body, the
// readiness metric, and the relay lane read as one vocabulary. The
// two gate reasons below complete the set: every local 401 names
// authentication_required and every local 403 names forbidden,
// uniformly, so callers get a reason beside the code.
const (
	reasonAuthenticationRequired = "authentication_required"
	reasonForbidden              = "forbidden"
)

// localErrorDetails maps one locally synthesized error to its closed
// details reason, or nil when the shape stays bare. 403 always names
// forbidden. 503 with the backend_unavailable code names unconfigured
// when the message says the origin is not configured (the fail-closed
// shape every native lane shares) and other otherwise — the remaining
// local 503s (an oversize ACS form, a failed terminated-lane core
// fetch) are genuinely not configuration states, so they must not
// borrow that reason. Every other status keeps the historical bare
// shape, and relayed core bytes never reach this mapping.
func localErrorDetails(status int, code, message string) map[string]string {
	switch status {
	case http.StatusForbidden:
		return map[string]string{"reason": reasonForbidden}
	case http.StatusServiceUnavailable:
		if code != backendUnavailableCode {
			return nil
		}
		if strings.Contains(message, "not configured") {
			return map[string]string{"reason": reasonUnconfigured}
		}
		return map[string]string{"reason": reasonOther}
	default:
		return nil
	}
}

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
// Details carries the closed reason vocabulary on locally synthesized
// errors (see localErrorDetails), including the relay-lane 503
// reasons (see dispatch.go); it stays absent otherwise so success,
// relayed core bytes, and unrelated error bytes never change.
type errorBody struct {
	ErrorCode     string            `json:"error_code"`
	Message       string            `json:"message"`
	CorrelationID string            `json:"correlation_id"`
	Details       map[string]string `json:"details,omitempty"`
}

// envelopeError is the error section of the negotiated Eshu envelope.
// Details mirrors the plain shape: present on locally synthesized
// errors, absent everywhere else.
type envelopeError struct {
	Code          string            `json:"code"`
	Message       string            `json:"message"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	Details       map[string]string `json:"details,omitempty"`
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
// gets the plain error_code shape. Both shapes name the closed
// authentication_required reason in details. The reason stays uniform
// across the missing-credential, wrong-credential, and
// unconfigured-server paths on purpose: distinguishing them would leak
// server configuration state to unauthenticated callers. Ported
// behavior from querycontract.WriteUnauthorized in eshu.
func WriteUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	correlationID := CorrelationID(r)
	details := map[string]string{"reason": reasonAuthenticationRequired}
	if acceptsEnvelope(r) {
		writeJSON(w, http.StatusUnauthorized, envelope{
			Error: &envelopeError{
				Code:          unauthenticatedCode,
				Message:       "authentication is required",
				CorrelationID: correlationID,
				Details:       details,
			},
		})
		return
	}
	writeJSON(w, http.StatusUnauthorized, errorBody{
		ErrorCode:     unauthenticatedCode,
		Message:       "authentication is required",
		CorrelationID: correlationID,
		Details:       details,
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

// RequireAuth gates next behind the shared-key bearer credential.
// Every request without a valid Bearer [REDACTED] denied with
// WriteUnauthorized. An empty configured key fails closed. The gate
// carries no public-path exceptions: probes stay public because the
// router registers them outside the gate, not because the gate admits
// them, so every request the gate sees is an authenticated-route request.
//
// The skeleton carries the shared-key source only. Scoped-token and OIDC
// resolvers land with core dispatch; the gate stays closed until then.
func RequireAuth(apiKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
