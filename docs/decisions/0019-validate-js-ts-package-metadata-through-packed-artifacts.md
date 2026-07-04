---
parent: Decisions
nav_order: 19
status: accepted
date: 12026-06-28
decision-makers: Yunseo Kim
---

# Validate JS/TS Package Metadata Through Packed Artifacts

## Context and Problem Statement

ADR 0013 scoped the initial JS/TS profile to npm registry package releases. ADR 0014 through ADR
0017 defined the supported package managers and package-manager selection and enforcement policy.
ADR 0018 decided that one profile run publishes one package, either from the repository root or from
one explicitly selected workspace package.

The next scope decision is package metadata validation: which `package.json` fields should the
profile validate before packing and publishing, which fields should be recorded for provenance, and
which fields should be left to npm, pnpm, Yarn, Node.js, or downstream package consumers.

The package metadata fields do not all have the same role. `name`, `version`, and `private` affect
whether an npm package can or should be published. `packageManager` and `devEngines.packageManager`
affect release tool selection. `publishConfig` can affect registry, access, provenance, and, for
some package managers, the manifest that is published. `files`, `.npmignore`, `.gitignore`,
`exports`, `main`, `type`, `bin`, `types`, and workspace dependency protocols affect the packed
artifact and package consumer surface.

Should the initial JS/TS npm package profile reimplement package metadata semantics, validate only
the minimum package identity, or treat the packed artifact as the authoritative source for package
contents and publish-time manifest rewrites?

## Decision Drivers

- Preserve a clear SLSA provenance boundary: identify and verify the released artifact, not lint the
  entire package consumer API.
- Fail early for metadata that makes the selected package unsafe, ambiguous, or impossible to
  publish.
- Avoid reimplementing npm packlist, npm publish, pnpm publish, Yarn publish, and Node.js module
  resolution semantics.
- Record verifier-relevant package metadata without turning the profile into a package quality
  linter.
- Support package-manager-specific manifest rewrites by observing the packed result rather than
  guessing from source metadata.
- Keep future semantic package validation possible as an additional profile or quality gate.

## Considered Options

- Validate only publish identity fields before publish.
- Validate publish identity and publish intent fields before publish.
- Validate publish identity and intent from source metadata, then use the packed artifact as the
  authoritative source for package contents and publish-time manifest rewrites.
- Semantically validate the full package consumer surface.
- Delegate all metadata checks to the selected package manager.

## Decision Outcome

Chosen option: "Validate publish identity and intent from source metadata, then use the packed
artifact as the authoritative source for package contents and publish-time manifest rewrites",
because it catches release-scope mistakes early while keeping npm, pnpm, Yarn, and Node.js semantics
authoritative where they already define package behavior.

The initial JS/TS npm package profile should validate source package metadata only for fields that
define release identity, release intent, package-manager selection, and profile scope.

The source manifest validation should fail when:

- the selected package manifest is missing;
- `name` is missing or invalid for an npm package publish target;
- `version` is missing or not a valid npm semver version;
- `private` is `true`;
- selected package directory or workspace metadata does not identify exactly one package, consistent
  with ADR 0018;
- package-manager metadata conflicts with ADR 0015, ADR 0016, or ADR 0017;
- source metadata or workflow inputs request multi-package publish;
- `publishConfig.directory` redirects the packed package root away from the selected package
  directory, unless a future ADR explicitly supports that behavior.

The profile should treat the package-manager pack result as authoritative for package contents and
publish-time manifest rewrites. It should inspect the packed tarball and its `package/package.json`
after pack, then fail if the packed manifest's `name` or `version` differs from the validated source
package identity.

The profile should record source metadata that explains release intent and packed metadata that
explains the actual released artifact. It should not semantically validate the full package consumer
surface in the initial profile.

### Metadata to Validate Before Packing

The profile should validate these source fields because they affect release identity, release
intent, or profile scope:

- `name`;
- `version`;
- `private`;
- `packageManager`;
- `devEngines.packageManager`;
- `publishConfig` fields that affect registry, access, dist-tag, provenance, or package root;
- workspace or package directory metadata needed to prove that exactly one selected package is in
  scope.

### Metadata and Artifact Data to Record

The profile should record, where applicable:

- selected source manifest path;
- selected package directory or workspace selector;
- source `name`, `version`, and `private`;
- source `packageManager` and `devEngines.packageManager`;
- source `publishConfig` values relevant to registry, access, dist-tag, provenance, and package
  root;
- selected package manager name, version, and selection source;
- packed tarball path, digest, and package URL identity;
- packed manifest `name` and `version`;
- packed manifest consumer-surface fields such as `exports`, `main`, `type`, `bin`, `types`,
  `typings`, `typesVersions`, and `files` when present;
- packed file list.

Recording these fields does not mean the initial profile guarantees that every entry point or type
definition is semantically correct. It means the verifier can see which package artifact and package
surface were built and published.

### Metadata Not Semantically Validated in the Initial Profile

The initial profile should not reimplement or fully validate:

- npm packlist behavior for `files`, `.npmignore`, `.gitignore`, and always-included or
  always-excluded files;
- Node.js `exports`, `imports`, `main`, and `type` module-resolution semantics;
- executable validity or shebang correctness for `bin` files;
- TypeScript declaration correctness for `types`, `typings`, or `typesVersions`;
- dependency graph semantics, peer dependency correctness, optional dependency platform behavior, or
  workspace protocol rewrite semantics beyond observing the packed result;
