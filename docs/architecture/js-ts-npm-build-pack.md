# JS/TS npm Build, Pack, Metadata, And Package Manager Rules

This document defines how the JS/TS npm profile selects a package, selects a package manager,
installs dependencies, runs build scripts, packs the artifact, and validates package metadata.

- Source ADRs:
  [0014](../decisions/0014-support-npm-pnpm-and-yarn-for-initial-js-ts-build-stages.md),
  [0015](../decisions/0015-use-manifest-first-package-manager-selection.md),
  [0016](../decisions/0016-use-corepack-for-pnpm-and-yarn-build-stages.md),
  [0017](../decisions/0017-require-explicit-package-manager-version-enforcement.md),
  [0018](../decisions/0018-publish-one-js-ts-package-per-profile-run.md),
  [0019](../decisions/0019-validate-js-ts-package-metadata-through-packed-artifacts.md),
  [0023](../decisions/0023-use-package-directory-as-required-js-ts-npm-package-selector.md),
  [0027](../decisions/0027-use-github-hosted-ubuntu-2404-and-node-24-runtime.md),
  [0033](../decisions/0033-run-build-script-only-when-declared.md)
- Related specs: [JS/TS npm package profile](js-ts-npm-package-profile.md),
  [JS/TS npm provenance and publish](js-ts-npm-provenance-publish.md),
  [Core profile contract](core-profile-contract.md)

## Scope and non-goals

**In scope:**

- Package selection.
- Package manager selection.
- Install, build, and pack commands.
- Source and packed metadata validation.
- Failure behavior for conflicts and missing metadata.

**Out of scope:**

- Provenance generation and signing (provenance spec).
- npm registry publish behavior (provenance and publish spec).
- GitHub Release asset distribution (publisher and composition specs).

## One package per run

The profile produces exactly one package per run. The package is selected by the `package-directory`
input.

## Package directory resolution

- `package-directory` is required.
- `package-directory` must be a repository-root-relative directory. `.` selects the repository root.
- The resolved package directory must stay inside the checked-out source repository after path
  normalization. Absolute paths, empty paths, path traversal outside the repository, and paths whose
  basename cannot be resolved to a directory must fail before reading `package.json`.
- The resolved package directory must contain the selected package's `package.json`.
- If `package-directory` is `.`, the selected package is the root package.
- If `package-directory` is a subdirectory, the selected package must be either:
  - a workspace package reachable from the nearest workspace root discovered by the rules below; or
  - a standalone package in that directory when no workspace root claims it.

### Workspace root discovery

The profile discovers workspace context by starting at the resolved package directory and walking
upward to the repository root. Each ancestor is a candidate workspace root. The first candidate
whose supported workspace metadata claims the selected package directory is the workspace root.

Workspace membership is recognized from supported workspace metadata in each candidate ancestor:

- npm and Yarn: `workspaces` in the candidate root `package.json`.
- pnpm: `pnpm-workspace.yaml` in the candidate root.

The initial production profile supports only these workspace metadata shapes:

- npm and Yarn `workspaces` as an array of string patterns, for example `["packages/*", "tools/*"]`.
- npm and Yarn `workspaces` as an object whose `packages` member is an array of string patterns, for
  example `{ "packages": ["packages/*"] }`.
- pnpm `pnpm-workspace.yaml` as a YAML object whose `packages` member is an array of string
  patterns.

Workspace patterns are evaluated as candidate-workspace-root-relative, slash-separated path patterns
after path normalization. For each candidate ancestor, the implementation converts the selected
package directory to a path relative to that candidate root, then evaluates the candidate root's
workspace patterns against that relative path. Matched package directories are emitted and recorded
as repository-root-relative normalized paths after matching.

Supported pattern syntax is intentionally limited to:

- literal path segments;
- `*` for exactly one path segment;
- `**` for zero or more path segments.

