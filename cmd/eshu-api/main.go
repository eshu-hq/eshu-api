// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/eshu-hq/eshu-api/api"
)

// apiKeyEnvVar is the only credential input: the shared-key bearer value
// the auth gate compares against. It travels by environment so the key
// never appears in process flags; an unset variable fails closed and
// every /api/* request 401s until the operator sets it.
const apiKeyEnvVar = "ESHU_API_KEY"

// server holds the dependencies of the dispatch surface. It answers
// health and readiness itself, gates /api/* behind the shared key, and
// dispatches authenticated calls to the core origin. It carries no
// credential at rest and takes no core handles.

type server struct {
	coreBaseURL string
	apiKey      string
	log         *slog.Logger
}

func newServer(coreBaseURL, apiKey string, log *slog.Logger) *server {
	return &server{coreBaseURL: coreBaseURL, apiKey: apiKey, log: log}
}

// handler returns the production middleware chain over the dispatch
// routes, with the sunset date read from the environment. The request
// log flows through log, so binary output carries the per-request
// structured lines.
func (s *server) handler() http.Handler {
	return api.NewServer(s.coreBaseURL, s.apiKey).WithTelemetry(api.NewTelemetry(s.log)).Handler(api.V0SunsetFromEnv(os.Getenv))
}

func run(args []string, logOut *slog.Logger) error {
	flags := flag.NewFlagSet("eshu-api", flag.ContinueOnError)
	addr := flags.String("addr", ":8080", "listen address for the dispatch surface")
	coreBaseURL := flags.String("core-base-url", "", "core read-API origin, for example https://core:8443")
	if err := flags.Parse(args); err != nil {
		return err
	}
	// The key travels by environment only, never by flag, so it stays out
	// of the process table. Empty fails closed at the auth gate.
	key := os.Getenv(apiKeyEnvVar)
	if *coreBaseURL == "" {
		return fmt.Errorf("eshu-api: --core-base-url is required")
	}
	srv := newServer(*coreBaseURL, key, logOut)
	logOut.Info("eshu-api serving",
		"core_read_contracts", api.CoreReadContractsVersion,
		"admission_contracts", api.SuppressionAdmissionContractVersion,
		"addr", *addr,
	)
	return buildServer(*addr, srv.handler()).ListenAndServe()
}

// Server timeouts. ReadHeaderTimeout stays the first bound a slow
// client meets; ReadTimeout then bounds the full request read so a
// dripping body cannot hold a connection indefinitely. WriteTimeout
// clears the 30 s relay bound (api.dispatchTimeout) with room for the
// body copy, so it never cuts a relay that is still within its own
// budget — it only bounds a genuinely stuck write. IdleTimeout
// recycles idle keep-alive connections.
const (
	serverReadHeaderTimeout = 5 * time.Second
	serverReadTimeout       = 10 * time.Second
	serverWriteTimeout      = 45 * time.Second
	serverIdleTimeout       = 60 * time.Second
)

// buildServer assembles the production HTTP server: the dispatch
// chain with every connection bound set, so no slow client can hold
// a worker indefinitely.
func buildServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(os.Args[1:], logger); err != nil {
		logger.Error("eshu-api exited", "error", err)
		os.Exit(1)
	}
}
