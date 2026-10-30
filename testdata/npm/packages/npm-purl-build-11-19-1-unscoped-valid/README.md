# npm PURL compatibility: build npm 11.19.1, unscoped

Source: this directory's `purl-compat@0.1.0` package tree.

Version key: npm 11.19.1 bundled with Node.js 24.22.1 (verification-policy taxonomy).

Purpose: pin the unscoped subject from `NPMPackagePURL`.

Verification: passed on 12026-10-29 UTC with Go 1.27.3 and Node.js 24.22.1 on darwin/arm64.
`TestNPMPURLCLICompatibilityLive` confirmed that this npm CLI's bundled `npm-package-arg`,
`NPMPackagePURL`, and the fixture subject agree byte-for-byte.

Command: `SLSA_BUILDER_LIVE_TOOLCHAIN=1 go test -count=1 ./internal/npmprofile -run 'Live$'` (no
`-short`, `GOFLAGS='-mod=readonly -v'`, command-scoped loopback `HTTPS_PROXY`). Evidence: baseline
`live-toolchain.log`. No publish or OIDC exchange was performed.

Recorded: 12026-10-30.

Regeneration: preserve package and subject bytes; review the pair on update.
