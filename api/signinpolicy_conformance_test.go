// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/eshu-hq/eshu-api/openapi"
)

// Signinpolicy-group conformance (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#72). The D0 proof per group: dest calls the frozen
// SignInPolicyReadAdmission v0 port (one method, GetSignInPolicy,
// exact frozen signature) and takes no table or store handle; the pin
// is byte-identical at the fresh core SHA with check-parity green;
// the served spec carries both ops line-verbatim.

// frozenSignInPolicyPort pins the exact dest-side method set behind
// the versioned #9669 O6 admission contract for the signinpolicy
// group: the single GetSignInPolicy read the terminated public
// sign-in-policy GET and the policy half of the terminated providers
// GET admit through. It mirrors the upstream frozen list
// (contracts/upstream/sign_in_policy_read_admission_contract_test.go:
// "GetSignInPolicy func(context.Context, string)
// (auth.SignInPolicy, error)") in the dest-owned shape — same method
// name, same parameters, dest-owned return — so a renamed, added,
// removed, or re-signed method fails here until the list is
// deliberately updated. A renamed, added, removed, or re-signed
// method fails here until that list is deliberately updated; bump
// SignInPolicyReadAdmissionContractVersion alongside (manually — the
// bump is convention, the frozen list is the enforcement).
var frozenSignInPolicyPort = []string{
	"GetSignInPolicy func(context.Context, string) (api.signInPolicy, error)",
}

// signInPolicyPortWitness proves signInPolicyReadPort is satisfiable
// without a store: the core-fetching adapter satisfies it. No other
// fake in this package carries the read method.
var _ signInPolicyReadPort = (*coreSignInPolicyPort)(nil)

// signInPolicyPortMethodSet reflects the live signInPolicyReadPort
// method set into sorted "name signature" strings for comparison
// with the frozen list.
func signInPolicyPortMethodSet(t *testing.T) []string {
	t.Helper()
	rt := reflect.TypeOf((*signInPolicyReadPort)(nil)).Elem()
	var got []string
	for i := 0; i < rt.NumMethod(); i++ {
		m := rt.Method(i)
		got = append(got, m.Name+" "+m.Type.String())
	}
	sort.Strings(got)
	return got
}

// TestSignInPolicyPortIsFrozen pins the D0 port: the dest-side seam
// is exactly the one frozen read. The version constant stays v0
// (api/version.go, pinned against
// contracts/sign-in-policy-read-admission.md by version_test.go);
// the frozen method set below is the enforcement.
func TestSignInPolicyPortIsFrozen(t *testing.T) {
	t.Parallel()

	got := signInPolicyPortMethodSet(t)
	if len(got) != len(frozenSignInPolicyPort) {
		t.Fatalf("signInPolicyReadPort method set = %q (len %d), want frozen %q (len %d)",
			got, len(got), frozenSignInPolicyPort, len(frozenSignInPolicyPort))
	}
	for i := range got {
		if got[i] != frozenSignInPolicyPort[i] {
			t.Errorf("signInPolicyReadPort differs at index %d: got %q, want %q", i, got[i], frozenSignInPolicyPort[i])
		}
	}
	if SignInPolicyReadAdmissionContractVersion != "v0" {
		t.Errorf("SignInPolicyReadAdmissionContractVersion = %q, want %q: bump it deliberately with the shape change",
			SignInPolicyReadAdmissionContractVersion, "v0")
	}
}

// TestSigninpolicyGroup_SpecCarriesBothOps pins the OpenAPI
// assembly: the served document carries both terminated public ops
// (getPublicSignInPolicy, listAuthProviders) with the admin op
// alongside. Fragment line-verbatim rides scripts/check-parity.sh;
// this test pins the assembly around those bytes.
func TestSigninpolicyGroup_SpecCarriesBothOps(t *testing.T) {
	t.Parallel()

	doc := openapi.Spec()
	for _, want := range []string{
		`"operationId": "getPublicSignInPolicy"`,
		`"operationId": "listAuthProviders"`,
		`"operationId": "getAdminSignInPolicy"`,
		`"/api/v0/auth/sign-in-policy"`,
		`"/api/v0/auth/providers"`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("served spec lacks %s", want)
		}
	}
}

// TestSigninpolicyGroup_TerminatedFilesNoAuthorizer pins the
// telemetry extension without renames: the terminated public GETs
// file their family on the existing native lane with the closed
// outcome vocabulary and time the port fetches — and file no
// authorizer judgment, because the routes are public and there is no
// gate to judge.
func TestSigninpolicyGroup_TerminatedFilesNoAuthorizer(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/api/v0/auth/providers" {
			_, _ = io.WriteString(w, `{"providers":[],"local_login_offered":true,"self_service_tokens_offered":true}`)
			return
		}
		_, _ = io.WriteString(w, `{"require_sso":false}`)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	tel := NewTelemetry(nil)
	srv.WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{
		"/api/v0/auth/sign-in-policy?tenant_id=tenant_a",
		"/api/v0/auth/providers?tenant_id=tenant_a",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", path, rec.Code)
		}
	}

	snap := tel.snapshot()
	for _, family := range []string{"sign_in_policy", "auth_providers"} {
		if snap.dispatches[family]["native"]["core_ok"] != 1 {
			t.Errorf("dispatch rows = %v, want one %s/native/core_ok", snap.dispatches[family], family)
		}
		if _, ok := snap.authorizer[family]; ok {
			t.Errorf("authorizer rows = %v, want none for public %s (no gate to judge)", snap.authorizer[family], family)
		}
		if got := len(snap.coreLatencies["sign_in_policy"]["native"].buckets) + len(snap.coreLatencies["auth_providers"]["native"].buckets); got == 0 {
			t.Errorf("no native core-latency histogram, want the port fetches timed")
		}
	}
}

// TestSigninpolicyGroup_SynthesizedKeysAreExact pins the wire shape:
// the synthesized bodies carry exactly the contracted keys — no
// credential, no tenant echo, no extra core keys leak through.
func TestSigninpolicyGroup_SynthesizedKeysAreExact(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/api/v0/auth/providers" {
			_, _ = io.WriteString(w, `{"providers":[{"provider_config_id":"prov_1","display_label":"Single sign-on (OIDC)","provider_kind":"oidc","icon_hint":"oidc","extra":"leak"}],"local_login_offered":true,"self_service_tokens_offered":true,"tenant_id":"tenant_a"}`)
			return
		}
		_, _ = io.WriteString(w, `{"require_sso":true,"tenant_id":"tenant_a"}`)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/sign-in-policy?tenant_id=tenant_a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != `{"require_sso":true}` {
		t.Errorf("sign-in-policy body = %q, want exactly the contracted key", body)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v0/auth/providers?tenant_id=tenant_a", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("providers status = %d, want 200", rec.Code)
	}
	var got map[string]any
	raw, _ := io.ReadAll(rec.Result().Body)
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("providers decode %q: %v", raw, err)
	}
	for _, want := range []string{"providers", "local_login_offered", "self_service_tokens_offered"} {
		if _, ok := got[want]; !ok {
			t.Errorf("providers body lacks key %q: %q", want, raw)
		}
	}
	if len(got) != 3 {
		t.Errorf("providers body has %d keys, want exactly 3: %q", len(got), raw)
	}
	if _, ok := got["tenant_id"]; ok {
		t.Errorf("providers body echoes tenant_id, want no tenant echo: %q", raw)
	}
}
