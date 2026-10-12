# npm PURL compatibility: publish npm 11.21.0, unscoped

Source: this directory's `purl-compat@0.1.0` package tree.

Version key: npm 11.21.0 in the publish-stage allowlist (provenance-publish spec).

Purpose: pin the unscoped subject from `NPMPackagePURL`.

Verification: `TestNPMPURLCLICompatibilityLive` passed on 12026-10-13 against the pinned npm 11.21.0
publish CLI (digest-verified `ProvisionNPM`) and npm 11.19.0 bundled with Node.js 24.21.0. It
exercised each CLI's `libnpmpublish` provenance-subject expression (`npm-package-arg`
`toPurl(resolve(name, version))`) without publishing or registry access for the PURL check, matching
this fixture's subject and `NPMPackagePURL`.

Recorded: 12026-10-13.

Regeneration: preserve package and subject bytes; review the allowlist on update.
