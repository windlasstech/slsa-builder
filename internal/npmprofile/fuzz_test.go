package npmprofile

import (
	"bytes"
	"encoding/json"
	"errors"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

// requireClassifiedDiagnostic asserts that err is non-nil and exposes a
// non-empty diagnostic ID, as every npmprofile validation error must.
func requireClassifiedDiagnostic(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a classified error, got nil")
	}
	var identified interface{ DiagnosticID() string }
	if !errors.As(err, &identified) {
		t.Fatalf("error %T does not expose a diagnostic ID: %v", err, err)
	}
	if identified.DiagnosticID() == "" {
		t.Fatalf("error exposes an empty diagnostic ID: %v", err)
	}
}

// mutateJSONTop applies mutate to the top-level object of encoded and returns
// the re-marshaled result, for building fuzz seed mutations.
func mutateJSONTop(f *testing.F, encoded []byte, mutate func(top map[string]json.RawMessage)) []byte {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &top); err != nil {
		f.Fatalf("decode seed for mutation: %v", err)
	}
	mutate(top)
	mutated, err := json.Marshal(top)
	if err != nil {
		f.Fatalf("re-encode mutated seed: %v", err)
	}
	return mutated
}

// FuzzDecodeExternalParameters feeds attacker-controlled bytes to the closed
// npm external-parameters decoder. Properties asserted for every input:
//   - decoding never panics;
//   - every rejection carries a classified diagnostic ID;
//   - a successful decode re-encodes to strict canonical JSON that re-decodes,
//     and the encode/decode round-trip is byte-stable.
func FuzzDecodeExternalParameters(f *testing.F) {
	valid, err := EncodeExternalParameters(validExternalParameters(ManagerPNPM))
	if err != nil {
		f.Fatalf("encode valid external parameters: %v", err)
	}
	f.Add([]byte(valid))

	// Unknown member injection (provenance_input_test.go "unknown external parameter").
	f.Add(mutateJSONTop(f, []byte(valid), func(top map[string]json.RawMessage) {
		top["injected"] = json.RawMessage("true")
	}))

	// Missing nested member (provenance_input_test.go "missing nested external parameter").
	f.Add(mutateJSONTop(f, []byte(valid), func(top map[string]json.RawMessage) {
		var distribution map[string]json.RawMessage
		if err := json.Unmarshal(top["distribution"], &distribution); err != nil {
			f.Fatalf("decode seed distribution member: %v", err)
		}
		delete(distribution, "linked_artifact_metadata")
		encoded, err := json.Marshal(distribution)
		if err != nil {
			f.Fatalf("re-encode distribution member: %v", err)
		}
		top["distribution"] = encoded
	}))

	// Null conditional source members (provenance_validate.go conditional source member rule).
	f.Add(mutateJSONTop(f, []byte(valid), func(top map[string]json.RawMessage) {
		var source map[string]json.RawMessage
		if err := json.Unmarshal(top["source"], &source); err != nil {
			f.Fatalf("decode seed source member: %v", err)
		}
		for _, name := range []string{"input_ref", "invocation_ref", "invocation_revision"} {
			source[name] = json.RawMessage("null")
		}
		encoded, err := json.Marshal(source)
		if err != nil {
			f.Fatalf("re-encode source member: %v", err)
		}
		top["source"] = encoded
	}))

	f.Fuzz(func(t *testing.T, data []byte) {
		parameters, err := DecodeExternalParameters(data)
		if err != nil {
			requireClassifiedDiagnostic(t, err)
			return
		}
		encoded, err := EncodeExternalParameters(parameters)
		if err != nil {
			t.Fatalf("decoded parameters failed to encode: %v", err)
		}
		if err := canonicaljson.Validate(encoded); err != nil {
			t.Fatalf("encoded parameters are not valid strict JSON: %v", err)
		}
		reparsed, err := DecodeExternalParameters(encoded)
		if err != nil {
			t.Fatalf("encoded parameters failed to re-decode: %v", err)
		}
		reencoded, err := EncodeExternalParameters(reparsed)
		if err != nil {
			t.Fatalf("re-decoded parameters failed to encode: %v", err)
		}
		if !bytes.Equal(reencoded, encoded) {
			t.Fatal("external parameter encode-decode round-trip is not byte-stable")
		}
	})
}

