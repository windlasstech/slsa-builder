package npmprofile

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublishCLIBuilderDependencies(t *testing.T) {
	t.Parallel()
	t.Run("annotations and order", testPublishCLIAnnotations)
	t.Run("rejections", testPublishCLIRejections)
	t.Run("precedence", testPublishCLIPrecedence)
}

func testPublishCLIAnnotations(t *testing.T) {
	t.Helper()
	t.Parallel()
	for _, annotation := range []string{"absent", "null", "false", "0", `"metadata"`, "[]", "{}", `{"digest_authority":"changed"}`, `{"digest_authority":"builder-pinned-record","extra":{"nested":[true,42]}}`} {
		for _, reversed := range []bool{false, true} {
			name := annotation + "/signer first"
			if reversed {
				name = annotation + "/publish first"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				// Given arbitrary descriptive annotations on an otherwise pinned descriptor.
				input := validProvenanceInput(t, ManagerPNPM)
				publish := publishCLIWithMember(t, "annotations", annotation)
				descriptors := "[" + publishCLIWantSigner + "," + publish + "]"
				if reversed {
					descriptors = "[" + publish + "," + publishCLIWantSigner + "]"
				}
				// When decoding and verifying the received bytes.
				err := validatePublishCLIBytes(t, input, publishCLIStatementBytes(t, input, descriptors))
				// Then descriptive annotations and array order do not affect acceptance.
				if err != nil {
					t.Fatalf("valid publish-cli rejected: %v", err)
				}
			})
		}
	}
}

func testPublishCLIRejections(t *testing.T) {
	t.Helper()
	t.Parallel()
	tests := []struct{ name, member, value string }{
		{"misnamed", "name", `"npm-cli"`},
		{"wrong version", "uri", `"pkg:npm/npm@11.21.0"`},
		{"wrong digest", "digest", `{"sha512":"` + strings.Repeat("0", 128) + `"}`},
		{"uppercase digest", "digest", `{"sha512":"` + strings.Repeat("A", 128) + `"}`},
		{"short digest", "digest", `{"sha512":"abcd"}`},
		{"wrong algorithm", "digest", `{"sha256":"` + testSHA256 + `"}`},
		{"extra algorithm", "digest", `{"sha512":"` + testSHA512 + `","sha256":"` + testSHA256 + `"}`},
		{"wrong media type", "mediaType", `"application/x-tar"`},
		{"extra false", "unexpected", "false"},
		{"extra zero", "unexpected", "0"},
		{"extra whitespace", "unexpected", `" "`},
		{"extra content", "content", `"e30="`},
		{"extra download location", "downloadLocation", `"https://example.test/npm.tgz"`},
	}
	for _, member := range []string{"name", "uri", "digest", "mediaType"} {
		for _, value := range []string{"absent", "null", `""`, "{}", "false", "[]"} {
			tests = append(tests, struct{ name, member, value string }{member + "/" + value, member, value})
		}
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given a single authenticated-field violation, with a forged trust annotation.
			input := validProvenanceInput(t, ManagerPNPM)
			publish := publishCLIWithMember(t, test.member, test.value)
			publish = strings.Replace(publish, `"builder-pinned-record"`, `"builder-pinned-record","verified":"true","version":"11.21.1"`, 1)
			raw := publishCLIStatementBytes(t, input, "["+publishCLIWantSigner+","+publish+"]")
			// When consuming the received statement, then the pin failure wins.
			requireNPMDiagnostic(t, validatePublishCLIBytes(t, input, raw), publishCLIMismatch)
		})
	}
	for _, test := range []struct{ name, descriptors, want string }{
		{"missing", "[" + publishCLIWantSigner + "]", publishCLIMismatch},
		{"duplicate", "[" + publishCLIWantSigner + "," + publishCLIWantDescriptor + "," + publishCLIWantDescriptor + "]", publishCLIMismatch},
		{"duplicate annotation key", "[" + publishCLIWantSigner + "," + strings.Replace(publishCLIWantDescriptor, `"builder-pinned-record"`, `"builder-pinned-record","digest_authority":"forged"`, 1) + "]", "windlass.verify.error.duplicate-json-member"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given a cardinality or strict JSON violation.
			input := validProvenanceInput(t, ManagerPNPM)
			// When consuming raw bytes, then reject with the owning diagnostic.
			requireNPMDiagnostic(t, validatePublishCLIBytes(t, input, publishCLIStatementBytes(t, input, test.descriptors)), test.want)
		})
	}
}

func testPublishCLIPrecedence(t *testing.T) {
	t.Helper()
	t.Parallel()
	extra := `{"name":"unknown-tool","uri":"pkg:npm/unknown@1.0.0","digest":{"sha512":"` + testSHA512 + `"}}`
	badSigner := strings.Replace(publishCLIWantSigner, "v1.4.0", "v0.0.0", 1)
	badPublish := strings.Replace(publishCLIWantDescriptor, "11.21.1", "11.21.0", 1)
	for _, test := range []struct{ name, descriptors, want string }{
		{"publish before signer and common", "[" + extra + "," + badSigner + "," + badPublish + "]", publishCLIMismatch},
		{"missing publish before unknown extra", "[" + extra + "," + publishCLIWantSigner + "]", publishCLIMismatch},
		{"signer before common", "[" + extra + "," + badSigner + "," + publishCLIWantDescriptor + "]", IDBuilderDependenciesMismatch},
		{"missing signer before common", "[" + extra + "," + publishCLIWantDescriptor + "]", IDBuilderDependenciesMismatch},
		{"unknown extra after required descriptors", "[" + extra + "," + publishCLIWantSigner + "," + publishCLIWantDescriptor + "]", commonBuilderMismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given simultaneous descriptor failures, including an extra first in the array.
			input := validProvenanceInput(t, ManagerPNPM)
			// When consuming the array, then priority is semantic rather than array order.
			requireNPMDiagnostic(t, validatePublishCLIBytes(t, input, publishCLIStatementBytes(t, input, test.descriptors)), test.want)
		})
	}
}

func publishCLIWithMember(t *testing.T, member, value string) string {
	t.Helper()
	var descriptor map[string]json.RawMessage
	if err := json.Unmarshal([]byte(publishCLIWantDescriptor), &descriptor); err != nil {
		t.Fatal(err)
	}
	if value == "absent" {
		delete(descriptor, member)
	} else {
		descriptor[member] = json.RawMessage(value)
	}
	return string(mustJSON(descriptor))
}
