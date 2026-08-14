# Go Testing Guide

This guide is the normative reference for all Go test code in this repository. It defines how tests
are organized, which components must carry fuzz targets, how security-negative cases are built, and
which quality gates every change must pass. slsa-builder is a security-critical SLSA provenance
builder: its tests are part of the assurance story, not an afterthought. Write them accordingly.

## Purpose and scope

This guide governs every `*_test.go` file in the module. It applies to new code and to changes that
touch existing code. Where this guide uses "must", the rule is mandatory; where it uses "should",
deviating requires justification in review.

Three principles motivate every rule below:

1. **Tests are assurance evidence.** Downstream consumers trust the provenance this builder signs
   and verifies. A claim made in an ADR or an architecture spec must be backed by a test that would
   fail if the claim stopped being true.
2. **The trust boundary is hostile.** Signed JSON, policies, handoff contracts, registry and OIDC
   responses, and workspace YAML are attacker-controlled bytes. Parsers on that boundary get fuzz
   targets and negative matrices, not just happy-path tests.
3. **Tests stay deterministic and hermetic.** No real network, no wall-clock dependence, no shared
   mutable state. A test that flakes is worse than no test, because it trains reviewers to ignore
   failures.

## Test organization

### Standard library only

Use the `testing` package and the standard library. Do not add testify, go-cmp, gomega, or any other
third-party assertion or matcher library. The dependency surface of this repository is part of its
trusted computing base, and the standard library is sufficient. Explicit comparison with clear
failure messages is preferred over matcher DSLs:

```go
if !bytes.Equal(got, want) {
	t.Fatalf("canonical form mismatch:\n got: %s\nwant: %s", got, want)
}
```

### External test packages

Test files that exercise a package's public contract use the external test package form
(`package foo_test`) and import the package under test, as in
`internal/canonicaljson/canonicaljson_test.go`:

```go
package canonicaljson_test

import (
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)
```

Internal test packages (`package foo`) are acceptable only when the test genuinely needs unexported
identifiers, as with the package-internal white-box tests in `internal/npmprofile`. Prefer the
external form whenever the public API can express the assertion.

### Table-driven tests and subtests

Group related cases into tables and run each case as a named subtest with `t.Run`. This is the idiom
documented in <https://go.dev/wiki/TableDrivenTests> and it is how this repository is written. From
`internal/canonicaljson/canonicaljson_test.go`:

```go
tests := []struct {
	name   string
	input  string
	member string
}{
	{name: "top level", input: `{"predicate":{},"predicate":{}}`, member: "predicate"},
	{name: "nested object", input: `{"predicate":{"buildType":"first","buildType":"second"}}`, member: "buildType"},
}

for _, test := range tests {
	t.Run(test.name, func(t *testing.T) {
		t.Parallel()

		err := canonicaljson.Validate([]byte(test.input))
		requireDuplicateMemberError(t, err, test.member)
	})
}
```

Table entry names must be descriptive enough that a failing `go test -run` output identifies the
case without opening the file.

### Parallel tests

Call `t.Parallel()` in every test and subtest unless the test mutates process-global state
(environment variables, the working directory, shared files). When a loop variable is captured by a
parallel subtest on a Go version where capture semantics require it, copy it first
(`packageDirectory := packageDirectory`, as in `internal/npmprofile/selection_test.go`). Parallel
execution is what makes `go test -race` meaningful, so skipping `t.Parallel()` silently reduces race
coverage.

### Test helpers and `t.Helper()`

Shared assertion helpers must call `t.Helper()` so failure output points at the failing call site,
not inside the helper. From `internal/canonicaljson/canonicaljson_test.go`:

```go
func requireDuplicateMemberError(t *testing.T, err error, member string) {
	t.Helper()

	var duplicate *canonicaljson.DuplicateMemberError
	if !errors.As(err, &duplicate) {
		t.Fatalf("error %v is not a DuplicateMemberError", err)
	}
	if duplicate.Member != member {
		t.Fatalf("duplicate member = %q, want %q", duplicate.Member, member)
	}
}
```

