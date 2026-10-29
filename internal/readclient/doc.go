// Package readclient calls versioned core read contracts over HTTP.
//
// It is the only path this repository takes to core truth: no core
// implementation package is imported anywhere in this tree, and no
// shared-table handle crosses this boundary. Every call targets a route
// under /api/v0/ on the configured core origin, carries the bearer key
// only in the Authorization header (never logged), and is bounded by the
// pinned timeout and response byte budget below.
package readclient
