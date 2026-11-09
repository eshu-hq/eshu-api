# Reference: configuring eshu-api

There's not much to set, and that's intentional. Two flags, two env
vars, and you're done. The one you'll trip over if you forget it is
`--core-base-url`: without it the process refuses to start. For a
walkthrough, see [Auth setup](../guides/auth-setup.md) and
[Getting started](../getting-started/README.md).

| Setting | Required | Default | What it does |
| --- | --- | --- | --- |
| `--addr` | No | `:8080` | Address and port to listen on |
| `--core-base-url` | Yes | none | Core read-API origin, for example `https://core:8443` |
| `ESHU_API_KEY` (env) | Yes, in practice | empty | Shared Bearer token checked on gated `/api/*` routes |
| `ESHU_API_V0_SUNSET_DATE` (env) | No | `Thu, 01 Jul 2027 00:00:00 GMT` | Date sent in the `Sunset` header on `/api/v0/*` replies |

A quick example:

```bash
ESHU_API_KEY='change-me' /tmp/eshu-api \
  --addr :8080 \
  --core-base-url http://localhost:8443
```

`ESHU_API_V0_SUNSET_DATE` has to be an RFC 1123 date. If it doesn't
parse, we quietly fall back to the default above rather than failing
the boot over a header.

## About the API key

The key travels by env only. There's no flag for it, and that's on
purpose: flags show up in process listings, env values don't. Callers
send the same value back as `Authorization: Bearer <key>`.

If you leave `ESHU_API_KEY` unset, the service still starts, but every
gated `/api/*` call gets a `401`. Empty means fail closed, never fail
open.

## Which routes need a token, and which don't

Here's the part people get wrong, so let's be precise. This page is
the home of the public-route carve-outs — other pages summarise and
link here.

Most `/api/*` routes need a valid Bearer token, and a missing or wrong
token gets a `401`. But not all of them do. Nineteen `/api/*` routes
register outside the auth gate in `api/router.go`, because they run
before a caller can have a credential: first-run setup, signing in,
and the pages that help you sign in. They are:

- Setup wizard (4): `GET /api/v0/auth/setup-state`,
  `POST /api/v0/auth/setup/claim`, `POST /api/v0/auth/setup/admin`,
  `POST /api/v0/auth/setup/mfa`
- Local session, pre-auth (4): `POST /api/v0/auth/local/login`,
  `POST /api/v0/auth/local/invitations/accept`,
  `POST /api/v0/auth/local/break-glass/session`,
  `POST /api/v0/auth/local/password/rotate`
- OIDC sign-in (2): `GET /api/v0/auth/oidc/login`,
  `GET /api/v0/auth/oidc/callback`
- GitHub sign-in (2): `GET /api/v0/auth/github/login`,
  `GET /api/v0/auth/github/callback`
- SAML sign-in (3):
  `GET /api/v0/auth/saml/providers/{provider_id}/metadata`,
  `GET /api/v0/auth/saml/providers/{provider_id}/login`,
  `POST /api/v0/auth/saml/providers/{provider_id}/acs`
- Docs and login picker (4): `GET /api/v0/docs`, `GET /api/v0/redoc`,
  `GET /api/v0/auth/providers`, `GET /api/v0/auth/sign-in-policy`

Public here means "no shared-key check at this layer", not "no checks
at all". Credential bodies and query values cross to the core as
opaque bytes, the shared key is never injected on these routes, and
the core still decides who gets in (including sealing setup with a
`410` once an identity exists). Method matters too: a non-GET spelling
of a public GET keeps the usual `401`, and trailing-slash spellings
mostly stay on the gated relay. If you're auditing the surface, read
the registration comments in `api/router.go` alongside this list.
They're the source of truth.

Everything else under `/api/*` sits behind the gate, including
`GET /api/v0/openapi.json`. The core serves that document publicly; we
don't add a new public surface for it here.

Outside `/api/*`, `GET /health`, `GET /readyz`, and `GET /metrics` are
public and answered locally. They never disclose the key. `/health`
does disclose the configured core origin, so keep that in mind if you
expose probes more widely than your cluster.

Foreign credential classes (a scoped token, an OIDC bearer, a session
cookie) get a middle path on several native read lanes: instead of
being judged here, they're relayed so the core can judge them, while
credential-less callers still get the standing `401`. You don't need to
configure any of that. It follows from the credential you present.
