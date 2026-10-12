# npm PURL compatibility: build npm 11.19.0, unscoped

Source: this directory's `purl-compat@0.1.0` package tree.

Version key: npm 11.19.0 with Node.js 24.21.0 (verification-policy taxonomy).

Purpose: pin the unscoped subject from `NPMPackagePURL`.

Verification: `TestNPMPURLCLICompatibilityLive` passed on 12026-10-13 against npm 11.19.0 bundled
with Node.js 24.21.0 and the pinned npm 11.21.0 publish CLI (digest-verified `ProvisionNPM`). It
exercised each CLI's `libnpmpublish` provenance-subject expression (`npm-package-arg`
`toPurl(resolve(name, version))`) without publishing or registry access for the PURL check, matching
this fixture's subject and `NPMPackagePURL`.

Recorded: 12026-10-13.

Regeneration: preserve package and subject bytes; review the pair on update.
