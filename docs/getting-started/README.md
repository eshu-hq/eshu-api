# Getting started with eshu-api

Fastest path from nothing to an answered call. You'll need Go 1.26.10
(see `go.mod`) and a running Eshu core to point at — this repo is only
the HTTP layer, so without a core there's nobody home behind the front
desk.

## 1. Build and run

```bash
go build -o /tmp/eshu-api ./cmd/eshu-api
ESHU_API_KEY='change-me' /tmp/eshu-api --core-base-url http://localhost:8443
```

Swap in your own core address. `--core-base-url` is required: leave it
off and the process refuses to start. Pick a real key, too — with
`ESHU_API_KEY` empty, every gated `/api/*` call answers `401`. Empty
means fail closed, never fail open.

## 2. Check it's alive

Both probes work without a token:

```bash
curl -s http://localhost:8080/health
curl -s http://localhost:8080/readyz
```

`/readyz` answers
`{"service":"eshu-api","probe":"readyz","status":"ok"}` and `/health`
lists the contract versions this build serves.

## 3. Make your first gated call

Send the same key back as a Bearer token:

```bash
curl -s -H 'Authorization: Bearer change-me' \
  http://localhost:8080/api/v0/openapi.json | head -c 200
```

A JSON document means you're through the gate. Most `/api/*` calls need
that token — though a handful of setup, login, and docs routes stay
public by design, because they run before anyone has a credential. The
full list lives in
[Reference: configuration](../reference/configuration.md), and
[Set up auth](../guides/auth-setup.md) walks through it step by step.

## Where next?

- [Run it day to day](../guides/operate.md) — image or binary, health, logs.
- [Concepts](../concepts/README.md) — what we own and how a request travels.
- [Diagrams](../diagrams/README.md) — the same story as pictures.
- [Troubleshoot](../guides/troubleshoot.md) — when step 1 or 3 goes sideways.
