---
parent: Decisions
nav_order: 23
status: accepted
date: 12026-07-01
decision-makers: Yunseo Kim
---

# Use `package-directory` as the Required JS/TS npm Package Selector

## Context and Problem Statement

ADR 0002 chose an extensible trusted reusable workflow foundation, ADR 0003 refined it into a thin
core with profile-owned reusable workflows, and ADR 0022 selected
`.github/workflows/js-ts-npm-package-slsa3.yml` as the initial JS/TS npm package profile entrypoint.
ADR 0013 through ADR 0019 scoped that profile to one npm registry package per run, with package
manager selection derived from package metadata and with publish identity and intent validated from
source metadata and the packed artifact.

The next public-contract decision is which `workflow_call` inputs the initial JS/TS npm package
profile requires for selecting the package and expressing npm publish intent. These inputs become
part of the SLSA provenance `externalParameters` surface and therefore must stay explicit, complete,
and small enough for downstream verification.

Which reusable workflow inputs should be required, optional, or forbidden for selecting the package
and publish intent?

## Decision Drivers

- Keep SLSA provenance v1 `externalParameters` small, explicit, and verifier-friendly.
- Preserve ADR 0018's one-package-per-run release unit.
- Support root packages and selected workspace packages without exposing package-manager-specific
  workspace selector syntax.
- Avoid caller-controlled package-manager, command, step, environment, service, or defaults
  injection into the trusted workflow boundary.
- Let version-controlled package metadata and `publishConfig` remain the primary source for package
  identity and publish intent where possible.
- Allow limited caller publish-intent overrides for registry, dist-tag, and access when needed.
- Keep authentication, trusted publishing, and provenance-file behavior separate from this package
  selection input decision.

## Considered Options

- Require `package-directory` and allow optional publish-intent overrides.
- Support both `package-directory` and a workspace selector.
- Require `package-directory`, `registry-url`, `dist-tag`, and `access`.
- Require a profile configuration file.
- Make all package and publish intent derive from source metadata with no publish override inputs.

## Decision Outcome

Chosen option: "Require `package-directory` and allow optional publish-intent overrides", because it
gives the reusable workflow one canonical package selector while keeping the public input surface
small and avoiding package-manager-specific workspace semantics.

The initial JS/TS npm package profile should define this required input:

```yaml
package-directory:
  required: true
  type: string
```

`package-directory` is the repository-root-relative path to the exact npm package directory selected
for this profile run. For a root package, callers should pass `.`. For a workspace package, callers
should pass the workspace package directory, such as `packages/foo`. The selected directory must
contain the package manifest whose `name`, `version`, `private`, package-manager metadata, and
publish intent are validated by the profile.

The initial profile should allow these optional publish-intent inputs:

```yaml
registry-url:
  required: false
  type: string
dist-tag:
  required: false
  type: string
access:
  required: false
  type: string
```

Empty or omitted optional publish-intent inputs should mean that the profile derives the value from
source package metadata, such as `publishConfig`, or from npm's documented defaults. If an optional
input conflicts with source metadata that expresses the same publish intent, the profile should fail
or otherwise require a separately documented policy before silently overriding source metadata.

The profile should record the supplied and resolved package-selection and publish-intent values in
verifier-relevant provenance parameters where appropriate. The exact `externalParameters` field
names and resolution details belong in the JS/TS npm package profile architecture specification.

The initial profile should not expose a public `workspace`, `workspace-selector`, `workspaces`, or
`include-workspace-root` input. It should also not expose package-manager selection, install
command, build command, publish command, arbitrary environment, arbitrary steps, services, defaults,
provenance, provenance-file, or one-time-password inputs as part of this package selection and
publish-intent contract.

### Consequences

- Good, because every profile run has one canonical package locator.
- Good, because root packages and workspace packages use the same public selector shape.
- Good, because package-manager-specific workspace syntax remains an implementation detail rather
  than becoming part of the public reusable workflow API.
- Good, because the public input surface stays small enough for SLSA verifier expectations.
- Good, because `publishConfig` and package metadata can remain the main source of release intent
  while still allowing targeted caller overrides.
- Neutral, because callers that prefer workspace names must pass package paths instead.
- Bad, because implementation must resolve workspace roots, lockfiles, and package-manager commands
  from the selected package directory rather than relying on a package-manager-native selector
  input.

### Confirmation

This decision is confirmed when the JS/TS npm package profile architecture specification and
implementation define:

