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
  [0033](../decisions/0033-run-build-script-only-when-declared.md),
  [0056](../decisions/0056-treat-non-selected-lockfiles-as-stale-diagnostics.md),
  [0063](../decisions/0063-limit-yarn-support-to-berry-v4-with-corepack-package-manager.md),
  [0064](../decisions/0064-use-npm-purl-subject-with-sha512-and-sha256.md), and
  [0070](../decisions/0070-record-package-manager-distributions-and-runner-image-in-resolved-dependencies.md)
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

### Path normalization and comparison

In accordance with ADR 0023's requirement that `package-directory` identify one exact
repository-root-relative package directory, package and workspace paths use this canonicalization:

1. `package-directory` must use `/` separators and be normalized lexically by collapsing repeated
   separators, removing `.` segments (including a leading `./`), and removing trailing separators;
   the repository root canonicalizes to `.`. An absolute path, backslash separator, `..` segment, or
   value that is empty after normalization must fail before workspace discovery.
2. The repository root and normalized package directory must then be resolved to real paths,
   following every symbolic link, before containment checks, relative-path derivation, equality
   comparison, or workspace-pattern matching. A broken or cyclic link, an unresolvable component, or
   a resolved path outside the real repository root must fail before reading package metadata.
3. Every caller-supplied literal path segment must match the corresponding directory entry with an
   exact, case-sensitive UTF-8 byte comparison, and every literal workspace-pattern segment must use
   the same comparison. A case mismatch must fail package resolution even on a case-insensitive
   filesystem; it must not be corrected or accepted from filesystem lookup results.
4. Directory identity and workspace membership must compare the repository-root-relative real path,
   not the pre-resolution lexical spelling. Real-path comparison prevents a symbolic-link alias from
   bypassing repository containment or naming a different package; if the selected real directory
   cannot be matched unambiguously, the profile must fail before install with a package-resolution
   error.

Valid matching examples:

- With actual directory `pkg/a`, `package-directory: ./pkg//a/` canonicalizes to `pkg/a` and selects
  that directory.
- With in-repository symbolic link `linked-a` pointing to `packages/a`,
  `package-directory: linked-a` resolves to canonical package path `packages/a` and pattern
  `packages/*` matches that real path.

Invalid matching examples:

- A symbolic link `linked-a` whose target is outside the real repository root fails containment
  before package metadata is read.
- With actual directory `packages/a`, `package-directory: Packages/A` fails the exact-case check
  even on a case-insensitive filesystem.
- A broken symbolic link or a case-variant literal pattern such as `Packages/*` does not match
  `packages/a`; package resolution fails when that mismatch prevents an unambiguous selection.

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

Pattern evaluation is against package directory paths, not arbitrary files. A pattern match is a
candidate package only when the matched path is a directory inside the repository and that directory
contains the selected package's `package.json`. A pattern that matches an ancestor, descendant, or
non-directory path of the selected `package-directory` does not claim the selected package unless
the matched normalized directory is exactly the selected package directory. The profile must not
scan all matching workspace directories and then choose a different package from the caller-selected
`package-directory`.

Supported pattern syntax is intentionally limited to:

- literal path segments;
- `*` for exactly one path segment;
- `**` for zero or more path segments.

The `*` segment is not recursive: `packages/*` matches `packages/a` but not `packages/a/b`. The `**`
segment is recursive and may match zero path segments: `packages/**` matches `packages/a` and
`packages/a/b` when those exact directories are the selected package directory and contain their own
`package.json`; `**` may match the candidate workspace root itself only when the selected
`package-directory` is that candidate root and that root is a valid package. Literal segments must
match exactly after slash normalization. No package-manager-native expansion beyond these three
segment types is part of the initial production profile.

The initial production profile rejects workspace patterns with negation (`!`), brace expansion,
extended glob syntax, absolute paths, empty path segments, `.` or `..` traversal segments, backslash
separators, or patterns that normalize outside the candidate workspace root or repository. A future
ADR or spec revision may add broader package-manager-native workspace pattern semantics.