The initial production profile rejects workspace patterns with negation (`!`), brace expansion,
extended glob syntax, absolute paths, empty path segments, `.` or `..` traversal segments, backslash
separators, or patterns that normalize outside the candidate workspace root or repository. A future
ADR or spec revision may add broader package-manager-native workspace pattern semantics.

Examples:

- With repository root `/repo`, candidate root `/repo`, pattern `packages/*`, and
  `package-directory: packages/a`, the relative path `packages/a` matches and the recorded package
  path is `packages/a`.
- With repository root `/repo`, candidate root `/repo/apps/web`, pattern `packages/*`, and
  `package-directory: apps/web/packages/a`, the relative path `packages/a` matches and the recorded
  package path is `apps/web/packages/a`.
- With repository root `/repo`, candidate root `/repo/apps/web`, pattern `apps/web/packages/*`, and
  `package-directory: apps/web/packages/a`, the relative path `packages/a` does not match. The
  pattern is rejected for that candidate if it was written with repository-root-relative assumptions
  that escape or duplicate the candidate root.

If multiple ancestors claim the selected package directory, the nearest claiming ancestor is the
workspace root. If no ancestor claims the selected package directory, the selected package is
treated as a standalone package and the selected package directory is its package root.

The profile must fail before install when a workspace metadata file is malformed, when workspace
membership cannot identify exactly one selected package, or when the selected package directory is
claimed by workspace metadata but lacks its own `package.json`.

If the selected package directory matches more than one supported pattern within the nearest
claiming workspace root, that is still one selected package when every matching pattern resolves to
the same normalized directory. If different patterns or metadata files identify different package
directories for the same `package-directory` input, the profile must fail before install with a
workspace resolution error.

## Package manager selection

The profile selects the package manager from the selected package manifest first, then from the
workspace root manifest when the selected package is a workspace package, and finally from lockfile
inference. The selection order is:

1. `packageManager` field in the selected package `package.json`.
2. `devEngines.packageManager` field in the selected package `package.json`.
3. `packageManager` field in the workspace root `package.json`, when a workspace root exists.
4. `devEngines.packageManager` field in the workspace root `package.json`, when a workspace root
   exists.
5. Lockfile inference from the package manager root.

The package manager root is the workspace root when a workspace root exists; otherwise it is the
selected package directory. Install commands run from the package manager root. Build and pack
commands must target the selected package only and must not build, pack, or publish sibling
workspace packages.

The selected source must be recorded in provenance with enough path information for strict verifier
matching. When manifest metadata selects the package manager, provenance must record the
repository-root-relative manifest path that supplied the selected field. When lockfile inference
selects npm, provenance must record the repository-root-relative lockfile path that supplied the
selection. A basename such as `package.json` is not sufficient by itself for workspace packages
because both the selected package and the workspace root may have manifests with that basename.

### `packageManager` field

- Format: `name@version`, for example `pnpm@9.1.0` or `yarn@4.1.0`.
- If the field selects pnpm or Yarn, the profile must use the exact package manager and version.
- If the field selects npm, the profile selects npm but uses the npm CLI bundled with the selected
  Node.js 24 toolchain; the manifest npm version must not override the builder-owned npm runtime.
- If the field is absent in the current manifest source, the profile falls back to the next source.

### `devEngines.packageManager` field

- Format: `name@version` or `name`.
- If the field selects pnpm or Yarn and omits a version, the profile fails because exact version
  enforcement is required for pnpm and Yarn.
- If the field selects pnpm or Yarn and specifies a version, the profile uses the exact package
  manager and version.
- If the field selects npm, the profile selects npm but uses the npm CLI bundled with the selected
  Node.js 24 toolchain; `devEngines.packageManager` must not override the builder-owned npm runtime.

### Lockfile inference

The profile infers the package manager from lockfiles in the package manager root only when all
selected-package and workspace-root package-manager manifest fields are absent:

