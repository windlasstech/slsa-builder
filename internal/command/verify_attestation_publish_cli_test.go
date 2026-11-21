package command

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestVerifyAttestationRejectsPublishCLI(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, publish string }{
		{"missing", ""},
		{"wrong version", `,{"name":"publish-cli","uri":"pkg:npm/npm@11.21.0","digest":{"sha512":"613ccbe90044257c87713bb426251ced128ca7e9b3949b65eeb18b98445a5d441e14720c73d6ad76be86f4cf42f2f0d66efe347a9a13400ccd2f6fcfb5954385"},"mediaType":"application/gzip","annotations":{"digest_authority":"builder-pinned-record"}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given an otherwise valid statement and a bundle that cannot reach network verification.
			raw := sourceBindingStatement(t, sourceBindingStatementOptions{})
			var statement provenance.Statement
			if err := json.Unmarshal(raw, &statement); err != nil {
				t.Fatal(err)
			}
			raw = mustMarshalSignJSON(t, statement)
			original := mustMarshalSignJSON(t, statement.Predicate.RunDetails.Builder.BuilderDependencies)
			if bytes.Count(raw, original) != 1 {
				t.Fatal("expected one builder dependency array")
			}
			replacement := `[{"name":"signing-adapter","uri":"pkg:golang/github.com/sigstore/sigstore-go@v1.4.0","digest":{"dirHash1":"0867457592e1612738db260dd3e94beb25a1eefd33dd2d22db5bf9d25bd260ac"}}` + test.publish + `]`
			raw = bytes.Replace(raw, original, []byte(replacement), 1)
			paths := writeSourceBindingInputs(t,
				sourceBindingPolicy("https://github.com/example/project", "refs/tags/v1.2.3", commandBuiltSHA),
				sourceBindingIdentity("https://github.com/example/project", "1", "2", "refs/heads/main", commandInvocationSHA), raw)
			// When invoking the real command adapter, without replacing its verifier.
			var output bytes.Buffer
			result := NewDispatcher(NewVerifyAttestationCommand()).Dispatch(t.Context(), paths.args(), &output)
			// Then publish semantics fail before the deliberately malformed bundle is verified.
			if result.ExitCode != 1 {
				t.Fatalf("exit code = %d, want 1; report = %s", result.ExitCode, output.String())
			}
			assertPrimaryDiagnostic(t, output.Bytes(), 1, "windlass.verify.error.builder-dependencies-publish-cli-mismatch")
		})
	}
}
