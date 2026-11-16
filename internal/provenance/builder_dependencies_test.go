package provenance_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const signerDescriptorJSON = `{"name":"signing-adapter","uri":"pkg:golang/github.com/sigstore/sigstore-go@v1.4.0","digest":{"dirHash1":"0867457592e1612738db260dd3e94beb25a1eefd33dd2d22db5bf9d25bd260ac"}}`
const signerMismatchID = "windlass.verify.error.builder-dependencies-signing-adapter-mismatch"
const descriptorInvalidID = "windlass.verify.error.builder-dependencies-descriptor-invalid"
const extraDescriptorJSON = `{"name":"release-bot","uri":"pkg:generic/release-bot@1.0.0","digest":{"sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}}`

func TestBuilderDependencyShape(t *testing.T) {
	t.Parallel()
	for _, member := range []string{"annotations", "mediaType", "content", "downloadLocation", "unknown"} {
		for _, value := range []string{"null", `""`, "[]", "{}"} {
			t.Run(member+" unset "+value, func(t *testing.T) {
				t.Parallel()
				// Given an undeclared member equivalent to unset.
				raw := signerDescriptorJSON[:len(signerDescriptorJSON)-1] + `,"` + member + `":` + value + `}`
				// When the full Statement boundary and common validator consume it.
				statement, err := decodeDependencyStatement(t, "["+raw+"]")
				if err == nil {
					_, err = statement.Validate(expectations(nil), nil)
				}
				// Then consumers accept it, while serialization emits only the declared shape.
				if err != nil {
					t.Fatalf("unset %s=%s rejected: %v", member, value, err)
				}
				requireSignerShape(t, statement.Predicate.RunDetails.Builder.BuilderDependencies)
			})
		}
	}
	t.Run("exact signer", func(t *testing.T) {
		t.Parallel()
		statement, err := decodeDependencyStatement(t, "["+signerDescriptorJSON+"]")
		if err != nil {
			t.Fatal(err)
		}
		requireSignerShape(t, statement.Predicate.RunDetails.Builder.BuilderDependencies)
	})
}

func requireSignerShape(t *testing.T, dependencies []provenance.BuilderDependency) {
	t.Helper()
	got, err := json.Marshal(dependencies)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := canonicaljson.Equal(got, []byte("["+signerDescriptorJSON+"]"))
	if err != nil || !equal {
		t.Fatalf("signer descriptor = %s, want [%s] (comparison error %v)", got, signerDescriptorJSON, err)
	}
}

func decodeDependencyStatement(t *testing.T, dependencies string) (provenance.Statement, error) {
	t.Helper()
	data := readFixture(t, "valid-statement.json")
	data = replaceDependencyJSON(t, data, []string{"predicate", "runDetails", "builder"}, json.RawMessage(dependencies))
	decoded, err := provenance.DecodeStatement(data)
	if err == nil {
		useGitHubInternalParameters(&decoded)
	}
	return decoded, err
}

func revisedBuilderJSON(t *testing.T, data []byte, path []string) []byte {
	t.Helper()
	return replaceDependencyJSON(t, data, path, json.RawMessage("["+signerDescriptorJSON+"]"))
}

// This edits test input bytes, not production values or historical fixture files.
func replaceDependencyJSON(t *testing.T, data []byte, path []string, dependencies json.RawMessage) []byte {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	if len(path) == 0 {
		object["builderDependencies"] = dependencies
	} else {
		object[path[0]] = replaceDependencyJSON(t, object[path[0]], path[1:], dependencies)
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := canonicaljson.Canonicalize(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

func testBuilderDependencyRejections(t *testing.T) {
	t.Parallel()
	for _, test := range builderDependencyRejections() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given one malformed descriptor set with otherwise valid common fields.
			statement, err := decodeDependencyStatement(t, test.raw)
			// When decoding and semantic verification run in that order.
			if err == nil {
				_, err = statement.Validate(expectations(nil), nil)
			}
			// Then rejection retains the owning diagnostic, not a generic JSON type error.
			requireDiagnosticID(t, err, test.id)
		})
	}
}

