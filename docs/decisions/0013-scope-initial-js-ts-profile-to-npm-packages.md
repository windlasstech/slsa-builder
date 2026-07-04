---
parent: Decisions
nav_order: 13
status: accepted
date: 12026-06-28
decision-makers: Yunseo Kim
---

# Scope the Initial JS/TS Profile to npm Packages

## Context and Problem Statement

ADR 0002 chose an extensible trusted reusable workflow foundation with JS/TS as the first profile.
ADR 0003 refined that architecture into a thin shared core with profile-owned reusable workflows.
The next required decision is the artifact scope of the initial JS/TS profile before writing stable
architecture specifications.

Windlass wants npm package format support first, while allowing flexible package-manager use for
development, install, build, and pack steps. For example, a target package may pin a non-npm
development package manager through `package.json` `devEngines`, and the builder should respect that
where it affects the build. However, current `pnpm publish` behavior does not support the
`provenance-file` option needed for externally generated provenance, so provenance-aware publishing
must use `npm publish` for the initial profile.

Should the initial JS/TS profile support only npm registry packages, include standalone package
tarballs, include GitHub Release assets, or also cover generic file artifacts and container images?

## Decision Drivers

- Keep the initial JS/TS profile focused enough to specify, implement, audit, and verify.
- Make npm package identity first-class: package name, version, registry metadata, package URL, and
  registry tarball integrity.
- Support flexible package-manager use for development, install, build, and pack steps without
  making every package manager a publish authority.
- Use `npm publish` for provenance-aware package publishing while `pnpm publish` lacks
  `provenance-file` support.
- Avoid collapsing JS/TS package semantics into a generic file-artifact model.
- Preserve a clear path for projects that publish both npm packages and GitHub Release assets.
- Keep container images and generic release files available as future profiles without expanding the
  first profile's trusted surface.

## Considered Options

- Scope the initial JS/TS profile to npm registry packages, treating the package tarball as the
  package artifact's physical representation.
- Support npm registry packages and standalone `.tgz` package tarballs as separate initial artifact
  classes.
- Support npm registry packages and GitHub Release assets in the initial JS/TS profile.
- Support generic file artifacts in the initial JS/TS profile.
- Support container images in the initial JS/TS profile.

## Decision Outcome

Chosen option: "Scope the initial JS/TS profile to npm registry packages, treating the package
tarball as the package artifact's physical representation", because npm package identity is the
first ecosystem-specific release target, while GitHub Release assets, generic files, and containers
have different verification units and should be modeled as separate profiles or orchestration
surfaces.

The initial JS/TS profile should support:

- npm registry package releases as the profile's release artifact;
- package-manager-flexible install, build, and pack behavior, subject to later profile contract
  decisions;
- `npm publish` as the provenance-aware publish mechanism;
- package identity based on package name, package version, registry package URL or package URL, and
  the published tarball's digest or integrity metadata;
- stable outputs that allow a future GitHub Release asset profile or release orchestration workflow
  to reuse the same package tarball.

The initial JS/TS profile should not directly support:

- standalone `.tgz` tarballs as a separate release artifact class independent of an npm package
  release;
- GitHub Release assets;
- generic file artifacts;
- container images;
- `pnpm publish` provenance publishing while `pnpm publish` lacks `provenance-file` support.

Projects that publish both npm packages and GitHub Release assets should use profile composition
rather than a single combined JS/TS profile. The npm profile owns npm package publish and package
provenance. A future GitHub Release asset profile should own release asset upload, asset digest,
release attestation, and release-asset verification. A higher-level release orchestration workflow
may call both profiles in sequence when a repository wants one release process that publishes to npm
and attaches assets to a GitHub Release.

To support that future composition, the npm profile should expose stable outputs such as package
name, package version, package directory, packed tarball path or name, tarball digests or integrity
values, package URL, and verification hints.

### Consequences

- Good, because the first profile's artifact model follows the JS/TS ecosystem's native release
  unit: an npm package version.
- Good, because npm registry package verification can be specified around package name, version,
  registry metadata, package URL, and tarball integrity rather than around arbitrary files.
- Good, because package-manager flexibility remains available for install, build, and pack steps
  without requiring all package managers to support provenance-aware publishing.
- Good, because `npm publish` can be the initial provenance-aware publish path while `pnpm publish`
  lacks `provenance-file` support.
- Good, because GitHub Release assets remain supported as a future composable profile instead of
  being mixed into npm package semantics.
- Good, because generic file and container support remain possible through separate future decisions
  and profiles.
- Neutral, because repositories that publish npm packages and GitHub Release assets will need an
  orchestration workflow that combines profiles.
- Bad, because projects that only distribute JS/TS build outputs as GitHub Release assets, generic
  files, or containers are outside the initial JS/TS profile.