| Lockfile            | Inferred package manager |
| ------------------- | ------------------------ |
| `package-lock.json` | npm                      |
| `pnpm-lock.yaml`    | pnpm                     |
| `yarn.lock`         | Yarn                     |

Lockfile inference has different outcomes by package manager:

- `package-lock.json` may select `npm`; the actual npm version is the npm CLI bundled with the
  selected Node.js 24 toolchain.
- `pnpm-lock.yaml` may identify `pnpm`, but the release build must fail because ADR 0017 requires an
  exact pnpm version from selected manifest metadata.
- `yarn.lock` may identify Yarn, but the release build must fail because ADR 0017 requires an exact
  Yarn version from selected manifest metadata.

Lockfile-only pnpm or Yarn projects must add exact release package-manager metadata to either the
selected package manifest or the workspace root manifest before using the production profile.

### Conflict handling

If the selected sources disagree in a way that cannot be resolved, the profile must fail. The
selection source is chosen by the ordered manifest-first rules above, but lockfiles in the package
manager root still constrain whether the selected package manager can run a reproducible release
install.

The initial production profile applies this decision matrix after parsing the selected package and
workspace metadata:

| Manifest selection state                                   | Lockfiles in package manager root                  | Result                                                                                                              |
| ---------------------------------------------------------- | -------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| Selected manager is `npm` from manifest metadata           | exactly `package-lock.json`                        | Use npm bundled with Node.js 24.                                                                                    |
| Selected manager is `npm` from manifest metadata           | no lockfile                                        | Fail before install because `npm ci` requires `package-lock.json`.                                                  |
| Selected manager is `npm` from manifest metadata           | `package-lock.json` plus pnpm or Yarn lockfiles    | Use npm bundled with Node.js 24; treat non-selected lockfiles as ignored stale diagnostics.                         |
| Selected manager is `npm` from manifest metadata           | pnpm or Yarn lockfiles without `package-lock.json` | Fail before install because `npm ci` requires `package-lock.json`; non-npm lockfiles must not select npm.           |
| Selected manager is `pnpm` from manifest metadata          | exactly `pnpm-lock.yaml`                           | Use the exact pnpm version from the selected manifest metadata through Corepack.                                    |
| Selected manager is `pnpm` from manifest metadata          | no lockfile                                        | Fail before install because frozen pnpm install requires `pnpm-lock.yaml`.                                          |
| Selected manager is `pnpm` from manifest metadata          | `pnpm-lock.yaml` plus npm or Yarn lockfiles        | Use the exact pnpm version through Corepack; treat non-selected lockfiles as ignored stale diagnostics.             |
| Selected manager is `pnpm` from manifest metadata          | npm or Yarn lockfiles without `pnpm-lock.yaml`     | Fail before install because frozen pnpm install requires `pnpm-lock.yaml`; non-pnpm lockfiles must not select pnpm. |
| Selected manager is `yarn` from manifest metadata          | exactly `yarn.lock`                                | Use the exact Yarn version from the selected manifest metadata through Corepack.                                    |
| Selected manager is `yarn` from manifest metadata          | no lockfile                                        | Fail before install because frozen Yarn install requires `yarn.lock`.                                               |
| Selected manager is `yarn` from manifest metadata          | `yarn.lock` plus npm or pnpm lockfiles             | Use the exact Yarn version through Corepack; treat non-selected lockfiles as ignored stale diagnostics.             |
| Selected manager is `yarn` from manifest metadata          | npm or pnpm lockfiles without `yarn.lock`          | Fail before install because frozen Yarn install requires `yarn.lock`; non-Yarn lockfiles must not select Yarn.      |
| No manifest metadata selects a manager                     | exactly `package-lock.json`                        | Infer npm from the lockfile and use npm bundled with Node.js 24.                                                    |
| No manifest metadata selects a manager                     | exactly `pnpm-lock.yaml`                           | Fail because pnpm requires an exact version from manifest metadata.                                                 |
| No manifest metadata selects a manager                     | exactly `yarn.lock`                                | Fail because Yarn requires an exact version from manifest metadata.                                                 |
| No manifest metadata selects a manager                     | no supported lockfile                              | Fail because the package manager cannot be selected reproducibly.                                                   |
| No manifest metadata selects a manager                     | more than one supported lockfile                   | Fail with a package-manager conflict.                                                                               |
| Any manifest field selects an unsupported manager          | any lockfile state                                 | Fail before install; lockfiles must not override an unsupported manifest-selected manager.                          |
| Manifest fields at different selection priorities disagree | any lockfile state                                 | Fail with a package-manager conflict; lower-priority metadata must not silently override higher-priority metadata.  |

