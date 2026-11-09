# Set up auth for eshu-api

One shared key guards most of `/api/*`. This guide sets it up and
explains the routes that stay public on purpose. For the exact,
route-by-route classification, use
[Reference: configuration](../reference/configuration.md) — that's the
page that names every public `/api/*` route.

## Step 1: give the service a key

The key travels by env only. There's no flag for it, and that's on
purpose: flags show up in process listings, env values don't.

```bash
ESHU_API_KEY='change-me' /tmp/eshu-api \
  --addr :8080 \
  --core-base-url http://localhost:8443
```

## Step 2: send the key with each call

Callers send the same value back as `Authorization: Bearer <key>`:

```bash
curl -s -H 'Authorization: Bearer change-me' \
  http://localhost:8080/api/v0/openapi.json | head -c 200
```

Your token crosses to the core only in the `Authorization` header. It
never shows up in logs or health output.

## Step 3: know what "no key" means

If you leave `ESHU_API_KEY` unset, the service still starts, but every
gated `/api/*` call gets a `401`. Empty means fail closed, never fail
open. So if everything under `/api/*` says `401`, check the env first.
Nine times out of ten that's it.

## The routes that stay public (and why)

Most `/api/*` routes need a valid Bearer token. A short list runs
outside the auth gate because it has to happen *before* a caller can
have a credential: the first-run setup wizard, local login and
invitation acceptance, OIDC, GitHub, and SAML sign-in, plus the docs
pages and the data behind the login picker.

Public here means "no shared-key check at this layer", not "no checks
at all". The core still makes every trust call on those routes. The
authoritative list — all nineteen routes, with method and
trailing-slash behaviour — lives in
[Reference: configuration](../reference/configuration.md). If you're
auditing the surface, read the registration comments in `api/router.go`
alongside it. They're the source of truth.

Everything else under `/api/*` sits behind the gate, including
`GET /api/v0/openapi.json`.

## Probes

Outside `/api/*`, `GET /health`, `GET /readyz`, and `GET /metrics` are
public and answered locally. They never disclose the key. `/health`
does disclose the configured core origin, so keep that in mind if you
expose probes more widely than your cluster.

Stuck on a `401` that shouldn't be one? See
[Troubleshoot](troubleshoot.md).