Examples:

- With repository root `/repo`, candidate root `/repo`, pattern `packages/*`, and
  `package-directory: packages/a`, the relative path `packages/a` matches and the recorded package
  path is `packages/a`.
- With repository root `/repo`, candidate root `/repo`, pattern `packages/*`, and
  `package-directory: packages/a/b`, the relative path `packages/a/b` does not match because `*`
  matches exactly one segment.
- With repository root `/repo`, candidate root `/repo`, pattern `packages/**`, and
  `package-directory: packages/a/b`, the relative path `packages/a/b` matches only if
  `/repo/packages/a/b/package.json` exists.
- With repository root `/repo`, candidate root `/repo/apps/web`, pattern `**`, and
  `package-directory: apps/web`, the zero-segment match claims the candidate root only if
  `/repo/apps/web/package.json` exists and the selected package directory is exactly `apps/web`.
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

Malformed workspace metadata includes a root `package.json` whose `workspaces` value uses an
unsupported shape, a `pnpm-workspace.yaml` file that is not a YAML object, a `pnpm-workspace.yaml`
object whose `packages` member is missing or is not an array of strings, and any workspace pattern
that uses unsupported syntax such as negation, brace expansion, extended glob syntax, absolute
paths, empty path segments, traversal segments, or backslash separators. A malformed metadata file
at a candidate root that would otherwise be considered for workspace ownership must fail closed
rather than being ignored in favor of a farther ancestor or standalone package mode.

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
- If the field selects Yarn, the descriptor must use an exact SemVer version greater than or equal
  to `4.0.0`. Yarn Classic 1.x, Yarn Berry v2, Yarn Berry v3, ranges, tags, URLs, hash-suffixed
  descriptors, and omitted versions are rejected before install.
- If the field selects npm, the profile selects npm but uses the npm CLI bundled with the selected
  Node.js 24 toolchain; the manifest npm version must not override the builder-owned npm runtime.
- If the field is absent in the current manifest source, the profile falls back to the next source.

### `devEngines.packageManager` field

- Format: a JSON object with required `name` and optional `version` and `onFail` members. This
  follows the package manager manifest shape documented by npm, pnpm, and Corepack for
  `devEngines.packageManager`; the initial production profile does not accept string or array forms
  for this field.
- `name` must be `npm`, `pnpm`, or `yarn`.
- `version`, when present, must be a JSON string.
- `onFail`, when present, must be `ignore`, `warn`, `error`, or `download`. The value is diagnostic
  metadata only for this production profile and must not weaken release-build enforcement.
- Unknown members are rejected.
- If the field selects pnpm, `version` is required and must be an exact SemVer version. Ranges,
  tags, URLs, hash-suffixed package-manager descriptors, and omitted versions are rejected because
  ADR 0017 prohibits release-time range resolution and Corepack Known Good Release fallback.
- If the field selects Yarn, the stable initial profile rejects it before install. Yarn support
  requires an exact Yarn Berry v4 or newer descriptor in a top-level `packageManager` field;
  `devEngines.packageManager` alone is not a Yarn selection source.
- If the field selects npm, the profile selects npm but uses the npm CLI bundled with the selected
  Node.js 24 toolchain; `devEngines.packageManager.version` must not override the builder-owned npm
  runtime.
- If `onFail` is `ignore` or `warn`, the profile still fails closed on package-manager policy
  violations such as an unsupported name, missing exact pnpm/Yarn version, package-manager mismatch,
  or required lockfile mismatch.

Examples:

```json
{
  "devEngines": {
    "packageManager": {
      "name": "pnpm",
      "version": "11.9.0",
      "onFail": "download"
    }
  }
}
```

Rejected examples:

- `"devEngines": { "packageManager": "pnpm@11.9.0" }` because string form is not part of the initial
  production profile contract.