Examples of conflicts:

- `packageManager` says `pnpm@9.1.0` but no `pnpm-lock.yaml` exists.
- `packageManager` says `npm` but no `package-lock.json` exists.
- `packageManager` says `yarn@4.1.0` but no `yarn.lock` exists.
- The selected package manifest says `pnpm@9.1.0` but the workspace root manifest says `yarn@4.1.0`.
- Multiple lockfiles exist and no `packageManager` field resolves the ambiguity.

When the result is a package-manager conflict, provenance must not be emitted. When lockfile
inference selects npm, `package_manager.selection_source` is `lockfile`,
`package_manager.selection_manifest` and `package_manager.selection_manifest_path` are `null`, and
`package_manager.selection_lockfile_path` is the repository-root-relative path to
`package-lock.json`.

When manifest metadata selects a package manager and that manager's required lockfile is present,
that selected lockfile is the only lockfile that constrains the release install. Other supported
lockfile names in the package manager root are ignored as stale diagnostics. The workflow should
warn about those ignored lockfiles and record their repository-root-relative paths in provenance,
but reviewers and verifiers must not treat their presence as a package-manager conflict or as an
input to the dependency graph used by the selected package manager's frozen install command.

## Corepack for pnpm and Yarn

- pnpm and Yarn must be managed by Corepack.
- Corepack must activate the exact version specified in the selected manifest metadata.
- Corepack's Known Good Release fallback is prohibited for release builds.
- If the exact version cannot be enforced, the profile fails.

## npm behavior

- npm is the package manager bundled with the selected Node.js toolchain.
- The profile uses the npm version that comes with Node.js 24.
- No separate npm version override is supported.

## Command matrix

The profile runs install commands from the package manager root and build/pack commands against the
selected package only. Command execution must use the selected package manager from the policy
above; callers cannot override these commands.

For a root package or standalone package where `package.directory` equals `package_manager.root`,
the profile runs:

| Step    | npm                         | pnpm                             | Yarn                             |
| ------- | --------------------------- | -------------------------------- | -------------------------------- |
| Install | `npm ci`                    | `pnpm install --frozen-lockfile` | `yarn install --frozen-lockfile` |
| Build   | `npm run build` if declared | `pnpm run build` if declared     | `yarn run build` if declared     |
| Pack    | `npm pack`                  | `pnpm pack`                      | `yarn pack`                      |

For a workspace package where `package.directory` differs from `package_manager.root`, the profile
runs install from the workspace root, then targets the selected workspace package with these command
templates:

| Step    | npm                                                                 | pnpm                                                                    | Yarn                                                             |
| ------- | ------------------------------------------------------------------- | ----------------------------------------------------------------------- | ---------------------------------------------------------------- |
| Install | `npm ci`                                                            | `pnpm install --frozen-lockfile`                                        | `yarn install --frozen-lockfile`                                 |
| Build   | `npm --workspace <package-directory> run build` if declared         | `pnpm --filter "{./<package-directory>}" run build` if declared         | `yarn workspace <package-name> run build` if declared            |
| Pack    | `npm pack --workspace <package-directory> --pack-destination <dir>` | `pnpm --filter "{./<package-directory>}" pack --pack-destination <dir>` | `yarn workspace <package-name> pack --out <tarball-output-path>` |

