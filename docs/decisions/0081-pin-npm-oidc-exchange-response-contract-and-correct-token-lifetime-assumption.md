---
parent: Decisions
nav_order: 81
status: accepted
date: 12026-08-14
decision-makers: Yunseo Kim
relations:
  - type: amends
    target: ADR-0076
    scope:
      "the npm OIDC exchange success-response contract (unpinned in ADR-0076; pinned in ADR-0081 to
      the empirically observed shape) and the exchange-token lifetime assumption (the 'typically 1
      hour' documentation note cited in ADR-0076; the observed lifetime is 15 minutes)"
  - type: see-also
    target: ADR-0082
---

# Pin the npm OIDC Exchange Response Contract to the Observed Shape and Correct the Token Lifetime Assumption

## Context and Problem Statement

ADR 0076 established the early npm OIDC token exchange preflight: before signing and before any
publish mutation, the profile performs the non-mutating trusted-publisher token exchange
(`POST /-/npm/v1/oidc/token/exchange/package/{package}`) so that trusted-publishing misconfiguration
is detected before registry mutation. That ADR pinned the endpoint, the failure status
classification (401/404 versus 5xx/malformed/unreadable), and the safety argument that the exchange
mutates nothing — but it did **not** pin the success-response body contract, and neither does the
JS/TS npm provenance and publish specification. The P03 implementation therefore wrote a strict
decoder against the npm API documentation example: exactly four members (`token_type`, `token`,
`created`, `expires`), RFC 3339 timestamp strings, unknown-member rejection, and semantic validation
(`expires` after `created` and after the exchange time).

On 12026-08-13 the second live M1 dogfood run — the first retry, following the 12026-08-12 first
attempt that failed at build-and-pack and never reached this path —
([vers-js run 31737001312](https://github.com/windlasstech/vers-js/actions/runs/31737001312))
exercised this path against the real registry for the first time. The exchange returned **HTTP 201**
— the trusted-publisher configuration is valid and a publish token was minted — but the strict
decoder rejected the response body, and the pipeline failed closed with
`windlass.verify.error.npm-oidc-exchange-indeterminate` ("npm OIDC exchange returned a malformed
response"). No token fallback was attempted and no registry mutation occurred; the publish job's
mutation steps were all skipped for want of the provenance-bundle handoff. This is the ADR 0076
confirmation event: the early-exchange preflight was empirically exercised, classified an
unreadable-by-contract surface, and blocked mutation exactly as designed.

The npm official API documentation
([npm Registry API, OIDC: Exchange OIDC id_token for npm registry token](https://api-docs.npmjs.com/#tag/OIDC/operation/exchangeOidcToken),
verified 12026-08-14) documents the operation as follows:

- `POST https://registry.npmjs.org/-/npm/v1/oidc/token/exchange/package/{package_name}`, where the
  path parameter is the URL-encoded package name.
- The Bearer token must be an OIDC `id_token` from a supported identity provider, and its `aud`
  claim must be `npm:registry.npmjs.org`.
- Documented response statuses: `201` success, `400`, `401`, `404`, `500`.
- The documented `201` response sample is:

  ```json
  {
    "token_type": "oidc",
    "token": "string",
    "created": "2025-07-18T10:30:00.000Z",
    "expires": "2025-07-18T11:30:00.000Z"
  }
  ```

- The same documentation's Authentication & Authorization section describes the OIDC exchange token
  as having a "limited lifetime (typically 1 hour)" — the note ADR 0076 cites.

A mutation-free observation probe on 12026-08-14
([vers-js run 31794210985](https://github.com/windlasstech/vers-js/actions/runs/31794210985), a
temporary `publish.yml` on a throwaway caller branch, deleted afterwards; the minted token was never
logged or persisted and expired within minutes) captured the **actual** `201` body returned by the
live registry for `@windlass/vers-js`:

```json
{
  "created": 1786705013,
  "expires": 1786705913,
  "token": "<375-character string>",
  "token_type": "oidc"
}
```

The observation pins three facts:

1. The member set is exactly the four documented members — no unknown members.
2. `created` and `expires` are **JSON numbers (epoch seconds)**, not the RFC 3339 strings shown in
   the documentation example. The observed values `1786705013` and `1786705913` are
   2026-08-14T10:56:53Z and 2026-08-14T11:11:53Z.
3. The token lifetime is exactly **900 seconds (15 minutes)**, not "typically 1 hour". The observed
   `token` is a 375-character string without the `npm_` prefix.

The ecosystem's own clients never notice this drift: the npm CLI
([`lib/utils/oidc.js`](https://github.com/npm/cli/blob/51c2bf81fa2c31547d0fec44fff2aaac3d9a9862/lib/utils/oidc.js#L117-L142))
and pnpm
([`publish/oidc/authToken.ts`](https://github.com/pnpm/pnpm/blob/5d33ac39b0f795ff72aa5c2df1704daf2450b435/pnpm11/releasing/commands/src/publish/oidc/authToken.ts#L96-L113))
both read only the `token` member and ignore everything else. Our strict boundary did notice — the
fail-closed design caught undocumented platform drift before any mutation. The problem to solve is
therefore not "become lax"; it is to pin the contract to the observed reality while retaining the
drift detection that just proved its value, and to correct the lifetime assumption that timing
decisions rely on.

## Decision Drivers

- Contracts that gate registry mutation must be pinned to empirically observed platform behavior,
  not to documentation examples; the live dogfood measurement is the authority.
- Fail-closed strictness caught real documentation/implementation drift pre-mutation; drift
  detection must be retained, not dismantled.
- npm may plausibly serve either the documented or the observed timestamp representation (its own
  documentation shows strings; its production registry sends numbers). Rejecting the documented form
  would recreate the same failure class the day npm aligns with its documentation.
- The observed 15-minute lifetime constrains exchange-to-use timing; no part of the publish path may
  assume the documented "typically 1 hour".
- Diagnostic IDs and the failure-classification mapping are a stable machine contract pinned by
  fixtures; this decision must not rename, renumber, or reclassify them.
- Secret safety: token values must never enter logs, diagnostics, or artifacts; the probe method
  (shape-only logging with masking) is the accepted observation pattern.

## Considered Options

1. Accept only the observed representation (epoch-number timestamps), keep unknown-member rejection.
2. Accept a union — epoch numbers **or** RFC 3339 strings — for `created` and `expires`, keep
   unknown-member rejection and all semantic validation.
3. Parse laxly like the ecosystem clients: read `token` only and ignore the rest.

## Decision Outcome

Chosen option: **option 2 — union timestamp decoding with retained strictness**, because it aligns
the decoder with both the documented and the observed legitimate representations while keeping every
other drift defense intact.

The npm OIDC exchange contract is pinned as follows:

- Request: `POST /-/npm/v1/oidc/token/exchange/package/{url-encoded package name}` with
  `Authorization: Bearer <GitHub Actions OIDC id_token>` minted with audience
  `npm:registry.npmjs.org`. (Unchanged; matches both the documentation and the implementation.)
- Status classification: `201` is the only success; `401` and `404` classify as
  `trusted-publisher-mismatch`; every other status classifies as `npm-oidc-exchange-indeterminate`.
  (Unchanged.)
- Success body: exactly the four documented members `token_type`, `token`, `created`, `expires`.
  Unknown members remain rejected — the observed member set equals the documented set, so a new
  member is precisely the drift we want to notice.
- `token_type` must be exactly `"oidc"`; `token` must be non-empty and free of CR/LF/NUL.
  (Unchanged.)
- `created` and `expires` accept **either** a JSON number interpreted as epoch seconds (integral,
  positive) **or** an RFC 3339 string, and are normalized internally to instants. Semantic
  validation is unchanged: `expires` must be after `created`, and `expires` must be after the
  exchange time.
- Token lifetime assumption: **15 minutes (observed)**, superseding for all timing purposes the
  "typically 1 hour" note in npm's official API documentation (<https://api-docs.npmjs.com/>,
  Authentication & Authorization section) — the note ADR 0076 cites from the same source. The
  publish-time exchange must occur immediately before the publish mutation, and token expiry must be
  re-validated at use time; an exchange performed as an early preflight validates configuration only
  and its token is never assumed usable later.
- Diagnostics: no diagnostic ID, classification mapping, or fixture identifier changes. Failure
  messages may name the failed aspect (for example, timestamp representation) without ever including
  token material.
- Specification: the JS/TS npm provenance and publish specification gains the exchange
  success-response contract above before the decoder change lands (spec-first per SDD).

### Consequences

- Good, because the decoder now matches the empirically verified registry behavior, and the next
  dogfood run can pass the early-exchange preflight.
- Good, because unknown-member rejection, exact `token_type` validation, semantic lifetime checks,
  and fail-closed classification all remain — the drift detection that caught this defect is
  preserved.
- Good, because the 15-minute observed lifetime removes a latent timing hazard: any exchange-to-use
  gap designed around "typically 1 hour" would have been silently wrong.
- Bad, because union parsing is wider than the single observed representation; if npm introduces a
  third representation the pipeline fails closed again. This is accepted: noticing drift is the
  design goal, and the classification path is proven.
- Neutral, because npm's documentation drift (string example, "typically 1 hour") is now recorded
  with evidence, and future readers are directed to the observed contract rather than the
  documentation example.

### Confirmation

- Decoder fixtures cover both the observed shape (epoch-number timestamps, exact four-member body)
  and the documented shape (RFC 3339 strings), plus rejections for unknown members, wrong
  `token_type`, inverted or expired lifetimes, and malformed values.
- The next live M1 dogfood run (after the vers-js caller pin carries this fix) must pass the early
  npm OIDC exchange preflight and complete the publish; the evidence record
  (`docs/dogfood/npm-m1.md`) captures the preflight classification.
- The ADR 0076 standing item — empirically exercise the early-exchange preflight on the first
  dogfood publish and record the result — is satisfied by
  [vers-js run 31737001312](https://github.com/windlasstech/vers-js/actions/runs/31737001312) as
  recorded on issue #30.

## Pros and Cons of the Options

### Accept only the observed epoch-number representation

- Good, because it is the tightest possible pin to measured reality.
- Bad, because npm's own documentation shows the string form; if npm aligns its registry with its
  documentation, the pipeline fails closed on a representation we had prior notice of. That is an
  avoidable self-inflicted outage class.

### Accept the union of epoch numbers and RFC 3339 strings

- Good, because both forms have legitimate standing (one documented, one observed), semantic
  validation is identical after normalization, and every other strict check is retained.
- Bad, because the accepted surface is wider than the single observation; a hypothetical third
  representation still fails closed. Accepted as intended drift detection.

### Parse laxly, reading only `token`

- Good, because it matches ecosystem client behavior and would never have failed here.
- Bad, because it dismantles the boundary that just proved its value: inverted, expired, or
  type-confused lifetimes and wrong token types would flow silently into the publish mutation. It
  also forfeits the expiry re-validation that the observed 15-minute lifetime makes necessary.
  Rejected.

## More Information

- npm official API documentation,
  [OIDC token exchange operation](https://api-docs.npmjs.com/#tag/OIDC/operation/exchangeOidcToken)
  (verified 12026-08-14): endpoint, audience requirement, status set, the RFC 3339 string response
  example, and the "typically 1 hour" exchange-token lifetime note in the Authentication &
  Authorization section.
- Empirical observation:
  [vers-js probe run 31794210985](https://github.com/windlasstech/vers-js/actions/runs/31794210985)
  (12026-08-14; temporary branch deleted after the run). Observed body members
  `created`/`expires`/`token`/`token_type`, epoch-second timestamps `1786705013` → `1786705913`
  (900-second lifetime), 375-character token without the `npm_` prefix.
- Dogfood failure that surfaced the drift:
  [vers-js run 31737001312](https://github.com/windlasstech/vers-js/actions/runs/31737001312)
  (12026-08-13), failing with `windlass.verify.error.npm-oidc-exchange-indeterminate` before any
  registry mutation; full report on
  [issue #30](https://github.com/windlasstech/slsa-builder/issues/30#issuecomment-5292605157).
- Ecosystem client behavior: npm CLI
  [`lib/utils/oidc.js`](https://github.com/npm/cli/blob/51c2bf81fa2c31547d0fec44fff2aaac3d9a9862/lib/utils/oidc.js#L117-L142)
  and pnpm
  [`publish/oidc/authToken.ts`](https://github.com/pnpm/pnpm/blob/5d33ac39b0f795ff72aa5c2df1704daf2450b435/pnpm11/releasing/commands/src/publish/oidc/authToken.ts#L96-L113)
  read only the `token` member.
- Amends [ADR 0076](0076-use-observation-preflights-and-first-mutation-classification.md): exchange
  success-response contract and token lifetime assumption (scope in frontmatter).
