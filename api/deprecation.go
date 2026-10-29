// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"strings"
	"time"
)

// DefaultV0Sunset is the Sunset response header date for /api/v0/ routes
// when ESHU_API_V0_SUNSET_DATE is unset or invalid. It matches the in-tree
// default so the extracted path advertises the same deprecation horizon.
const DefaultV0Sunset = "Thu, 01 Jul 2027 00:00:00 GMT"

// v0SunsetEnvVar is the only environment input to the deprecation date.
const v0SunsetEnvVar = "ESHU_API_V0_SUNSET_DATE"

// V0SunsetFromEnv returns the date for the Sunset response header. A set
// variable must validate as RFC 1123 (http.TimeFormat); a parse failure
// falls back to the default. Ported behavior from readSunsetDate in
// go/cmd/api/wiring_router.go in eshu.
func V0SunsetFromEnv(getenv func(string) string) string {
	if v := getenv(v0SunsetEnvVar); v != "" {
		if _, err := time.Parse(time.RFC1123, v); err == nil {
			return v
		}
	}
	return DefaultV0Sunset
}

// deprecationHeadersMiddleware adds Deprecation: true and Sunset: <date>
// headers to every response whose request path starts with /api/v0/. It is
// applied BEFORE the v1-to-v0 rewrite middleware so /api/v1/ requests
// never carry the headers. Headers are set before the next handler runs,
// so they appear on error responses too.
//
// Ported behavior from go/cmd/api/wiring_router.go in eshu.
func deprecationHeadersMiddleware(next http.Handler, sunset string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v0/") {
			w.Header().Set("Deprecation", "true")
			w.Header().Set("Sunset", sunset)
		}
		next.ServeHTTP(w, r)
	})
}
