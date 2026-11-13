# Sign-in policy admission contract

Versioned admission-contract port for the admin sign-in policy mutation
at `PATCH /api/v0/auth/admin/sign-in-policy` (Refs
`eshu-hq/eshu#8872`). The API handler keeps the HTTP shape below and
calls this port; the core-owned Postgres store satisfies it. Canonical
mutations persist through explicit core admission; `eshu-api` never takes
a direct table handle.

## Version

`SignInPolicyAdmissionContractVersion = "v0"`, with the port defined as:

```go
type SignInPolicyAdmission = SignInPolicyMutationStore
```

Both lines are vendored verbatim in
`contracts/upstream/sign-in-wire-shapes.txt`, with the surrounding
contract comment and the partial-update request struct. The frozen method
set is `frozenSignInPolicyAdmission` in
`contracts/upstream/sign_in_policy_admission_contract_test.go`
(1 method: `UpsertSignInPolicy`).

## HTTP shape

The operator-facing PATCH body and the policy response, vendored
verbatim in `contracts/upstream/sign-in-wire-shapes.txt`:

- Request: partial update — `RequireSSO`, `AllowLocalUserCreation`,
  `RequireMFAForAllUsers`, `IdleTimeoutSeconds`,
  `AbsoluteTimeoutSeconds`. A nil field is left unchanged.
- Guardrails (checked atomically inside the store's row-locked
  transaction): `require_sso` needs a provider config with a passing
  connection test and an admin with SSO proof; a non-zero absolute
  timeout must not be shorter than a non-zero idle timeout. A breach
  answers 400.
- Response: HTTP 200 with the stored policy detail.
- Auth: all-scope admin authentication with an admin tenant scope;
  otherwise 403.
- Unconfigured handler (nil or typed-nil store unwrapped to nil): 503.

## Enforcement

`TestSignInPolicyAdmissionContractIsVersioned` pins the version and the
frozen method set; the wiring identity test pins the exact injected
port. A breaking change to the admission shape fails the frozen test
until the list is deliberately updated; bump
`SignInPolicyAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.

## Native termination (W7 writers: terminated writer)

The admin sign-in policy PATCH is the W7-writers terminated-writer
lane (Refs `eshu-hq/eshu-api#114`, core writer record
`eshu-hq/eshu#10531`): `api/signin.go` registers the explicit route
behind the family credential-class gate (`signInPolicyWriterGate`)
ahead of the generic proxy and forwards to the same core route with
method, path, query, headers (including the original `Authorization`
header), and body relayed; core bytes come back unmodified with
exactly one forward attempt and no retry, and core re-judges every
forwarded call as backstop. A valid shared key terminates natively
with the AllScopes family authorizer
(`TestSignInPolicyNative_AuthorizerRejectsNonAllScopes` pins the
helper, `TestSignInPolicyWriter_TerminatesAheadOfRelay` pins the
native-lane filing); a non-key Bearer or session-cookie-shaped
Cookie relays explicitly on the relay lane so core judges the class
dest cannot resolve (`TestSignInPolicyWriterGate_ForeignCredentialRelaysExplicit`);
a credential-less or junk-cookie caller keeps the standing dest 401
with zero core contact. Every dest-local answer files its outcome on
the native lane (`local_503` on unconfigured origin, `denied` on the
unreachable deny branch), and the family authorizer judgment counts
on both branches. Dest holds the AllScopes family authorizer — the
single trust root shared with the W7-read GET (see
`api/signinpolicy_admin_terminate.go`) — and synthesizes nothing:
no 401/403/404/500 for relayed classes, no Retry-After on the
permanent local 503. Both spellings plus the v1 alias ride the
family gate (`TestSignInPolicyNative_BothSpellingsWriteThrough`,
`TestSignInPolicyNative_V1AliasOrderingUnchanged`).

Scope honesty: dest holds the AllScopes authorizer per the dispatch
ruling, but with no scoped/session resolver the deny branch is
unreachable on the live route until one lands — the live 403s on this
route are core's, which re-judges all-scope admin, admin tenant scope,
and the `identity_admin.sign_in_policy_update` permission catalog as
backstop. Core is the sole writer. The deny-shape test pins the
contract for the resolver-landing day, not a live path. Exactly-once
needs no dest machinery here: the PATCH is the core's atomic
row-locked upsert, so a retried PATCH lands exactly as it would
against the core directly — dest forwards twice and relays both
core answers (`TestSignInPolicyNative_WriteThroughRelaysCoreBytes`,
`TestSignInPolicyNative_ParallelProbe`).

Audit parity: governance audit stays core-side on forwarded requests.
Unlike suppression, the in-tree sign-in mutation handler at the
provenance pin DOES emit audit — `EventTypeIDPConfigChange` for the
allowed outcome and for every denied outcome, including guardrail
rejections — and it emits from inside the core handler that still runs
on every forwarded request, so write-through keeps every audit event
core-side with no gap. Dest emits none. A family whose handler emits
audit must never be re-implemented dest-side without a versioned
audit-append contract; this lane re-implements nothing.
`scripts/check-parity.sh` gates the lane with the no-core-import proof
(`go list -deps` over the runtime packages must show no `eshu-hq/eshu`
import) alongside the family-403/store-503 wire-shape excerpt.

OpenAPI: native since this lane (satellite-design Decision 5(c),
overriding the `docs/parity.md` deferral for this family). The
fragment moved row-for-row from the pinned core commit `428790ab` —
`go/internal/query/openapi/paths/auth/sign_in_policy.go` into dest
`openapi/paths/auth/sign_in_policy.go`, byte-identical (operationIds
`getPublicSignInPolicy`, `getAdminSignInPolicy`,
`updateAdminSignInPolicy` preserved; `scripts/check-parity.sh` gates
the identity). The dest assembly (`openapi/spec.go`, minimal shared
block in `openapi/components.json`: the two family schemas plus
`ErrorResponse` and the `Forbidden`/`InternalError`/`ServiceUnavailable`
responses the fragment references) serves `GET /api/v0/openapi.json`
from compiled constants ahead of the generic proxy — no core import,
no core fetch, served even with no core origin configured — inside
`RequireAuth` like every dest `/api/*` route. `TestSpec_*` pin the
family section, lossless assembly, and local-$ref resolution;
`TestOpenAPINative_*` pin the served route, the v1 alias landing, and
the key gate.

Scope honesty: the dest document serves exactly the landed families
(sign-in-policy first). Other families' entries are absent until their
lanes port their fragments — this endpoint no longer relays core's full
spec. Core serves the spec public; dest keeps it behind the shared key,
adding no new public surface in this lane.

Rollback: additive. Revert this lane's dest commit (or re-pin the prior
digest) and the PATCH falls back to the generic byte-relay with the
same wire contract — no core change, no migration, no flag.