Name helpers that assert failure conditions with a `require` or `assert` prefix, matching the
existing `requireDuplicateMemberError`, `assertPass`, `assertRejected`, and `assertReportPrimary`
helpers. See <https://pkg.go.dev/testing#T.Helper>.

### Fixtures and golden files

Static inputs and expected outputs live under `testdata/` (package-local) or the repository-root
`testdata/` tree for cross-package vectors. `TestJCSVectors` in `internal/canonicaljson` reads
paired input/output files from `testdata/canonicaljson/` and compares bytes; `internal/handoff`
loads JSON fixtures from `testdata/handoff/` via a `loadFixture` helper. Never embed large blobs as
string literals when a fixture file reads better, and never write to `testdata/` from a test:
fixtures are inputs, not outputs.

### HTTP tests with `net/http/httptest`

Tests for HTTP clients must use `httptest.NewServer` or `httptest.NewTLSServer` and the server's own
client; real network access is forbidden in unit tests. Assert on the request the client sent, not
just the response it parsed. From `internal/npmprofile/registry_client_test.go`:

```go
server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet || request.URL.EscapedPath() != "/%40windlass%2Fslsa-builder" {
		t.Errorf("request = %s %s", request.Method, request.URL.EscapedPath())
	}
	writer.Header().Set("Content-Type", "application/json")
	fmt.Fprint(writer, `{"name":"@windlass/slsa-builder", ... }`)
}))
defer server.Close()
```

Note that `t.Errorf` inside the handler reports against the test's `*testing.T`; the server
constructor helper (`newOIDCExchangeServer` in `internal/npmprofile/oidc_client_test.go`) accepts
`t` and calls `t.Helper()` for exactly this reason.

### Gated live integration tests

Tests must stay hermetic by default: no real network, no real package-manager toolchain, no mutable
external state. When a test genuinely exercises the real toolchain or network surface—for example,
invoking Corepack to acquire pnpm or Yarn and running `npm pack` against a real package—name it with
a `Live` suffix and gate it twice:

- Skip under `go test -short` with `testing.Short()`.
- Skip unless the operator explicitly opts in with `SLSA_BUILDER_LIVE_TOOLCHAIN=1`.

Use a helper such as `requireLiveToolchain(t)` that calls `t.Helper()` and then
`t.Skip("live toolchain test: set SLSA_BUILDER_LIVE_TOOLCHAIN=1 to run")` when the variable is
absent. These tests never run in CI; they are reserved for manual dogfood verification.

## Fuzzing

