# Fixture Corpus Knowledge Base

## OVERVIEW

Byte-exact, registry-governed conformance fixtures shared by `internal/` packages. Every normative
spec requirement maps to accepted and rejected fixtures registered in
`testdata/fixtures/index.json`.

## STRUCTURE

```text
testdata/
├── attestation/       # Sigstore trust-root, bundle, and expected Statement data.
├── canonicaljson/     # RFC 8785 paired input/output vectors; upstream provenance in its README.
├── diagnostics/       # Canonical verification-report goldens.
├── fixtures/          # index.json registry plus data/ and rejected/ registry examples.
├── handoff/           # Producer-to-publisher handoff accept/reject contracts.
├── identity/          # Builder, source, and certificate identity cases.
├── npm/               # packages/ valid+rejected package trees; buildpack/ hermetic byte assets
│                      # including Corepack payloads (regeneration README); provenance/ predicate goldens.
├── platform/          # contracts/ pinned external-platform evidence copies (README.md + README.ko.md);
│                      # runtime-delivery/ F01 spike evidence and report schema.
├── policy/            # Explicit-policy, intersection, TUF, and manifest-expectation vectors.
├── provenance/        # SLSA Statement/predicate accept and reject vectors.
└── signing/           # Go-native signer identity, Statement, and bundle fixtures.
```

`composition/`, `publisher/`, `release-manifest/`, and `trustroots/` do not exist yet; they arrive
in later waves.

## WHERE TO LOOK

| Task or topic                                 | Location                                                          | Notes                                        |
| --------------------------------------------- | ----------------------------------------------------------------- | -------------------------------------------- |
| Registry schema and loader validation         | `internal/fixture/model.go`, `loader.go`                          | Manifest shape and rejection rules.          |
| Registered surfaces, requirements, categories | `internal/fixture/registry.go`                                    | Closed registries; extend here first.        |
| Fixture taxonomy normative source             | `docs/architecture/verification-policy-and-fixtures.md`           | Spec that this corpus proves.                |
| Vector provenance                             | `README.md` under each fixture directory                          | Origin and regeneration steps per directory. |
| npm fixture cases                             | `testdata/npm/packages/`, `internal/fixture/npm_fixtures_test.go` | Expected-case map gates npm fixtures.        |
| Test policy, fuzzing, quality gates           | `docs/testing-guide.md`                                           | Owns generic test policy; not repeated here. |

## CONVENTIONS

- **Registry shape**: `index.json` is a closed object with only `fixtures`. Each manifest requires
  `name`, `type` (`accepted` or `rejected`), `surface` (`npm`, `publisher`, `composition`,
  `release-manifest`), artifact/provenance/release-manifest paths (relative, contained under
  `testdata/`), `expected-result`, `expected-failure-category`, `expected-primary-id`,
  `expected-secondary-ids`, and `covered-requirement` (a registered `ADR-####.<req>` or
  `ARCH-<req>`).
- **Accepted fixtures**: `expected-result` is `pass` with null category and primary ID.
- **Rejected fixtures**: `expected-result` is `fail` with a registered category, and
  `expected-primary-id` equals `windlass.verify.error.<same-category>`. Secondary IDs are canonical,
  unique, and must not duplicate the primary.
- **Loader rejections**: duplicate JSON members, unknown or missing fields, duplicate names,
  absolute paths, traversal, and paths escaping `testdata/`.
- **Registration order**: new requirements or failure categories register first in
  `internal/fixture/registry.go`. New npm fixtures also register in the expected-case map of
  `internal/fixture/npm_fixtures_test.go`, which asserts no fixture command contains `npm publish`.
- **Byte-exactness**: `.prettierignore` excludes `testdata/**/*.json|jsonl|yaml|yml`. Never run
  formatters over fixture bytes or reserialize them. JCS bytes, bundles, tarballs, and evidence
  payloads stay bit-identical; autofix.ci broke vectors once, and the exclusions exist because of
  that.
- **Platform evidence**: `platform/` files are non-production copies. Never install them as
  production workflows. Preserve emitted bundle bytes exactly; signature bytes may differ across
  runs.
- **Phase metadata**: `phase` is not stored in `index.json`. npm package-local `fixture.json` files
  carry phases (`build-pack`, `provenance-inputs`) mapped through `registry.go` covered-requirement
  entries.

## ANTI-PATTERNS

- Never pretty-print or reformat fixture bytes, and never let formatter bots touch `testdata/`.
- Never add a fixture without `index.json` registration and `registry.go` entries.
- Never point a fixture path outside `testdata/` or use absolute paths.
- Never register a rejected fixture whose primary ID does not match its failure category.
- Never treat `platform/contracts` or `platform/runtime-delivery` spike files as production workflow
  sources; the runtime-delivery negative-test `delivery_sha` input is intentionally unsafe.