Command template variables have these meanings:

- `<package-directory>` is the normalized repository-root-relative selected package directory.
- `<package-name>` is the selected package's validated `name` field from its source manifest.
- `<dir>` is a trusted empty temporary directory created by the workflow for the pack output.
- `<tarball-output-path>` is the exact expected tarball file path inside that trusted temporary
  directory.

### Packed tarball name

The authoritative tarball name is the basename of the single file produced by the selected package
manager's pack command in the trusted pack output directory. The profile must use that basename
unchanged as the package tarball name, provenance subject name, public `package-tarball-name`
output, and any downstream release asset name unless a later profile explicitly defines a signed
rename mapping.

The initial npm package profile expects npm-compatible pack output with a `.tgz` suffix. It must not
rename the pack-produced tarball to `.tar.gz`, reinterpret a local path as the tarball name, or use
a registry URL, package identity, or workflow artifact name as the tarball name. If the selected
pack command produces no file, more than one file, a file whose basename is unsafe, or a package
tarball whose basename does not end in `.tgz`, the profile must fail before signing or publishing.

The profile must fail before build or pack when the selected package manager cannot target exactly
the selected package with the command template above, when the package manager reports that the
target matches zero or multiple workspaces, or when the resulting packed tarball is not the only
file created in the trusted pack output directory. A workspace build or pack command must not run
against all workspaces and must not rely on the current working directory alone to select the
package.

### Build script detection

- The profile reads the `scripts` field of `package.json`.
- If `build` is declared, the profile runs it.
- If `build` is not declared, the profile performs a successful no-op build step.
- The profile must not fail when the `build` script is missing.
- The profile must not run an arbitrary fallback command.

## Source manifest validation

The profile reads and records the following fields from the source `package.json`:

- `name`
- `version`
- `private`
- `packageManager`
- `devEngines.packageManager`
- `publishConfig`
- Workspace metadata if applicable

The profile must fail before packing when `private` is `true`. The initial release profile publishes
public npm package releases only; it does not support a pack-only, provenance-only, or no-publish
mode for private packages.

## Packed artifact inspection

The packed artifact is the authoritative source of package contents for publish time validation. The
profile must:

1. Extract the packed `package/package.json`.
2. Read the packed `name` and `version`.
3. Compare packed `name` and `version` with the source `package.json`.
4. Fail if they differ.

### Packed metadata fields

- `name`
- `version`
- Packed file list
- Consumer-surface fields when present

## Metadata fields recorded but not semantically validated

The profile records additional metadata for provenance and diagnostics but does not treat it as a
trust root. For example:

- `license`
- `description`
- `keywords`
- `author`
- `repository`
- `homepage`

These fields are stored as-is and may be used for diagnostics.

## Failure behavior

The profile must fail before packing when:

- `package.json` is missing or invalid.
- `name` or `version` is missing.
- `private` is `true`.
- `package-directory` resolves outside the repository, is not a directory, or does not identify
  exactly one selected package.
- Package manager selection is ambiguous.
- An exact package manager version cannot be determined for pnpm or Yarn.
- Lockfile is missing for npm `npm ci` or pnpm/Yarn `--frozen-lockfile`.
- Source and packed `name`/`version` mismatch.
- Pack command fails.

## TDD and fixtures

- Fixture matrix across npm, pnpm, and Yarn.
- Root package and workspace package cases.
- Missing lockfile, conflicting lockfiles, and missing `packageManager` version.
- Malformed workspace metadata, unsupported workspace patterns, and ambiguous workspace ownership.
- Workspace command targeting failures for npm, pnpm, and Yarn where the command matches zero,
  multiple, or sibling packages.
- Missing `build` script (successful no-op).
- Source/packed identity mismatch.
- Private package rejection before pack.
- Workspace package using root package-manager metadata and root lockfile.
- Corepack exact version enforcement failure.