// FuzzNPMPackagePURL asserts that accepted name/version pairs always produce a
// pkg:npm/ Package URL and that rejection is classified npm-subject-mismatch.
func FuzzNPMPackagePURL(f *testing.F) {
	for _, seed := range [][2]string{
		{"@scope/name", "1.2.3"},
		{"name", "0.0.0"},
		// Invalid names from provenance_input_test.go TestScopedPURL.
		{"", "1.0.0"},
		{"@scope", "1.0.0"},
		{"@/name", "1.0.0"},
		{"@scope/name/extra", "1.0.0"},
		{"name/extra", "1.0.0"},
		{"name\n", "1.0.0"},
	} {
		f.Add(seed[0], seed[1])
	}

	f.Fuzz(func(t *testing.T, name, version string) {
		purl, err := NPMPackagePURL(name, version)
		if err != nil {
			requireNPMDiagnostic(t, err, IDNPMSubjectMismatch)
			return
		}
		if !strings.HasPrefix(purl, "pkg:npm/") {
			t.Fatalf("accepted PURL %q lacks the pkg:npm/ prefix", purl)
		}
	})
}

// FuzzValidateReleaseRefEquality asserts that the release-identity equality
// check never panics and that every rejection is classified
// release-ref-mismatch.
func FuzzValidateReleaseRefEquality(f *testing.F) {
	f.Add("refs/tags/v1.2.3", "refs/tags/v1.2.3", "refs/tags/v1.2.3", "v1.2.3", testSourceSHA, testSourceSHA)
	// Mutation rows from provenance_input_test.go TestReleaseRefEquality.
	f.Add("v1.2.3", "refs/tags/v1.2.3", "refs/tags/v1.2.3", "v1.2.3", testSourceSHA, testSourceSHA)
	f.Add("refs/heads/main", "refs/heads/main", "refs/heads/main", "main", testSourceSHA, testSourceSHA)
	f.Add("refs/tags/v1.2.3", "refs/tags/v1.2.3", "refs/tags/v1.2.4", "v1.2.3", testSourceSHA, testSourceSHA)
	f.Add("refs/tags/v1.2.3", "refs/tags/v1.2.3", "refs/tags/v1.2.3", "v1.2.4", testSourceSHA, testSourceSHA)
	f.Add("refs/tags/v1.2.3", "refs/tags/v1.2.3", "refs/tags/v1.2.3", "v1.2.3", testSourceSHA, testAttestSHA)

	f.Fuzz(func(t *testing.T, sourceRef, releaseRef, runtimeRef, versionTag, sourceRevision, peeledRevision string) {
		if err := ValidateReleaseRefEquality(sourceRef, releaseRef, runtimeRef, versionTag, sourceRevision, peeledRevision); err != nil {
			requireNPMDiagnostic(t, err, IDReleaseRefMismatch)
		}
	})
}

// FuzzValidateSourceRefInput asserts that the tags-only source-ref validator
// never panics and that every rejection is classified source-ref-invalid.
func FuzzValidateSourceRefInput(f *testing.F) {
	f.Add("", "refs/tags/v1.2.3", "1.2.3")
	f.Add("refs/tags/v1.2.3", "refs/tags/v1.2.3", "1.2.3")
	f.Add("refs/tags/v1.2.3", "refs/heads/main", "1.2.3")
	// Rejection rows from source_ref_test.go TestValidateSourceRefInput.
	f.Add(" \t\n\r\v\f", "refs/tags/v1.2.3", "1.2.3")
	f.Add(" refs/tags/v1.2.3", "refs/heads/main", "1.2.3")
	f.Add("refs/tags/v1.2.3 ", "refs/heads/main", "1.2.3")
	f.Add("\trefs/tags/v1.2.3\t", "refs/heads/main", "1.2.3")
	f.Add("refs/heads/main", "refs/heads/main", "1.2.3")
	f.Add("v1.2.3", "refs/heads/main", "1.2.3")
	f.Add("0123456789abcdef0123456789abcdef01234567", "refs/heads/main", "1.2.3")
	f.Add("refs/tags/v1.2.4", "refs/heads/main", "1.2.3")
	f.Add("refs/tags/v1.2.4", "refs/tags/v1.2.3", "1.2.4")

	f.Fuzz(func(t *testing.T, sourceRef, invocationRef, packageVersion string) {
		if err := ValidateSourceRefInput(sourceRef, invocationRef, packageVersion); err != nil {
			requireNPMDiagnostic(t, err, IDSourceRefInvalid)
		}
	})
}

