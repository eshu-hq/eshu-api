// Package readclient calls versioned core read contracts over HTTP.
//
// It offers one bounded client for versioned read-contract use: every
// call targets a route under /api/v0/ on the configured core origin,
// carries the bearer key only in the Authorization header (never
// logged), and is bounded by the pinned timeout and response byte
// budget below.
//
// It is one bounded client among several routes to core truth: the
// dispatch relay (api/dispatch.go) proxies /api/* over its own bounded
// client so core bytes cross byte-for-byte, and the native lanes reach
// core without this package — no production code imports it today.
// The relay bound stays pinned equal to DefaultTimeout by test, so
// every lane shares the same ceiling whether or not it dials through
// here.
package readclient