- Bad, because standalone package-tarball distribution must wait for a GitHub Release asset or
  generic file profile if it is not tied to npm registry publishing.

### Confirmation

This decision is confirmed when the initial JS/TS profile architecture specification defines:

- npm registry package releases as the supported artifact scope;
- the package tarball as the physical representation and digest source for the npm package release,
  not as an independent initial artifact class;
- `npm publish` as the initial provenance-aware publish mechanism;
- the package-manager flexibility boundary for install, build, and pack steps;
- stable outputs that allow later GitHub Release asset composition;
- explicit exclusions for standalone tarballs, GitHub Release assets, generic files, container
  images, and provenance-aware `pnpm publish`.

Implementation review should verify that the initial JS/TS profile does not silently grow GitHub
Release asset, generic file, or container behavior without a follow-up ADR and profile contract.

## Pros and Cons of the Options

### Scope the initial JS/TS profile to npm registry packages

The profile treats an npm package version as the release artifact. The package tarball is the
concrete file whose digest or integrity metadata supports package verification, but it is not a
separate initial artifact class.

- Good, because it matches the first intended ecosystem target: JS/TS packages published to an
  npm-compatible registry.
- Good, because package identity and verification can be modeled through npm package name, version,
  registry metadata, package URL, and tarball integrity.
- Good, because `npm publish` supports the provenance-aware publish path needed by the profile.
- Good, because non-npm package managers can still be used where they are appropriate for
  development, install, build, and pack behavior.
- Good, because it keeps the first profile small enough to audit and test.
- Neutral, because a repository that also wants GitHub Release assets needs profile composition.
- Bad, because non-registry JS/TS artifacts are excluded from the first profile.

### Support npm packages and standalone `.tgz` tarballs

The profile would support npm registry package releases and also support standalone package tarballs
that are generated, attested, or distributed independently of npm registry publishing.

- Good, because package tarballs are the physical artifact users ultimately download from the npm
  registry.
- Good, because it could support dry-run, offline validation, or manual distribution workflows more
  directly.
- Neutral, because the npm profile still needs tarball digest handling internally.
- Bad, because registry-published packages and standalone files have different publication and
  verification semantics.
- Bad, because it would require the initial profile to define both package identity and generic file
  identity.
- Bad, because standalone distribution is better modeled by a GitHub Release asset or generic file
  profile.

### Support npm packages and GitHub Release assets

The profile would publish npm packages and also upload or attest files attached to a GitHub Release.

- Good, because many GitHub-hosted projects publish npm packages and maintain GitHub Releases
  together.
- Good, because a release orchestration workflow could produce a convenient single release path for
  downstream users.
- Neutral, because GitHub Release assets are still important for future release workflows.
- Bad, because npm package provenance and GitHub Release asset attestation use different artifact
  identities, verification commands, and failure modes.
- Bad, because the profile would need to define release tag, asset name, asset digest, immutable
  release behavior, and release verification in addition to npm package behavior.
- Bad, because this mixes ecosystem package semantics with general release-asset semantics.

### Support generic file artifacts

The profile would support arbitrary file outputs and checksums in addition to npm packages.

- Good, because it is the most flexible artifact model.
- Good, because it can support projects that produce bundles, archives, generated manifests, or
  other files.
- Bad, because it weakens the profile's JS/TS package focus and risks reproducing a
  lowest-common-denominator generic artifact model.
- Bad, because arbitrary files do not capture npm package identity, registry verification, or
  tarball integrity semantics by themselves.
- Bad, because generic file support should be a separate profile with its own artifact and
  verification contract.

### Support container images

The profile would support container image builds and attestations for JS/TS applications.

- Good, because many Node.js applications are deployed as container images rather than published as
  npm packages.
- Good, because container image identity by registry name and immutable digest has a clear
  verification model.
- Bad, because container images require different build, registry, permission, SBOM, and
  linked-artifact concerns from npm packages.
- Bad, because this would expand the first JS/TS profile into an application deployment profile
  rather than a package profile.
- Bad, because container support should be handled by a future container profile or separate ADR.

## More Information

This decision refines ADR 0002 and ADR 0003 for the first concrete JS/TS profile.

The decision assumes that future architecture specifications will still decide the detailed
package-manager contract, workspace support, subject and digest schema, provenance-file handling,
publish authentication, and verification commands.

Reference points considered:

- npm `publish` supports folder and tarball publishing and includes `provenance` and
  `provenance-file` options.
- pnpm `publish` is native in current pnpm releases and documents `provenance`, but not
  `provenance-file`; pnpm documents `pnpm pack && npm publish *.tgz` as the workaround for npm CLI
  publish behavior.
- npm package verification is centered on package identity and registry tarball integrity.
- GitHub artifact attestations are appropriate for release artifacts that consumers download or
  deploy, but GitHub Release assets and containers have different verification surfaces from npm
  packages.