- `"devEngines": { "packageManager": { "name": "pnpm" } }` because pnpm requires an exact version.
- `"devEngines": { "packageManager": { "name": "pnpm", "version": ">=11 <12" } }` because release
  builds must not resolve ranges.
- `"devEngines": { "packageManager": [{ "name": "pnpm", "version": "11.9.0" }] }` because array form
  is ambiguous for a one-package-manager release profile.

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
- `yarn.lock` may identify Yarn, but the release build must fail because stable Yarn support
  requires an exact Yarn Berry v4 or newer descriptor from a top-level `packageManager` field.

Lockfile-only pnpm projects must add exact release package-manager metadata to either the selected
package manifest or the workspace root manifest before using the production profile. Lockfile-only
Yarn projects must add top-level exact `packageManager` metadata selecting Yarn Berry v4 or newer.

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
| Selected manager is `yarn` from top-level `packageManager` | exactly `yarn.lock`                                | Use the exact Yarn Berry v4+ version from `packageManager` through Corepack.                                        |
| Selected manager is `yarn` from manifest metadata          | no lockfile                                        | Fail before install because frozen Yarn install requires `yarn.lock`.                                               |
| Selected manager is `yarn` from top-level `packageManager` | `yarn.lock` plus npm or pnpm lockfiles             | Use the exact Yarn Berry v4+ version through Corepack; treat non-selected lockfiles as ignored stale diagnostics.   |
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

