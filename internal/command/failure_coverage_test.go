package command

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
	"github.com/windlasstech/slsa-builder/internal/policy"
)

func TestPublishInvocationFailurePreservesCause(t *testing.T) {
	t.Parallel()
	// Fixed registered constants make the diagnostic-construction panic branches unreachable.
	cause := errors.New("constructor rejected input")
	result, err := publishInvocationFailure(cause)
	if err != cause {
		t.Fatalf("error = %v, want original cause %v", err, cause)
	}
	if result.State != npmprofile.PublishIndeterminate || result.Report.PrimaryID == nil || *result.Report.PrimaryID != diagnostic.IDVerifierExecutionFailure || result.Report.ExitCode != 2 {
		t.Fatalf("invocation failure result = %#v", result)
	}
}

func TestRunPublishRejectsOIDCConstructorInput(t *testing.T) {
	for _, name := range []string{"ACTIONS_ID_TOKEN_REQUEST_URL", "ACTIONS_ID_TOKEN_REQUEST_TOKEN", "GITHUB_WORKFLOW_REF"} {
		t.Setenv(name, "")
	}
	// Empty OIDC variables alone fail preflight, not construction; insecure registry input fails construction.
	// The path after preflight is seam-blocked: it constructs real registry and online verifier clients.
	result, err := runNPMProfilePublish(t.Context(), npmProfilePublishInput{RegistryURL: "http://registry.npmjs.org/"})
	if err == nil || result.State != npmprofile.PublishIndeterminate || result.Report.PrimaryID == nil || *result.Report.PrimaryID != diagnostic.IDVerifierExecutionFailure {
		t.Fatalf("constructor failure = %#v, %v", result, err)
	}
}

func TestPolicyFailureClassification(t *testing.T) {
	t.Parallel()
	entry, err := diagnostic.New("windlass.verify.error.policy-schema-invalid", "policy", "invalid policy")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		cause error
		id    string
	}{
		{"validation", fmt.Errorf("wrapped: %w", &policy.ValidationError{Diagnostic: entry}), entry.ID},
		{"identified", newIdentityPolicyBindingError("windlass.verify.error.duplicate-json-member", "duplicate member"), "windlass.verify.error.duplicate-json-member"},
		{"fallback", errors.New("malformed policy"), "windlass.verify.error.policy-schema-invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			if err := emitPolicyFailure(&output, test.cause); !errors.Is(err, ErrVerificationFailure) {
				t.Fatalf("error = %v, want verification sentinel", err)
			}
			assertPrimaryDiagnostic(t, output.Bytes(), 1, 1, test.id)
		})
	}
}

func TestTypedVerificationFailureClassification(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, id   string
		invocation bool
	}{
		{"policy", "windlass.verify.error.policy-schema-invalid", false},
		{"attestation", "windlass.verify.error.signature-invalid", false},
		{"invocation", diagnostic.IDInputUnavailable, true},
		{"fallback", diagnostic.IDVerifierExecutionFailure, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			entry, err := diagnostic.New(test.id, "verification", "test failure")
			if err != nil {
				t.Fatal(err)
			}
			var cause error = &attestation.VerificationError{Diagnostic: entry}
			switch test.name {
			case "policy":
				cause = &policy.ValidationError{Diagnostic: entry}
			case "fallback":
				cause = errors.New("unclassified failure")
			}
			var output bytes.Buffer
			err = emitTypedVerificationFailure(&output, cause)
			want, code := ErrVerificationFailure, 1
			if test.invocation {
				want, code = ErrInvocationFailure, 2
			}
			if !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
			assertPrimaryDiagnostic(t, output.Bytes(), code, code, test.id)
			writeFailure := errors.New("writer failed")
			if err := emitTypedVerificationFailure(&stagedWriter{err: writeFailure}, cause); !errors.Is(err, writeFailure) {
				t.Fatalf("write error = %v, want original writer error", err)
			}
		})
	}
}