- `package-directory` as the only required package-selection input;
- root package behavior when `package-directory` is `.`;
- selected workspace package behavior when `package-directory` points to a workspace package;
- `registry-url`, `dist-tag`, and `access` as optional publish-intent inputs;
- resolution and conflict rules between optional inputs, source `publishConfig`, npm defaults, and
  unsupported values;
- provenance fields for supplied and resolved package-selection and publish-intent values;
- rejection of package-manager override, workspace-multiple-selection, arbitrary command, arbitrary
  environment, arbitrary step, service, defaults, provenance, provenance-file, and OTP inputs.

Implementation review should verify that a caller cannot use workflow inputs to publish multiple
packages, switch package managers, inject arbitrary trusted workflow behavior, or silently redirect
publish intent away from validated package metadata.

## Pros and Cons of the Options

### Require `package-directory` and allow optional publish-intent overrides

Require one repository-relative package directory and allow optional `registry-url`, `dist-tag`, and
`access` overrides.

- Good, because it creates one canonical package selector for both root packages and workspace
  packages.
- Good, because it avoids exposing npm, pnpm, and Yarn's different workspace selector syntaxes.
- Good, because the required input surface remains small and easy to map into SLSA provenance.
- Good, because package-manager and publish metadata decisions remain aligned with ADR 0015 through
  ADR 0019.
- Neutral, because publish-intent optional inputs still need strict conflict and validation rules.
- Bad, because callers cannot select a package by workspace name alone.

### Support both `package-directory` and a workspace selector

Allow callers to provide a package directory or a workspace selector such as a workspace name, npm
workspace value, pnpm filter, or Yarn workspace identity.

- Good, because monorepo callers may already know packages by workspace name.
- Good, because package-manager-native commands often accept workspace selectors.
- Bad, because the selector's meaning differs across npm, pnpm, and Yarn.
- Bad, because overlapping `package-directory` and workspace selector inputs can conflict.
- Bad, because it creates pressure to support multiple workspace selection and therefore
  multi-package publish, which ADR 0018 excludes.

### Require `package-directory`, `registry-url`, `dist-tag`, and `access`

Make package location and publish destination fully explicit in every caller workflow.

- Good, because caller workflows visibly declare where and how the package is published.
- Good, because the supplied values are straightforward to record in provenance.
- Bad, because it duplicates `publishConfig` and npm defaults.
- Bad, because duplicated source and workflow values can drift or conflict.
- Bad, because it expands the verifier-relevant external parameter surface more than necessary.

### Require a profile configuration file

Use a version-controlled profile configuration file as the single required input, and put package
selection and publish intent inside that file.

- Good, because the caller input surface can be very small.
- Good, because richer configuration can be versioned and reviewed in the source repository.
- Neutral, because this may become useful if the profile contract grows substantially.
- Bad, because it creates a second configuration surface alongside `package.json` and
  `publishConfig`.
- Bad, because it adds schema, versioning, validation, and documentation burden before the initial
  profile needs it.

### Derive all package and publish intent from source metadata

Require no publish-intent override inputs and derive package selection or publish behavior entirely
from source files.

- Good, because it minimizes external inputs.
- Good, because release intent is fully version-controlled.
- Bad, because the workflow still needs a way to select exactly one package in monorepos.
- Bad, because legitimate release use cases may need caller-selected registry, tag, or access
  values.
- Bad, because forcing all publish intent into source metadata can make release-channel variation
  awkward.

## More Information

This decision follows ADR 0022's workflow entrypoint decision and complements ADR 0015, ADR 0018,
and ADR 0019. It decides the package selection and publish-intent input surface only.
Authentication, npm token versus OIDC trusted publishing, required secrets, caller permissions,
runner policy, Node.js version policy, package-manager version behavior, and reusable workflow
version pinning remain separate contract decisions.

Reference points considered:

- SLSA provenance v1 defines `externalParameters` as externally controlled build interface values
  that must be complete at SLSA Build L3 and should be kept small to ease verification.
- GitHub reusable workflow inputs are declared under `on.workflow_call.inputs` and are passed by
  callers through `jobs.<job_id>.with` as scalar values.
- npm `publish` supports publishing a folder or tarball, `tag` defaults to `latest`, and `access`
  has npm-defined behavior for new, existing, scoped, and unscoped packages.
- npm `publishConfig` can express publish-time configuration such as registry, tag, and access in
  source metadata.
- npm, pnpm, and Yarn expose different workspace-selection syntax, so a package-directory selector
  is the most stable cross-package-manager public contract for one package per profile run.
- npm trusted publishing and npm provenance behavior affect authentication and provenance
  generation, but those concerns should not be mixed into the package selector input decision.