The selected lockfile path and digest feed the name-keyed `lockfile` descriptor defined in the
[provenance and publish specification](js-ts-npm-provenance-publish.md#jsts-npm-resolveddependencies-schema).
`externalParameters.package_manager` records how the package manager was selected. The selected
manager name and exact effective version select the unordered dependency set by descriptor `name`:
npm emits `lockfile` and `runner-image`, while pnpm and Yarn each additionally emit exactly one
`package-manager-distribution`. The profile must not select, create, or validate a descriptor by
array position. A missing, duplicate, unexpected, or manager-incompatible descriptor must stop
before signing with the descriptor diagnostic defined in the provenance and publish specification.
Stale non-selected lockfiles may be copied into both
`externalParameters.package_manager.ignored_lockfile_paths` and the lockfile descriptor annotations
as diagnostics, but they must not become separate dependency descriptors or selected graph inputs.

## Corepack for pnpm and Yarn

- pnpm and Yarn must be managed by Corepack.
- Corepack must activate the exact version specified in the selected manifest metadata.
- Corepack's Known Good Release fallback is prohibited for release builds.
- If the exact version cannot be enforced, the profile fails.
- Yarn must be Yarn Berry v4 or newer and selected from a top-level `packageManager` field. The
  profile must fail before install if Yarn would run from an ambient global installation, Corepack
  Known Good Release fallback, `devEngines.packageManager` alone, a version range, or `yarn.lock`
  without top-level exact `packageManager` metadata.

### Corepack distribution capture

ADR 0070 records the distribution that supplied the package manager executed by the build. Before
Corepack acquires pnpm or Yarn, the profile must know the selected manager's exact version from the
package-manager selection result. If that version is unavailable, non-exact, or differs from the
executed manager, the profile must stop before predicate construction with
`windlass.verify.error.resolved-dependencies-package-manager-distribution` and exit code `1`.

During that exact-version acquisition, the profile must capture the actual Corepack distribution URL
that supplied the executed manager. It must create the name-keyed `package-manager-distribution`
descriptor only for pnpm or Yarn, with `package_manager`, `package_manager_version`, and
`acquisition_source: "corepack"` annotations. The captured URL, selected manager, and executed
manager must agree. Missing capture evidence stops before predicate construction with
`windlass.verify.error.input-unavailable` and exit code `2`; a captured disagreement or malformed
descriptor stops with `windlass.verify.error.resolved-dependencies-package-manager-distribution` and
exit code `1`.

- For pnpm, the profile must obtain the SHA-512 `dist.integrity` value from authenticated registry
  metadata for the actual Corepack distribution, decode the `sha512-<base64>` SRI value to bytes,
  and encode those bytes as exactly 128 lowercase hexadecimal characters in `digest.sha512`. Its
  `digest_authority` annotation must be `registry-integrity`. Missing, non-SHA-512, undecodable, or
  nonmatching registry integrity evidence stops before predicate construction with
  `windlass.verify.error.input-unavailable` and exit code `2`; a captured wrong-authority or
  malformed descriptor stops with
  `windlass.verify.error.resolved-dependencies-package-manager-distribution` and exit code `1`.
- For Yarn, the profile must calculate SHA-512 over the actual bytes downloaded by Corepack for the
  executed distribution and encode the result as exactly 128 lowercase hexadecimal characters in
  `digest.sha512`. Its `digest_authority` annotation must be `download-hash`. Unavailable downloaded
  bytes or hash evidence stops before predicate construction with
  `windlass.verify.error.input-unavailable` and exit code `2`; a captured wrong authority, wrong
  version, or malformed descriptor stops with
  `windlass.verify.error.resolved-dependencies-package-manager-distribution` and exit code `1`.

The Corepack path must not use a Known Good Release fallback, an ambient pnpm or ambient Yarn
installation, or a registry override that changes the ADR-selected acquisition path. Each condition
must stop before predicate construction with
`windlass.verify.error.resolved-dependencies-package-manager-distribution` and exit code `1`. The
profile must not replace missing capture evidence with a guessed distribution URL or a manifest
hash. That condition must stop before predicate construction with
`windlass.verify.error.input-unavailable` and exit code `2`.

## Runner image capture

ADR 0070 records the GitHub-hosted runner image that supplied the Node.js runtime. At build time,
the profile must observe `ImageOS`, `ImageVersion`, and the exact output of `node --version`. It
must read `/imagegeneration/imagedata.json` verbatim as a JSON array and select the entry whose
`group` is `Runner Image`. That entry's `detail` must contain `Image: <label>`,
`Version: <full-version>`, `Included Software: <url>`, and `Image Release: <url>` lines. If the file
is unavailable or malformed, the group or detail lines are unavailable or malformed, the image label
is inconsistent with `$ImageOS`, `Version` differs from `$ImageVersion`, or the Included Software
URL is absent or empty, the profile must stop before predicate construction with
`windlass.verify.error.input-unavailable` and exit code `2`.

The canonical name-keyed `runner-image` descriptor `uri` must be the non-empty Included Software URL
read verbatim from that file. Its annotations must be `image_os: $ImageOS`,
`image_version: $ImageVersion`, and `node_version` equal to the exact observed `node --version`
output. The profile must not derive, reconstruct, or rewrite an undocumented runner URL from
`ImageOS`, `ImageVersion`, or another source. A captured malformed runner descriptor, including a
digest member or a mismatch with these observations, must stop before predicate construction with
`windlass.verify.error.resolved-dependencies-runner-image` and exit code `1`.

No network validation of the Included Software URL is required. If GitHub removes or reshapes the
file, producers fail closed before signing with `windlass.verify.error.input-unavailable` and exit
code `2`; they must not emit guessed provenance.

## Capture timing and signing boundary

The profile must fix the selected manager, exact executed version, Corepack distribution evidence,
runner observations, and named dependency descriptors before candidate predicate construction and
before signing. If any required evidence becomes unavailable before predicate construction, the
profile must stop with `windlass.verify.error.input-unavailable` and exit code `2`. It must not
construct, sign, or publish a predicate after a capture failure. If evidence is captured but
contradicts the selected manager or version, or produces malformed descriptor values, the profile
must stop with the corresponding narrow descriptor diagnostic and exit code `1`.

## Yarn install mode

The stable initial Yarn path uses Yarn Berry v4 or newer in immutable install mode.
`yarn install --immutable` is the default Yarn install command for the initial profile. If a future
profile allows another reproducible Yarn install mode, that mode must be specified in this section
before release builds may use it.

The profile records the effective Yarn install mode in provenance so producer-side and consumer-side
verification can distinguish a supported immutable install from an unsupported fallback or ambient
Yarn invocation.

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

| Step    | npm                                                   | pnpm                                                   | Yarn                                    |
| ------- | ----------------------------------------------------- | ------------------------------------------------------ | --------------------------------------- |
| Install | `npm ci`                                              | `pnpm install --frozen-lockfile`                       | `yarn install --immutable`              |
| Build   | `npm run build` if declared                           | `pnpm run build` if declared                           | `yarn run build` if declared            |
| Pack    | `npm pack --pack-destination <pack-output-directory>` | `pnpm pack --pack-destination <pack-output-directory>` | `yarn pack --out <tarball-output-path>` |

For a workspace package where `package.directory` differs from `package_manager.root`, the profile
runs install from the workspace root, then targets the selected workspace package with these command
templates:

| Step    | npm                                                                                                                 | pnpm                                                                                                                    | Yarn                                                             |
| ------- | ------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| Install | `npm ci`                                                                                                            | `pnpm install --frozen-lockfile`                                                                                        | `yarn install --immutable`                                       |
| Build   | `npm --workspace <package-manager-root-relative-package-directory> run build` if declared                           | `pnpm --filter "{./<package-manager-root-relative-package-directory>}" run build` if declared                           | `yarn workspace <package-name> run build` if declared            |
| Pack    | `npm pack --workspace <package-manager-root-relative-package-directory> --pack-destination <pack-output-directory>` | `pnpm --filter "{./<package-manager-root-relative-package-directory>}" pack --pack-destination <pack-output-directory>` | `yarn workspace <package-name> pack --out <tarball-output-path>` |

Command template variables have these meanings:

- `<package-directory>` is the normalized repository-root-relative selected package directory. It is
  a specification value, not a command path argument when the package manager root is nested.
- `<package-manager-root-relative-package-directory>` is the normalized selected package directory
  relative to `package_manager.root`. The npm workspace path and pnpm path selector are resolved
  from the package manager root because every command in this matrix starts there. For example, when
  `package.directory` is `apps/web/packages/a` and `package_manager.root` is `apps/web`, both
  arguments are `packages/a`, not `apps/web/packages/a`.
- `<package-name>` is the selected package's validated `name` field from its source manifest.
- `<pack-output-directory>` is the absolute real path of a trusted empty temporary directory created
  by the workflow for the pack output. npm and pnpm resolve `--pack-destination` from their command
  context, while Yarn resolves `--out` from its selected workspace context. The absolute path makes
  all three commands select the same directory without relying on a package-manager-specific working
  directory.
- `<tarball-output-path>` is the absolute real path of the exact expected tarball inside
  `<pack-output-directory>`. Yarn accepts no workspace path argument: `yarn workspace` selects only
  by `<package-name>`, and its absolute `--out` path is therefore independent of both repository and
  package-manager roots.

### Packed tarball name

The authoritative tarball name is the basename of the single file produced by the selected package
manager's pack command in the trusted pack output directory. The profile must use that basename
unchanged as the package tarball name, public `package-tarball-name` output, and any downstream
release asset name unless a later profile explicitly defines a signed rename mapping. The npm
provenance Statement subject name is the package Package URL defined by the provenance and publish
spec, not this tarball basename.

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
- `repository`
- Workspace metadata if applicable

The profile must fail before packing when `private` is `true`. The initial release profile publishes
public npm package releases only; it does not support a pack-only, provenance-only, or no-publish
mode for private packages.

### Repository identity validation

`repository` is required verifier-relevant metadata. The profile must normalize it, compare the
result with the observed caller repository identity, and fail before packing with
`package-repository-identity-mismatch` when it is missing, malformed, unsupported, or different. The
observed caller repository identity is the GitHub owner and repository observed from the caller
workflow context. It uses the canonical source repository URL rules in the
[provenance and publish specification](js-ts-npm-provenance-publish.md#canonical-source-repository-url).

The accepted `repository` JSON forms are closed:

- string shorthand `owner/repository`;
- string shorthand `github:owner/repository`;
- `https://github.com/owner/repository`, `git+https://github.com/owner/repository`, or
  `git://github.com/owner/repository`;
- SCP-like `git@github.com:owner/repository.git`;
- `ssh://git@github.com/owner/repository.git`; or
- a closed object with exactly `type: "git"` and string `url`, plus optional string `directory`,
  where `url` is one of the accepted string forms above.

Every accepted form normalizes to exactly:

```text
https://github.com/<lowercase-owner>/<lowercase-repository>
```

The normalizer lowercases the owner and repository. It may remove one terminal `.git` suffix and one
terminal `/` from an accepted URL path, then requires exactly two non-empty path segments. The
canonical output has an `https` scheme, `github.com` host, no trailing slash, no `.git` suffix, no
userinfo, no port, no query, no fragment, and no extra path segment. Comparison is case-insensitive
for GitHub owner and repository names before this lowercase canonical form is emitted.

The optional object `directory` records a diagnostic package location only. It does not contribute
to repository identity and cannot alter normalization or comparison.

The profile rejects missing, empty, or non-string/non-object values; non-GitHub hosts; non-`git`
object types; credentials; ports; query or fragment components; backslashes; path traversal;
percent-encoded separators; empty path segments; malformed shorthand; extra path segments; and
ambiguous object shapes. It also rejects a `.git` suffix or trailing slash anywhere other than the
permitted terminal path position. It does not accept arbitrary repository URLs that npm may accept.

These values are valid and normalize as shown:

| Source `repository` value                                                                                    | Canonical repository identity             |
| ------------------------------------------------------------------------------------------------------------ | ----------------------------------------- |
| `WindlassTech/Example`                                                                                       | `https://github.com/windlasstech/example` |
| `github:WindlassTech/Example`                                                                                | `https://github.com/windlasstech/example` |
| `git+https://github.com/WindlassTech/Example.git`                                                            | `https://github.com/windlasstech/example` |
| `git://github.com/WindlassTech/Example/`                                                                     | `https://github.com/windlasstech/example` |
| `git@github.com:WindlassTech/Example.git`                                                                    | `https://github.com/windlasstech/example` |
| `{ "type": "git", "url": "ssh://git@github.com/WindlassTech/Example.git", "directory": "packages/example" }` | `https://github.com/windlasstech/example` |

The following are invalid and fail before packing with `package-repository-identity-mismatch`:

| Source `repository` value                                                                | Reason                                               |
| ---------------------------------------------------------------------------------------- | ---------------------------------------------------- |
| missing or `""`                                                                          | Repository identity is required.                     |
| `gitlab:WindlassTech/Example`                                                            | Unsupported host and shorthand.                      |
| `https://github.com/WindlassTech/Example/releases`                                       | Extra path segment.                                  |
| `https://github.com/WindlassTech/Example?tab=readme`                                     | Query is prohibited.                                 |
| `https://token@github.com/WindlassTech/Example`                                          | Credentials are prohibited.                          |
| `https://github.com:443/WindlassTech/Example`                                            | Port is prohibited.                                  |
| `https://github.com/WindlassTech/%2E%2E/Example`                                         | Path traversal and encoded separator are prohibited. |
| `{ "type": "svn", "url": "https://github.com/WindlassTech/Example" }`                    | Object type must be `git`.                           |
| `{ "type": "git", "url": "https://github.com/WindlassTech/Example", "name": "example" }` | Object shape is ambiguous because `name` is unknown. |

For example, a selected package declaring `github:OtherOrg/Example` while the observed caller
repository is `WindlassTech/Example` fails before packing with
`package-repository-identity-mismatch`; neither install, build, nor pack may start.

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

## Diagnostic package metadata

The profile may route only the following raw source-manifest values to the closed
`diagnostic_metadata.package_manifest` report surface defined by the
[verification policy and fixtures](verification-policy-and-fixtures.md#producer-diagnostic-metadata-extension):

- `repository`
- `license`
- `description`
- `keywords`
- `author`
- `homepage`

Absent values are omitted. Their raw JSON forms must follow that report surface's closed,
source-preserving rules. Raw values are diagnostic-only and must not change trust decisions or enter
SLSA `internalParameters` or `externalParameters`. The normalized repository identity defined above
is verifier-relevant, but this task does not assign it a provenance field.

## Failure behavior

The profile must emit the following diagnostic, phase, exit code, and mutation flag for each listed
failure. These producer-side checks do not mutate remote state, so `mutation_possible` is always
`false`. Bare names shown in the fixture column are stable fixture aliases only. Implementations
must emit the canonical diagnostic ID.

| Failure condition                                                                                                                                                                                                                                                                  | Diagnostic ID                                                              | Stable fixture alias                                 | Phase        | Exit code | `mutation_possible` |
| ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------- | ---------------------------------------------------- | ------------ | --------- | ------------------- |
| The selected `package.json` is missing or invalid.                                                                                                                                                                                                                                 | `windlass.verify.error.package-manifest-invalid`                           | `package-manifest-invalid`                           | policy       | `1`       | `false`             |
| The selected source manifest has no `name` or `version`.                                                                                                                                                                                                                           | `windlass.verify.error.package-metadata-required`                          | `package-metadata-required`                          | policy       | `1`       | `false`             |
| The selected source manifest sets `private` to `true`.                                                                                                                                                                                                                             | `windlass.verify.error.package-private`                                    | `package-private`                                    | policy       | `1`       | `false`             |
| `package-directory` resolves outside the repository, is not a directory, or does not identify exactly one selected package.                                                                                                                                                        | `windlass.verify.error.package-resolution-invalid`                         | `package-resolution-invalid`                         | policy       | `1`       | `false`             |
| Package-manager selection is ambiguous or conflicting.                                                                                                                                                                                                                             | `windlass.verify.error.package-manager-conflict`                           | `package-manager-conflict`                           | policy       | `1`       | `false`             |
| An exact package-manager version cannot be determined for pnpm or Yarn.                                                                                                                                                                                                            | `windlass.verify.error.package-manager-version-required`                   | `package-manager-version-required`                   | policy       | `1`       | `false`             |
| Yarn is selected from a source other than top-level `packageManager`, or its exact version is lower than `4.0.0`.                                                                                                                                                                  | `windlass.verify.error.yarn-selection-invalid`                             | `yarn-selection-invalid`                             | policy       | `1`       | `false`             |
| The selected manager's lockfile is missing for npm `npm ci`, pnpm `--frozen-lockfile`, or Yarn `--immutable`.                                                                                                                                                                      | `windlass.verify.error.required-lockfile-missing`                          | `required-lockfile-missing`                          | policy       | `1`       | `false`             |
| Required package-manager distribution or runner-image capture evidence is unavailable before predicate construction.                                                                                                                                                               | `windlass.verify.error.input-unavailable`                                  | `input-unavailable`                                  | invocation   | `2`       | `false`             |
| Captured package-manager distribution evidence has the wrong authority or version, contradicts the selected manager, uses a Known Good Release fallback or ambient manager, changes the Corepack acquisition path through a registry override, or produces a malformed descriptor. | `windlass.verify.error.resolved-dependencies-package-manager-distribution` | `resolved-dependencies-package-manager-distribution` | verification | `1`       | `false`             |
| Captured runner-image evidence has a malformed descriptor, carries a digest, or disagrees with the observed runner values.                                                                                                                                                         | `windlass.verify.error.resolved-dependencies-runner-image`                 | `resolved-dependencies-runner-image`                 | verification | `1`       | `false`             |
| `repository` is missing, malformed, unsupported, or normalizes to an identity different from the observed caller repository.                                                                                                                                                       | `windlass.verify.error.package-repository-identity-mismatch`               | `package-repository-identity-mismatch`               | policy       | `1`       | `false`             |
| Packed `name` or `version` differs from the selected source manifest.                                                                                                                                                                                                              | `windlass.verify.error.packed-package-metadata-mismatch`                   | `packed-package-metadata-mismatch`                   | verification | `1`       | `false`             |
| The selected package manager's pack command fails, produces no file or more than one file, produces an unsafe basename, or produces a basename without a `.tgz` suffix.                                                                                                            | `windlass.verify.error.package-pack-failed`                                | `package-pack-failed`                                | verification | `1`       | `false`             |

The package-repository-identity diagnostic stops before install, build, or pack. Every other policy
failure stops before the command whose precondition it violates. A verification failure stops before
signing or publishing. Exit code `2` identifies unusable locally required capture input; exit code
`1` identifies a completed policy or verification rejection.

## TDD and fixtures

- Fixture matrix across npm, pnpm, and Yarn.
- Root package and workspace package cases.
- Missing lockfile, conflicting lockfiles, and missing `packageManager` version.
- Yarn Classic, Yarn Berry v2 or v3, Yarn ranges, Yarn selected from `devEngines.packageManager`,
  and lockfile-only Yarn inference.
- Malformed workspace metadata, unsupported workspace patterns, and ambiguous workspace ownership.
- Workspace command targeting failures for npm, pnpm, and Yarn where the command matches zero,
  multiple, or sibling packages.
- Missing `build` script (successful no-op).
- Source/packed identity mismatch.
- Private package rejection before pack.
- Valid repository normalization for every accepted shorthand, URL, SSH, and object form, with each
  result equal to the observed caller repository's lowercase canonical identity.
- Rejected repository cases for missing values, malformed shorthand, non-GitHub hosts, non-`git`
  object types, credentials, ports, queries, fragments, traversal, encoded separators, extra path
  segments, and ambiguous object shapes. Each emits `package-repository-identity-mismatch` before
  install, build, or pack.
- A normalized repository identity mismatch with the observed caller repository, including a
  case-only match that succeeds and a different owner or repository that fails.
- Raw `repository`, `license`, `description`, `keywords`, `author`, and `homepage` values recorded
  only in `diagnostic_metadata.package_manifest`, preserving permitted source JSON forms and never
  entering SLSA `internalParameters` or `externalParameters`.
- Workspace package using root package-manager metadata and root lockfile.
- Corepack exact version enforcement failure.
- Successful npm capture emits only name-keyed `lockfile` and `runner-image` descriptors; successful
  pnpm capture emits its registry-integrity package-manager distribution; successful Yarn capture
  emits its download-hash package-manager distribution.
- Unavailable package-manager distribution capture, unavailable runner-image file or Included
  Software URL, and unavailable required runner observations each emit
  `windlass.verify.error.input-unavailable`, exit `2`, and stop before signing.
- Wrong pnpm or Yarn digest authority, wrong captured manager version, Known Good Release fallback,
  ambient pnpm or Yarn, and a registry override that changes the Corepack acquisition path each emit
  `windlass.verify.error.resolved-dependencies-package-manager-distribution`, exit `1`, and stop
  before signing.
- A reconstructed or guessed runner URL, a runner URL missing from the `Runner Image` detail, and
  runner label, version, Node.js version, or descriptor-shape mismatches each stop before signing
  with their specified runner-image or input-unavailable diagnostic and expected exit code.