type dependencyRejection struct{ name, raw, id string }

func builderDependencyRejections() []dependencyRejection {
	signer := signerDescriptorJSON
	old := `{"uri":"pkg:golang/github.com/sigstore/sigstore-go@v1.3.0","digest":{"h1":"hnIMHREyCNTYFtOE1o7ae3Axa9B5W5EjUSBJICP2NBE="},"annotations":{"role":"signing-adapter"}}`
	tests := []dependencyRejection{
		{"missing signer", `[]`, signerMismatchID},
		{"null array", `null`, signerMismatchID},
		{"legacy signer", "[" + old + "]", signerMismatchID},
		{"duplicate signer", "[" + signer + "," + signer + "]", signerMismatchID},
		{"unknown extra", "[" + signer + "," + extraDescriptorJSON + "]", descriptorInvalidID},
		{"unknown extra first", "[" + extraDescriptorJSON + "," + signer + "]", descriptorInvalidID},
		{"unknown duplicates", "[" + signer + "," + extraDescriptorJSON + "," + extraDescriptorJSON + "]", descriptorInvalidID},
		{"nameless extra", "[" + signer + `,{"uri":"pkg:generic/extra@1","digest":{"sha256":"` + sha256Hex + `"}}]`, descriptorInvalidID},
		{"empty extra", "[" + signer + ",{}]", descriptorInvalidID},
		{"null extra", "[" + signer + ",null]", descriptorInvalidID},
		{"signer failure precedes unknown", "[" + extraDescriptorJSON + "," + old + "]", signerMismatchID},
		{"signer failure precedes unknown reversed", "[" + old + "," + extraDescriptorJSON + "]", signerMismatchID},
		{"missing signer with unknown", "[" + extraDescriptorJSON + "]", signerMismatchID},
	}
	for _, member := range []string{"name", "uri", "digest"} {
		for _, value := range []string{"", "null", `""`, "{}", "[]", "false", "0", `" "`} {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(signer), &fields); err != nil {
				panic(err) // Constant test input, never fuzzed.
			}
			if value == "" {
				delete(fields, member)
			} else {
				fields[member] = json.RawMessage(value)
			}
			encoded, err := json.Marshal(fields)
			if err != nil {
				panic(err)
			}
			tests = append(tests, dependencyRejection{member + " required " + value, "[" + string(encoded) + "]", signerMismatchID})
		}
	}
	for _, member := range []string{"annotations", "mediaType", "content", "downloadLocation", "unknown"} {
		for _, value := range []string{"false", "0", `" "`, `{"role":"signing-adapter"}`, `[1]`} {
			raw := signer[:len(signer)-1] + `,"` + member + `":` + value + `}`
			tests = append(tests, dependencyRejection{member + " present " + value, "[" + raw + "]", signerMismatchID})
		}
	}
	for _, replacement := range []struct{ name, from, to string }{
		{"wrong name", "signing-adapter", "wrong-name"},
		{"uppercase name", "signing-adapter", "Signing-Adapter"},
		{"old version", "v1.4.0", "v1.3.0"},
		{"old algorithm", "dirHash1", "h1"},
		{"empty digest value", "0867457592e1612738db260dd3e94beb25a1eefd33dd2d22db5bf9d25bd260ac", ""},
		{"malformed digest value", "0867457592e1612738db260dd3e94beb25a1eefd33dd2d22db5bf9d25bd260ac", "zz"},
		{"base64 digest", "0867457592e1612738db260dd3e94beb25a1eefd33dd2d22db5bf9d25bd260ac", "CGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw="},
		{"uppercase digest", "0867457592e1612738db260dd3e94beb25a1eefd33dd2d22db5bf9d25bd260ac", "0867457592E1612738DB260DD3E94BEB25A1EEFD33DD2D22DB5BF9D25BD260AC"},
	} {
		tests = append(tests, dependencyRejection{replacement.name, "[" + strings.ReplaceAll(signer, replacement.from, replacement.to) + "]", signerMismatchID})
	}
	return tests
}
