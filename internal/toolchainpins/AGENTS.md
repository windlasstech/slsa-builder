# Toolchain Pin Records

`internal/toolchainpins` embeds the builder-owned publish-stage npm pin and the Node.js/npm build
toolchain pair. These independent machine-readable records change only through the ADR 0082 and ADR
0085 bump procedures, respectively.

- `publish-npm.json` follows the
  [publish-stage npm CLI allowlist](../../docs/architecture/js-ts-npm-provenance-publish.md#publish-stage-npm-cli-allowlist).
- `node-runtime.json` follows the
  [Node.js and npm toolchain pair](../../docs/architecture/js-ts-npm-build-pack.md#nodejs-and-npm-toolchain-pair).

`pin-check` compares explicit disk inputs (`--publish-pin`, `--pair-record`, `--publish-spec`, and
`--build-spec`), not embedded records. It requires one data row per specification table and exact
equality of the publish version, URL, and SRI, and of the Node.js/npm pair. Markdown table padding
and inline-code delimiters do not affect comparison. Missing sections/tables, malformed or
unreadable inputs, and unequal values fail with `windlass.verify.error.toolchain-pin-drift` (check
`toolchain.pins.drift`), a canonical report, and exit 1. Success exits 0; missing flags exit 2. Do
not duplicate pin literals in provisioning or publish-side validation code.

`ParseSigningAdapterModule` reads explicitly supplied `go.mod` and `go.sum` bytes for generation and
drift tooling only. It accepts canonical exact candidate versions independently of the compiled
signer pin, rejects signer replacements, and requires exactly one matching module-content `h1`
checksum (never a `/go.mod` checksum). Strict base64 decoding must yield 32 bytes; `DirHash1` is
their lowercase hexadecimal representation. Invalid or ambiguous inputs return `DriftError` with
`windlass.verify.error.toolchain-pin-drift`.

Production issuance and verification use `provenance.SigningAdapterBuilderDependency()` and its
compiled constants. They never read module manifests at runtime or panic during initialization.
Candidate parsing is not an allowlist decision: graph-to-compiled-pin agreement belongs to drift
checks and tests.
