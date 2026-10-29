// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"strings"
)

// v1PrefixAliasMiddleware rewrites /api/v1/* request paths to /api/v0/*
// before passing to next. It is applied ahead of the auth gate so scoped
// downstream classification sees the rewritten v0 path, not the raw v1
// path. The request is cloned; method, headers, query parameters, and body
// are preserved.
//
// Ported behavior from go/cmd/api/wiring_router.go in eshu: the alias is
// a pure path rewrite, so this copy must stay byte-for-byte behavioral
// with the in-tree middleware until the in-tree implementation is removed
// after production acceptance.
func v1PrefixAliasMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/") {
			r2 := r.Clone(r.Context())
			u := *r.URL
			u.Path = "/api/v0/" + r.URL.Path[len("/api/v1/"):]
			r2.URL = &u
			next.ServeHTTP(w, r2)
			return
		}
		next.ServeHTTP(w, r)
	})
}