Fuzzing is the centerpiece of testing on this repository's trust boundary. Go 1.18 made Go the first
major language with fuzzing fully integrated into its standard toolchain
(<https://go.dev/blog/go1.18#fuzzing>), and the canonical reference for the facility is
<https://go.dev/doc/security/fuzz/>. This section defines where fuzz targets are required, how to
write them, and how they run locally and in CI.

### Why fuzzing matters here

Every parser, decoder, and validator on the trust boundary consumes bytes chosen by an attacker: a
signed in-toto Statement from an untrusted producer, a verification policy, a handoff contract, an
npm registry or OIDC response, a workspace `pnpm-workspace.yaml`. Table-driven tests cover the
malformed inputs an engineer can imagine; coverage-guided fuzzing covers the ones nobody imagined,
including the class of parser-differential bugs (escaped duplicate members, surrogate pairs, number
spellings) that signed-JSON verification must reject deterministically.

Two normative constraints from <https://go.dev/doc/security/fuzz/> shape every fuzz target in this
repository:

- Fuzz functions "should be fast and deterministic".
- Fuzz tests "should not persist past the end of each call" and "should not depend on global state".

In practice this means: no network, no filesystem outside `t.TempDir()`, no time-dependent branches,
no package-level mutable caches inside the fuzz function. Every fuzzed call must be a pure function
of its inputs.

### Which code must have fuzz targets

Policy: **every parser, decoder, or validator that consumes external or untrusted input must have a
fuzz target.** Adding a new trust-boundary parser without a fuzz target is an incomplete change.

The concrete candidate list for this repository, derived from the trust boundary:

- `internal/attestation`: bundle parsing (`ParseBundle`) and signature verification input handling.
- `internal/policy/decode.go`: verification policy decoding.
- `internal/handoff`: handoff contract decoding and `ValidateSafeBasename`.
- `internal/npmprofile/provenance_validate.go` and `internal/npmprofile/provenance_input.go`:
  provenance validation and input normalization.
- `internal/workflowcheck` and `internal/npmprofile/workspace.go`: YAML parsing and workspace member
  resolution.
- `internal/npmprofile/registry_client.go` and `internal/npmprofile/oidc_client.go`: registry
  metadata, attestation, and OIDC exchange response decoding.
- Identity and digest validators (`builder.id`, `buildType` URIs, digest strings).

Current state: `internal/canonicaljson` carries the repository's one fuzz target, `FuzzStrictJSON`.
Treat it as the exemplar to copy when adding targets to the packages above.

### Anatomy of a fuzz target

A fuzz target is a function named `FuzzXxx` taking exactly one `*testing.F` parameter, which calls
`f.Fuzz` exactly once with a function whose first parameter is `*testing.T`
(<https://pkg.go.dev/testing#hdr-Fuzzing>, <https://pkg.go.dev/testing#F.Fuzz>). Arguments after
`*testing.T` must use the supported fuzzing types: `string`, `[]byte`, `rune`, `byte`, the signed
and unsigned integer types, `float32`, `float64`, and `bool`. Optional seed values registered with
`f.Add` must match the fuzz function's argument types exactly and in order
(<https://pkg.go.dev/testing#F.Add>).

The repository exemplar, `FuzzStrictJSON` in `internal/canonicaljson/canonicaljson_test.go`
(trimmed):

```go
func FuzzStrictJSON(f *testing.F) {
	f.Fuzz(func(t *testing.T, input []byte) {
		if err := canonicaljson.Validate(input); err == nil {
			canonical, err := canonicaljson.Canonicalize(input)
			if err != nil {
				t.Fatalf("valid input failed canonicalization: %v", err)
			}
			if err := canonicaljson.Validate(canonical); err != nil {
				t.Fatalf("canonical output is not strict JSON: %v", err)
			}
			equal, err := canonicaljson.Equal(input, canonical)
			if err != nil {
				t.Fatalf("compare input with canonical output: %v", err)
			}
			if !equal {
				t.Fatal("canonicalization changed the parsed JSON value")
			}
		}

		encodedKey, err := json.Marshal(string(input))
		// ...build a {key:0,key:1} duplicate-member object from the fuzzed bytes...
		requireDuplicateMemberError(t, canonicaljson.Validate(duplicate), normalizedKey)
	})
}
```

Note what the target does not do: it does not recover from panics (a panic in the code under test is
exactly what fuzzing exists to find), and it does not return early on error; it converts every
contract violation into `t.Fatal`.

Corpus files generated or added for a target live under `testdata/fuzz/<FuzzName>/` next to the test
file, in the corpus file format defined at <https://go.dev/doc/security/fuzz/#corpus-file-format>.
Each file begins with the header line `go test fuzz v1` followed by the argument encoding, for
example `internal/canonicaljson/testdata/fuzz/FuzzStrictJSON/duplicate-member`:

```text
go test fuzz v1
[]byte("{\"predicate\":{},\"predicate\":{}}")
```

### Property-based fuzzing, not crash-only fuzzing

A fuzz target that only checks "did it panic" wastes the engine. Assert invariants instead
(<https://go.dev/doc/tutorial/fuzz> demonstrates the round-trip property for string reversal).
Useful invariant shapes for this repository:

- **Round-trip:** decode(encode(x)) == x, or encode(decode(bytes)) is stable.
- **Idempotence:** canonicalize(canonicalize(x)) == canonicalize(x). `TestJCSVectors` asserts this
  for table inputs; a fuzz target asserts it for all generated inputs.
- **Semantic preservation:** canonicalization must not change the parsed value, which is what
  `FuzzStrictJSON` checks with the `Validate` → `Canonicalize` → `Validate` → `Equal` chain.
- **Valid input never panics and always succeeds:** if `Validate(input)` returns nil, every
  downstream consumer of that input must succeed.
- **Invalid input fails in a classified, bounded way:** rejection must produce the registered
  diagnostic error (for example `DuplicateMemberError` with its diagnostic ID), not a panic, an
  unbounded hang, or a generic parse error. `FuzzStrictJSON` checks exactly this by constructing a
  duplicate-member object from fuzzed bytes and requiring the classified error.

If the fuzz function receives input outside the domain being tested, skip it with `t.Skip` rather
than asserting on it, as the tutorial shows for non-UTF-8 inputs
(<https://go.dev/doc/tutorial/fuzz>).

Avoid tautological properties. `Canonicalize(x) == Canonicalize(x)` proves nothing, and neither does
re-implementing the parser inside the test and comparing the two. Properties must relate independent
operations (parse versus print, validate versus canonicalize, accept versus classify) or assert an
external contract such as a diagnostic ID.

### Seed corpus discipline

Two seed sources, both cheap:

1. **Convert existing table cases into `f.Add` seeds.** Every malicious or edge-case input in a
   table-driven test is a seed. The `TestEscapedDuplicateMembers` cases (`{"a":1,"\u0061":2}`,
   surrogate pairs, control escapes) are exactly the kind of starting point that steers the engine
   toward parser differentials. Add them to the fuzz target with `f.Add`:

   ```go
   func FuzzStrictJSON(f *testing.F) {
    	f.Add([]byte(`{"a":1,"a":2}`))
    	f.Add([]byte(`{"a":1,"\u0061":2}`))
    	f.Add([]byte(`{"😂":1,"\ud83d\ude02":2}`))
    	f.Add([]byte(`{"predicate":{"builder":{"id":"trusted"}}}`))
   	f.Fuzz(func(t *testing.T, input []byte) {
   		// ...
   	})
   }
   ```

2. **Commit discovered crashers.** When a fuzz run finds a failing input, the engine writes it to
   the build cache. Move that file into `testdata/fuzz/<FuzzName>/` with a descriptive name and
   commit it alongside the fix. Committed corpus entries run as ordinary regression tests under
   plain `go test`, so the crasher stays fixed forever. The four committed entries under
   `internal/canonicaljson/testdata/fuzz/FuzzStrictJSON/` (`duplicate-member`,
   `escaped-duplicate-member`, `trailing-value`, `valid-nested`) are the model.

### Running fuzzing locally

Fuzz targets run as seed-corpus regression tests under `go test ./...` with no extra flags. To run
the engine itself, pick one target per `go test` invocation
(<https://pkg.go.dev/cmd/go#hdr-Testing_flags>):

```bash
# Fuzz one target for 30 seconds.
go test -fuzz=FuzzStrictJSON -fuzztime=30s ./internal/canonicaljson/

# Keep minimizing a found crasher for 10 seconds after discovery.
go test -fuzz=FuzzStrictJSON -fuzztime=30s -fuzzminimizetime=10s ./internal/canonicaljson/

# Use more parallel workers.
go test -fuzz=FuzzStrictJSON -fuzztime=60s -parallel=8 ./internal/canonicaljson/

# Replay a single failing input as a unit test.
go test -run=FuzzStrictJSON/<hash> ./internal/canonicaljson/

# Clear the build-cache corpus when it grows stale.
go clean -fuzzcache
```

Coverage-guided fuzzing is supported on AMD64 and ARM64; on other architectures the targets still
run as seed-corpus tests but the engine is unavailable (<https://go.dev/doc/security/fuzz/>).

### Fuzzing in CI

Recommended policy for this repository:

- **On pull requests:** a bounded fuzz smoke run of 30 to 60 seconds per target, executed after the
  normal `go test ./...` job. This catches shallow regressions in the code a PR actually touches
  without blocking review.
- **On a schedule:** longer runs (tens of minutes per target) on a nightly or weekly workflow, with
  the build-cache corpus uploaded as a workflow artifact so newly found crashers can be retrieved
  and committed.
- **Crash handling:** any crasher found in CI becomes a committed corpus file plus a regression fix
  in a follow-up change before the fuzz job is allowed to pass again.

This mirrors the OSS-Fuzz continuous-integration guidance
(<https://google.github.io/oss-fuzz/getting-started/continuous-integration/>): short bounded runs on
each PR, artifacts preserved for crashers, with CIFuzz itself defaulting to 600 seconds per run. For
a pure-Go module, native `go test -fuzz` in a scheduled workflow is simpler than adopting
ClusterFuzzLite (<https://google.github.io/clusterfuzzlite/>, listed only as an alternative).
Upstream OSS-Fuzz integration for Go projects
(<https://google.github.io/oss-fuzz/getting-started/new-project-guide/go-lang/>) remains a future
option once the target set stabilizes.

## Security-focused negative testing

Fuzzing explores; negative tests pin. Every trust-boundary component must carry explicit, named
subtests for its abuse cases, because a fuzzer may never rediscover a specific regression and
reviewers must be able to read the threat model as a test list.

Patterns already established in this repository, and required for new code:

- **Malformed input matrices.** Enumerate contract violations as a table and assert the classified
  diagnostic for each. `TestOIDCExchangeResponseContractRejections` in
  `internal/npmprofile/oidc_client_test.go` covers unknown members, missing members, wrong
  `token_type`, inverted lifetimes, fractional epochs, and mixed timestamp representations, each
  asserted to fail with `IDNPMOIDCExchangeIndeterminate` before any registry mutation.
- **Parser-differential cases.** Signed JSON must be read by exactly one parser semantics. Cover
  escaped duplicate members (`{"a":1,"\u0061":2}`), escaped control characters, and surrogate pairs
  (`{"😂":1,"\ud83d\ude02":2}`), as `TestEscapedDuplicateMembers` does. Any new signed-JSON consumer
  must reject the same differentials.
- **Path traversal and symlink escapes.** `internal/npmprofile/selection_test.go` rejects
  `../outside`, absolute escapes, Windows-style backslash paths, and symlinks whose targets leave
  the repository root (both directory symlinks and single-file symlinks on the manifest and
  lockfile). `internal/handoff` drives `ValidateSafeBasename` against a committed
  `testdata/handoff/traversal.json` fixture of hostile names. Prefer committed traversal fixtures
  over inline lists when the matrix is shared or long.
- **Credential-in-URL and redirect rejection.** `TestRegistryClientRejectsInsecureURL` rejects
  `http://`, userinfo credentials (`https://token@...`), query tokens, and fragments in the registry
  URL; `TestRegistryClientRejectsRedirect` stands up two TLS servers and asserts the client refuses
  to follow a 302. `internal/npmprofile/selection_test.go` similarly rejects a repository URL
  carrying a credential. Any new HTTP client must repeat both patterns.
- **Response size limits.** `TestOIDCResponseSizeLimit` serves `maxOIDCResponse + 1` bytes and
  asserts bounded rejection. Every `io.LimitReader` bound in production code must have a test that
  steps exactly one byte past the limit.

The rule of thumb: for every validation a function performs, there is a named subtest that violates
exactly that validation and asserts the exact diagnostic ID or sentinel error, not merely "an error
occurred".

## Quality gates

Run all of these before requesting review; CI enforces them:

```bash
# Full suite, including fuzz seed corpora as regression tests.
go test ./...

# Race detector over the full suite.
go test -race ./...

# Static analysis.
go vet ./...

# Repository lint policy (.golangci.yml).
golangci-lint run
```

The race detector "only finds races that happen at runtime"
(<https://go.dev/doc/articles/race_detector>), which is why the `t.Parallel()` and `httptest`-driven
concurrency in this suite is load-bearing: paths that are never exercised concurrently in tests are
paths the detector cannot check. When you add a concurrent code path, add a test that drives it
concurrently.

Coverage is a gap-finding aid, not a goal:

```bash
go test -coverprofile=cover.out ./...
go tool cover -func=cover.out
```

Use the function-level report to find untested branches on the trust boundary, then decide case by
case whether a test (or a fuzz target) is warranted. Do not chase a percentage, and do not write
assertion-free tests to raise one. See <https://go.dev/doc/build-cover> for the coverage tooling
model.

## Test style rules

Naming, messages, and determinism follow the Google Go Style Guide
(<https://google.github.io/styleguide/go/guide> and
<https://google.github.io/styleguide/go/decisions>), the table-driven idiom
(<https://go.dev/wiki/TableDrivenTests>), and the classic testing techniques talk
(<https://go.dev/talks/2014/testing.slide>). Concretely, for this repository:

- **Names describe behavior.** `TestOIDCExchangeResponseContractRejections`, not `TestExchange2`.
  Subtest names read as the case: `"symlink escape"`, `"fractional epoch"`.
- **Failure messages are actionable.** State what was got, what was wanted, and the identity of the
  case: `t.Fatalf("duplicate member = %q, want %q", duplicate.Member, member)`. A message like
  `t.Fatal("failed")` forces the next reader to rerun the test under a debugger; do not write them.
- **No shared mutable state between tests.** Parallel tests share nothing; per-test state comes from
  `t.TempDir()`, fresh `httptest` servers, and local variables.
- **No wall-clock or calendar dependence.** Fix times as constants
  (`var testExchangeNow = time.Date(2026, 8, 7, 12, 0, 1, 0, time.UTC)` in
  `internal/npmprofile/oidc_client_test.go`) instead of calling `time.Now()`.
- **No network, no sleeps, no ordering assumptions.** A test that needs `time.Sleep` to pass is
  testing the scheduler, not the code; synchronize on channels or poll with a bounded deadline
  instead.
- **One concept per test function.** A test that asserts ten unrelated contracts fails opaquely;
  split it into subtests or separate functions, as the OIDC exchange suite does.

## References

- <https://go.dev/doc/security/fuzz/>: canonical Go fuzzing documentation; corpus file format at
  <https://go.dev/doc/security/fuzz/#corpus-file-format>.
- <https://go.dev/doc/tutorial/fuzz>: tutorial introducing property-based fuzz tests and `t.Skip`
  for out-of-domain inputs.
- <https://pkg.go.dev/testing>: testing package API; fuzzing overview at
  <https://pkg.go.dev/testing#hdr-Fuzzing>, `F.Add` at <https://pkg.go.dev/testing#F.Add>, `F.Fuzz`
  at <https://pkg.go.dev/testing#F.Fuzz>, `T.Helper` at <https://pkg.go.dev/testing#T.Helper>.
- <https://pkg.go.dev/cmd/go#hdr-Testing_flags>: `go test` flag reference, including `-fuzz`,
  `-fuzztime`, and `-fuzzminimizetime`.
- <https://go.dev/blog/go1.18#fuzzing>: Go 1.18 release notes introducing integrated fuzzing.
- <https://go.dev/blog/fuzz-beta>: original fuzzing beta announcement with engine background.
- <https://google.github.io/styleguide/go/guide> and
  <https://google.github.io/styleguide/go/decisions>: Google Go Style Guide, the basis for this
  repository's test style rules.
- <https://go.dev/wiki/TableDrivenTests>: table-driven test idiom reference.
- <https://go.dev/talks/2014/testing.slide>: "Testing Techniques" talk on helpers, fixtures, and
  hermetic tests.
- <https://go.dev/doc/articles/race_detector>: race detector documentation and its runtime-only
  detection model.
- <https://go.dev/doc/build-cover>: coverage tooling documentation.
- <https://google.github.io/oss-fuzz/getting-started/new-project-guide/go-lang/>: OSS-Fuzz
  integration guide for Go projects (future option).
- <https://google.github.io/oss-fuzz/getting-started/continuous-integration/>: OSS-Fuzz CIFuzz
  guidance on bounded PR fuzzing and crasher artifacts.
- <https://google.github.io/clusterfuzzlite/>: ClusterFuzzLite, noted only as an alternative to
  native `go test -fuzz` CI jobs.