// FuzzParseJSONWorkspacePatterns asserts that workspaces parsing never panics
// and that the direct-array and {"packages":[...]} forms stay equivalent.
func FuzzParseJSONWorkspacePatterns(f *testing.F) {
	f.Add([]byte(`["packages/*"]`))
	f.Add([]byte(`{"packages":["packages/*"]}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`["a",1]`))
	f.Add([]byte(`{"packages":"x"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		patterns, err := parseJSONWorkspacePatterns(jsonRaw(data))

		var array []json.RawMessage
		if json.Unmarshal(data, &array) != nil {
			return // Non-array input: no array-form property to assert.
		}
		allStrings := true
		for _, element := range array {
			var text string
			if json.Unmarshal(element, &text) != nil {
				allStrings = false
				break
			}
		}
		if allStrings && err != nil {
			t.Fatalf("array of strings rejected: %v", err)
		}
		if !allStrings && err == nil {
			t.Fatal("array containing a non-string element was accepted")
		}

		// The wrapped {"packages":...} form must accept exactly the same
		// arrays as the direct form and yield the same patterns.
		wrapped := append(append([]byte(`{"packages":`), data...), '}')
		wrappedPatterns, wrappedErr := parseJSONWorkspacePatterns(jsonRaw(wrapped))
		if allStrings {
			if wrappedErr != nil {
				t.Fatalf("wrapped form rejected a valid array: %v", wrappedErr)
			}
			if !slices.Equal(wrappedPatterns, patterns) {
				t.Fatalf("wrapped form = %v, direct form = %v", wrappedPatterns, patterns)
			}
		} else if wrappedErr == nil {
			t.Fatal("wrapped form accepted an array with a non-string element")
		}
	})
}

// FuzzParsePNPMWorkspacePatterns asserts that pnpm-workspace.yaml parsing
// never panics, is deterministic, and honors the root-only contract: rootOnly
// is true exactly when no workspace patterns are returned.
func FuzzParsePNPMWorkspacePatterns(f *testing.F) {
	f.Add([]byte("packages:\n  - 'packages/*'\n"))
	// Settings-only fixture shape (testdata/npm/packages/pnpm-settings-only-valid, ADR 0078).
	f.Add([]byte("minimumReleaseAge: 1440\nminimumReleaseAgeExclude:\n  - '@windlass-fixtures/*'\nonlyBuiltDependencies:\n  - esbuild\ntrustPolicy: no-downgrade\n"))
	f.Add([]byte("packages: x\n"))
	f.Add([]byte("[1,2]\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		patterns, rootOnly, err := parsePNPMWorkspacePatterns(data)
		secondPatterns, secondRootOnly, secondErr := parsePNPMWorkspacePatterns(data)
		if (err == nil) != (secondErr == nil) || rootOnly != secondRootOnly || !slices.Equal(patterns, secondPatterns) {
			t.Fatalf("parsePNPMWorkspacePatterns is nondeterministic for %q", data)
		}
		if err != nil {
			return
		}
		if rootOnly && len(patterns) != 0 {
			t.Fatalf("root-only result carried workspace patterns: %v", patterns)
		}
	})
}

// FuzzValidateWorkspacePattern asserts that accepted workspace patterns are
// relative, backslash-free, and contain no empty, ".", or ".." segments.
func FuzzValidateWorkspacePattern(f *testing.F) {
	for _, seed := range []string{
		"packages/*", "packages/**", "packages/{a,b}", "", "/packages/*",
		`..\outside`, "../outside", "packages/.", "packages/..", "packages/**foo",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, pattern string) {
		first := validateWorkspacePattern(pattern)
		second := validateWorkspacePattern(pattern)
		if (first == nil) != (second == nil) {
			t.Fatalf("validateWorkspacePattern(%q) is nondeterministic: first %v, second %v", pattern, first, second)
		}
		if first != nil {
			return
		}
		if pattern == "" || strings.Contains(pattern, `\`) || strings.HasPrefix(pattern, "/") {
			t.Fatalf("accepted unsafe workspace pattern %q", pattern)
		}
		for _, segment := range strings.Split(pattern, "/") {
			if segment == "" || segment == "." || segment == ".." {
				t.Fatalf("accepted workspace pattern %q with an unsafe segment", pattern)
			}
		}
	})
}

// FuzzParsePackageManager asserts that packageManager descriptor parsing never
// panics, that rejections carry a registered diagnostic ID, and that accepted
// managers satisfy the npm/pnpm/yarn version rules.
func FuzzParsePackageManager(f *testing.F) {
	for _, seed := range []string{
		`"npm@11.5.1"`, `"pnpm@10.14.0"`, `"yarn@4.9.2"`,
		`"yarn@3.6.0"`, `"pnpm@latest"`, `42`, `"pnpm@"`,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		candidate, failureID := parsePackageManager(jsonRaw(data), "package.json")
		if failureID != "" {
			if _, ok := diagnostic.Lookup(failureID); !ok {
				t.Fatalf("rejection carries unregistered diagnostic ID %q", failureID)
			}
			return
		}
		switch candidate.name {
		case ManagerNPM:
		case ManagerPNPM:
			if !exactSemver(candidate.version) {
				t.Fatalf("accepted pnpm with non-exact version %q", candidate.version)
			}
		case ManagerYarn:
			if !exactSemver(candidate.version) {
				t.Fatalf("accepted yarn with non-exact version %q", candidate.version)
			}
			// exactSemver guarantees a numeric major without leading zeros;
			// x/mod/semver accepts a bare major ("v4") and compares
			// arbitrary-length majors, so compare digit strings directly.
			major, _, _ := strings.Cut(candidate.version, ".")
			if len(major) == 1 && major < "4" {
				t.Fatalf("accepted yarn with pre-v4 version %q", candidate.version)
			}
		default:
			t.Fatalf("accepted unknown package manager %q", candidate.name)
		}
	})
}

// FuzzParseDevEngines asserts that devEngines parsing never panics, treats
// empty input as absent, classifies rejections with registered diagnostic IDs,
// and never accepts Yarn (the Yarn devEngines path must reject).
func FuzzParseDevEngines(f *testing.F) {
	for _, seed := range []string{
		`{}`,
		`{"packageManager":{"name":"pnpm","version":"10.14.0"}}`,
		`{"packageManager":{"name":"yarn","version":"4.9.2"}}`,
		`{"packageManager":{"name":"pnpm","version":"10.14.0","onFail":"download"}}`,
		`{"unknown":1}`,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		candidate, present, failureID := parseDevEngines(jsonRaw(data), "package.json")
		if len(data) == 0 {
			if present || failureID != "" {
				t.Fatalf("empty input must be absent, got present=%t failureID=%q", present, failureID)
			}
			return
		}
		if failureID != "" {
			if _, ok := diagnostic.Lookup(failureID); !ok {
				t.Fatalf("rejection carries unregistered diagnostic ID %q", failureID)
			}
			return
		}
		if !present {
			return
		}
		switch candidate.name {
		case ManagerNPM:
		case ManagerPNPM:
			if !exactSemver(candidate.version) || strings.Contains(candidate.version, "+") {
				t.Fatalf("devEngines accepted pnpm with non-exact version %q", candidate.version)
			}
		default:
			t.Fatalf("devEngines accepted package manager %q", candidate.name)
		}
	})
}

// FuzzDecodePackedManifest asserts that packed-manifest decoding never panics,
// that rejections wrap the package-pack-failed diagnostic ID, and that an
// accepted manifest is duplicate-free JSON with string identity members.
func FuzzDecodePackedManifest(f *testing.F) {
	f.Add([]byte(`{"name":"x","version":"1.0.0"}`))
	f.Add([]byte(`{"name":"x"}`))
	f.Add([]byte(`{"name":1,"version":"1.0.0"}`))
	f.Add([]byte(`{"name":"x","name":"y","version":"1.0.0"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		decoded, err := decodePackedManifest(data)
		if err != nil {
			if !strings.Contains(err.Error(), IDPackagePackFailed) {
				t.Fatalf("error does not wrap %s: %v", IDPackagePackFailed, err)
			}
			return
		}
		if err := canonicaljson.Validate(data); err != nil {
			t.Fatalf("accepted manifest is not duplicate-free JSON: %v", err)
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(data, &object); err != nil {
			t.Fatalf("accepted manifest is not an object: %v", err)
		}
		var name, version string
		if json.Unmarshal(object["name"], &name) != nil || json.Unmarshal(object["version"], &version) != nil {
			t.Fatal("accepted manifest lacks string name or version members")
		}
		if name != decoded.Name || version != decoded.Version {
			t.Fatalf("decoded identity (%q, %q) differs from manifest members (%q, %q)", decoded.Name, decoded.Version, name, version)
		}
	})
}

// FuzzValidateArchivePath asserts that accepted archive paths are canonical,
// relative, backslash-free paths under the package/ root.
func FuzzValidateArchivePath(f *testing.F) {
	for _, seed := range []string{
		"package/file", "package/dist/index.js", "", "..", "../outside",
		"package/../outside", "/package/file", `package\file`, "other/file", "package//file",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, name string) {
		first := validateArchivePath(name)
		second := validateArchivePath(name)
		if (first == nil) != (second == nil) {
			t.Fatalf("validateArchivePath(%q) is nondeterministic: first %v, second %v", name, first, second)
		}
		if first != nil {
			return
		}
		if !strings.HasPrefix(name, "package/") || path.Clean(name) != name || strings.Contains(name, `\`) || path.IsAbs(name) {
			t.Fatalf("accepted unsafe archive path %q", name)
		}
	})
}
