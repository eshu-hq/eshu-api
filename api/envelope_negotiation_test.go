// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// decodeNegotiationBody decodes an already-read body into the generic
// map plus the sorted top-level key set. Envelope-shape tests elsewhere
// read the recorder once via decodeEnvelope; these tests already hold
// the raw bytes for the byte-identity assertion, so they decode from
// the same bytes instead of re-reading the recorder.
func decodeNegotiationBody(t *testing.T, body []byte) (map[string]any, []string) {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}
	keys := make([]string, 0, len(decoded))
	for k := range decoded {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return decoded, keys
}

// Envelope negotiation on success reads (Refs eshu-hq/eshu-api#66, part
// of eshu-hq/eshu#9669). The edge never synthesizes the negotiated
// {data, truth} envelope: success shape is core-owned, so the edge
// forwards the caller Accept verbatim and relays the core bytes
// verbatim. Where core renders flat (its WriteJSON admin reads and the
// setup-state handler), the edge serves flat even to an
// envelope-negotiated caller — flat-with-embedded-truth (result_limits,
// pagination, group fields) is the deliberate edge contract there, and
// no caller data is lost. Where core negotiates (its WriteSuccess
// routes), the envelope flows through untouched. Dest-synthesized
// errors stay envelope-aware via writeError: negotiation applies where
// dest owns the bytes.

// TestEnvelopeNegotiation_RelaysCoreEnvelope pins the passthrough: a
// stub core answering the negotiated {data, truth} envelope reaches an
// envelope-negotiated caller byte-identical on both the terminated lane
// (a wave-gated D3 read) and the generic relay, with the caller Accept
// forwarded upstream.
func TestEnvelopeNegotiation_RelaysCoreEnvelope(t *testing.T) {
	t.Parallel()

	const envelopeBody = `{"data":{"items":[]},"error":null,"truth":{"profile":"full"}}`
	for _, route := range []string{
		"/api/v0/auth/admin/roles", // terminated D3 lane
		"/api/v0/repositories",     // generic relay lane
	} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: envelopeBody}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := authedRequest(http.MethodGet, route)
			req.Header.Set("Accept", EnvelopeMIME)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want the core's 200", rec.Code)
			}
			if core.lastAccept != EnvelopeMIME {
				t.Errorf("upstream Accept = %q, want the caller negotiation", core.lastAccept)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != envelopeBody {
				t.Errorf("body = %q, want the core envelope bytes %q", body, envelopeBody)
			}
			decoded, keys := decodeNegotiationBody(t, body)
			if !slices.Equal(keys, []string{"data", "error", "truth"}) {
				t.Fatalf("top-level keys = %v, want [data error truth]", keys)
			}
			if decoded["data"] == nil || decoded["truth"] == nil {
				t.Errorf("envelope = %v, want core data and truth relayed", decoded)
			}
		})
	}
}

// TestEnvelopeNegotiation_FlatStaysFlat pins the #66 contract: a stub
// core answering the flat shape core's WriteJSON admin reads serve
// (inline result_limits truth, no envelope) reaches even an
// envelope-negotiated caller flat and byte-identical. The edge forwards
// the negotiation but never wraps: wrapping would diverge from core and
// invent truth dest cannot build.
func TestEnvelopeNegotiation_FlatStaysFlat(t *testing.T) {
	t.Parallel()

	const flatBody = `{"items":[],"result_limits":{"limit":50,"truncated":false},"truncated":false}`
	for _, route := range []string{
		"/api/v0/auth/admin/roles", // terminated D3 lane
		"/api/v0/repositories",     // generic relay lane
	} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: flatBody}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := authedRequest(http.MethodGet, route)
			req.Header.Set("Accept", EnvelopeMIME)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want the core's 200", rec.Code)
			}
			if core.lastAccept != EnvelopeMIME {
				t.Errorf("upstream Accept = %q, want the caller negotiation forwarded", core.lastAccept)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != flatBody {
				t.Errorf("body = %q, want the flat core bytes %q (no edge wrap)", body, flatBody)
			}
			decoded, keys := decodeNegotiationBody(t, body)
			if slices.Contains(keys, "data") {
				t.Errorf("top-level keys = %v, want no edge-synthesized envelope, decoded %v", keys, decoded)
			}
		})
	}
}

