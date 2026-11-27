// Command coreserveconsumer is the N1 (issue #446) external-consumer proof
// for the shared auth serving library required by ADR-0002 section 2.
//
// It is a separate Go module so the main eshu-api module never imports the
// seam transitively: the proof imports ONLY the versioned shared library
// (github.com/eshu-hq/eshu/go/coreserve) and serves /readyz plus one auth
// read against fixture stores with zero core-origin HTTP calls.
//
// Status: RED. The go/coreserve library does not exist yet in eshu-hq/eshu,
// so `go test ./...` here fails resolving the import. Turning it green
// requires, in order: (1) the core-side extraction of the nested
// go/coreserve module (aliases for CoreServeDeps/CoreAuthWiring plus a
// delegating NewCoreServeMux), (2) tag go/coreserve/v0.1.0, (3) add that
// exact require to this module's go.mod. No replace directive may land in
// the committed go.mod. Production paths (cmd/eshu-api, api/auth.go, all
// api/*_terminate.go leaves) stay untouched until P5.
package coreserveconsumer