- package quality checks that belong in tests, package linters, or downstream consumer validation.

### Consequences

- Good, because identity and publish-intent mistakes fail before publish.
- Good, because the profile does not need to clone package-manager-specific packlist and manifest
  rewrite behavior.
- Good, because provenance can describe both the selected source package and the actual packed
  artifact.
- Good, because workspace and package-manager metadata remain aligned with ADR 0015 through
  ADR 0018.
- Neutral, because package quality checks remain the responsibility of project tests or optional
  validation layers.
- Bad, because some broken package entry points may only be caught by package-manager errors,
  consumer tests, or downstream users.
- Bad, because specs must clearly define the difference between source manifest metadata and packed
  manifest metadata.

### Confirmation

This decision is confirmed when the initial JS/TS npm package profile architecture specification
defines:

- source manifest discovery for root packages and selected workspace packages;
- source metadata validation failures for `name`, `version`, `private`, package-manager metadata,
  single-package scope, and unsupported `publishConfig.directory` behavior;
- the pack command as the source of truth for included files and publish-time manifest rewrites;
- packed tarball inspection, including `package/package.json` extraction;
- failure when source and packed `name` or `version` differ;
- provenance or log fields for source metadata, packed metadata, tarball digest, and packed file
  list;
- explicit exclusion of full package consumer-surface semantic validation from the initial profile.

Implementation review should verify that the profile never decides package inclusion by manually
reimplementing `files` or ignore-file semantics, and never claims that recorded `exports`, `main`,
`bin`, or type metadata have been semantically validated unless a later ADR adds that gate.

## Pros and Cons of the Options

### Validate only publish identity fields

Validate only the fields needed to identify an npm package release, such as `name`, `version`, and
`private`.

- Good, because this is small and closely tied to package identity.
- Good, because it minimizes package-manager-specific behavior in the profile.
- Bad, because registry, access, dist-tag, provenance, and package-root intent may remain implicit
  or ambiguous.
- Bad, because verifier-relevant release choices can be hidden in `publishConfig`, environment, or
  package-manager behavior.

### Validate publish identity and publish intent fields

Validate package identity plus source metadata that expresses publish destination, visibility, and
tool selection.

- Good, because it catches more release-scope mistakes before pack and publish.
- Good, because `publishConfig`, package-manager metadata, and selected package scope become
  explicit verifier-relevant inputs.
- Bad, because it still does not explain which files and manifest rewrites are actually published.
- Bad, because package-manager-specific `publishConfig` behavior differs and may be hard to infer
  from source metadata alone.

### Validate source intent and use the packed artifact as authoritative

Validate source identity and intent, then inspect the packed tarball as the source of truth for
published package contents and manifest rewrites.

- Good, because it matches the actual artifact being published.
- Good, because npm, pnpm, and Yarn remain authoritative for packlist and manifest rewrite behavior.
- Good, because source metadata and packed metadata can both be recorded for provenance.
- Good, because it avoids turning the trusted profile into a consumer-surface linter.
- Neutral, because the profile must extract and inspect the tarball after pack.
- Bad, because some source metadata errors are only visible after pack.

### Semantically validate the full package consumer surface

Validate `exports`, `main`, `type`, `bin`, `types`, `files`, dependency fields, and related package
consumer behavior before publish.

- Good, because it catches many package quality problems before release.
- Good, because downstream users are less likely to receive a package with broken entry points.
- Bad, because it requires reimplementing large parts of npm, pnpm, Yarn, Node.js, bundler, and
  TypeScript behavior.
- Bad, because the profile's SLSA provenance boundary becomes mixed with package linting and
  testing.
- Bad, because semantic validation rules may become stale as the JS/TS ecosystem changes.

### Delegate all metadata checks to the package manager

Run pack and publish and let the package manager fail if metadata is invalid.

- Good, because package-manager behavior is always authoritative.
- Good, because the profile has very little metadata logic.
- Bad, because preventable release-scope mistakes fail late.
- Bad, because `private`, selected package ambiguity, and publish-intent conflicts are not explained
  in profile-specific terms.
- Bad, because provenance may omit important source intent that affected the release.

## More Information

This decision complements:

- ADR 0013's npm package artifact scope;
- ADR 0015's manifest-first package-manager selection;
- ADR 0017's package-manager version enforcement;
- ADR 0018's one-package-per-run scope.

Reference points considered:

- npm treats `name` and `version` as the required package identity for publishing and uses packlist
  semantics to decide included files.
- npm publish fails when a package name and version already exist in the target registry and submits
  tarball integrity metadata to the registry.
- npm and Yarn treat `private: true` as a publish-prevention signal.
- Corepack uses `packageManager` and `devEngines.packageManager` to choose or validate package
  manager usage.
- pnpm and Yarn can apply publish-time manifest behavior through `publishConfig`.
- Node.js defines `exports`, `main`, and `type` package behavior, with `exports` taking precedence
  over `main` in supported versions.

Future decisions may define:

- whether to support `publishConfig.directory` as a build-output package-root selection mechanism;
- whether non-default registries are allowed, denied, or constrained by organization policy;
- whether package consumer-surface validation should be offered as a separate optional quality gate;
- whether packed manifest fields should be normalized into a stable provenance schema or recorded as
  raw metadata.