// TestEnvelopeNegotiation_PlainCallerKeepsPlainShape pins the other
// half of the matrix: without the envelope Accept the core bytes cross
// untouched too, so a plain caller never sees an envelope the edge (or
// the stub core) did not send.
func TestEnvelopeNegotiation_PlainCallerKeepsPlainShape(t *testing.T) {
	t.Parallel()

	const flatBody = `{"items":[],"result_limits":{"limit":50,"truncated":false}}`
	core := &stubCore{status: http.StatusOK, body: flatBody}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, "/api/v0/auth/admin/roles")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want the core's 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != flatBody {
		t.Errorf("body = %q, want the core bytes %q", body, flatBody)
	}
}

// TestEnvelopeNegotiation_SetupStateFlatBothShapes pins the terminated
// setup-state GET: dest synthesizes the flat two-key shape exactly like
// the core handleState it replaces (WriteJSON, no negotiation), in both
// negotiated shapes, while still forwarding the caller Accept to the
// port fetch.
func TestEnvelopeNegotiation_SetupStateFlatBothShapes(t *testing.T) {
	t.Parallel()

	const coreBody = `{"needs_setup":true,"bootstrap_mode":"local"}`
	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		t.Run(accept, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: coreBody}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/setup-state", nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("Accept %q: status = %d, want terminated 200", accept, rec.Code)
			}
			if core.lastAccept != accept {
				t.Errorf("Accept %q: upstream Accept = %q, want the caller negotiation", accept, core.lastAccept)
			}
			decoded := decodeBody(t, rec)
			if decoded["needs_setup"] != true {
				t.Errorf("Accept %q: needs_setup = %v, want true", accept, decoded["needs_setup"])
			}
			if decoded["bootstrap_mode"] != "local" {
				t.Errorf("Accept %q: bootstrap_mode = %v, want local", accept, decoded["bootstrap_mode"])
			}
			if _, wrapped := decoded["data"]; wrapped {
				t.Errorf("Accept %q: body = %v, want the flat two-key shape (no edge wrap)", accept, decoded)
			}
		})
	}
}

// TestEnvelopeNegotiation_DestErrorsNegotiate pins that negotiation
// applies where dest owns the bytes: the fail-closed local 503 answers
// the envelope error section to an envelope caller and the plain
// error_code shape otherwise.
func TestEnvelopeNegotiation_DestErrorsNegotiate(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, "/api/v0/auth/admin/roles")
	req.Header.Set("Accept", EnvelopeMIME)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want the local 503", rec.Code)
	}
	decoded, keys := decodeEnvelope(t, rec)
	if !slices.Equal(keys, []string{"data", "error", "truth"}) {
		t.Fatalf("top-level keys = %v, want [data error truth]", keys)
	}
	errSection, ok := decoded["error"].(map[string]any)
	if !ok {
		t.Fatalf("body has no error section: %v", decoded)
	}
	if errSection["code"] != backendUnavailableCode {
		t.Errorf("error.code = %v, want %q", errSection["code"], backendUnavailableCode)
	}

	plain := authedRequest(http.MethodGet, "/api/v0/auth/admin/roles")
	plainRec := httptest.NewRecorder()
	handler.ServeHTTP(plainRec, plain)

	if plainRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("plain status = %d, want the local 503", plainRec.Code)
	}
	plainDecoded := decodeBody(t, plainRec)
	if plainDecoded["error_code"] != backendUnavailableCode {
		t.Errorf("error_code = %v, want %q", plainDecoded["error_code"], backendUnavailableCode)
	}
}
