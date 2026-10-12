# Toolchain Pin Records

`internal/toolchainpins` embeds the builder-owned publish-stage npm pin and the Node.js/npm build
toolchain pair. These independent machine-readable records change only through the ADR 0082 and ADR
0085 bump procedures, respectively.

- `publish-npm.json` follows the
  [publish-stage npm CLI allowlist](../../docs/architecture/js-ts-npm-provenance-publish.md#publish-stage-npm-cli-allowlist).
- `node-runtime.json` follows the
  [Node.js and npm toolchain pair](../../docs/architecture/js-ts-npm-build-pack.md#nodejs-and-npm-toolchain-pair).

A future `pin-check` subcommand will compare the specification tables against these records and
reject drift. Do not duplicate pin literals in provisioning or publish-side validation code.
