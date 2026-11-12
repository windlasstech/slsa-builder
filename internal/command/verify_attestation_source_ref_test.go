package command

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
	"github.com/windlasstech/slsa-builder/internal/policy"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const (
	commandBuiltSHA      = "0123456789abcdef0123456789abcdef01234567"
	commandInvocationSHA = "89abcdef0123456789abcdef0123456789abcdef"
)

func TestBindIdentityPolicySourceRefRebinding(t *testing.T) {
	dispatchStatement := sourceBindingStatement(t, sourceBindingStatementOptions{})

	tests := []struct {
		name      string
		statement []byte
		policy    policy.ExplicitPolicy
		identity  attestation.IdentityExpectation
		wantID    string
	}{
		{
			name:      "built policy and invocation certificate contexts accepted",
			statement: dispatchStatement,
			policy:    sourceBindingPolicy("https://github.com/example/project", "refs/tags/v1.2.3", commandBuiltSHA),
			identity:  sourceBindingIdentity("https://github.com/example/project", "1", "2", "refs/heads/main", commandInvocationSHA),
			wantID:    "windlass.verify.error.signature-invalid",
		},
		{
			name:      "policy bound to invocation context rejected",
			statement: dispatchStatement,
			policy:    sourceBindingPolicy("https://github.com/example/project", "refs/heads/main", commandInvocationSHA),
			identity:  sourceBindingIdentity("https://github.com/example/project", "1", "2", "refs/heads/main", commandInvocationSHA),
			wantID:    "windlass.verify.error.policy-schema-invalid",
		},
		{
			name:      "certificate bound to built context rejected",
			statement: dispatchStatement,
			policy:    sourceBindingPolicy("https://github.com/example/project", "refs/tags/v1.2.3", commandBuiltSHA),
			identity:  sourceBindingIdentity("https://github.com/example/project", "1", "2", "refs/tags/v1.2.3", commandBuiltSHA),
			wantID:    "windlass.verify.error.source-ref-mismatch",
		},
		{
			name:      "explicit policy built ref mismatch",
			statement: dispatchStatement,
			policy:    sourceBindingPolicy("https://github.com/example/project", "refs/tags/v1.2.4", commandBuiltSHA),
			identity:  sourceBindingIdentity("https://github.com/example/project", "1", "2", "refs/heads/main", commandInvocationSHA),
			wantID:    "windlass.verify.error.source-ref-mismatch",
		},
		{
			name:      "explicit policy built revision mismatch",
			statement: dispatchStatement,
			policy:    sourceBindingPolicy("https://github.com/example/project", "refs/tags/v1.2.3", commandInvocationSHA),
			identity:  sourceBindingIdentity("https://github.com/example/project", "1", "2", "refs/heads/main", commandInvocationSHA),
			wantID:    "windlass.verify.error.source-digest-mismatch",
		},
		{
			name:      "certificate invocation revision mismatch",
			statement: dispatchStatement,
			policy:    sourceBindingPolicy("https://github.com/example/project", "refs/tags/v1.2.3", commandBuiltSHA),
			identity:  sourceBindingIdentity("https://github.com/example/project", "1", "2", "refs/heads/main", commandBuiltSHA),
			wantID:    "windlass.verify.error.source-digest-mismatch",
		},
		{
			name: "signed repository differs from policy",
			statement: sourceBindingStatement(t, sourceBindingStatementOptions{
				repository: "https://github.com/example/other",
			}),
			policy:   sourceBindingPolicy("https://github.com/example/project", "refs/tags/v1.2.3", commandBuiltSHA),
			identity: sourceBindingIdentity("https://github.com/example/project", "1", "2", "refs/heads/main", commandInvocationSHA),
			wantID:   "windlass.verify.error.source-identity-mismatch",
		},
		{
			name:      "numeric repository identity remains bound",
			statement: dispatchStatement,
			policy:    sourceBindingPolicy("https://github.com/example/project", "refs/tags/v1.2.3", commandBuiltSHA),
			identity:  sourceBindingIdentity("https://github.com/example/project", "9", "2", "refs/heads/main", commandInvocationSHA),
			wantID:    "windlass.verify.error.source-numeric-id-mismatch",
		},
		{
			name:      "missing Build Trigger expectation rejected",
			statement: dispatchStatement,
			policy:    sourceBindingPolicy("https://github.com/example/project", "refs/tags/v1.2.3", commandBuiltSHA),
			identity:  withoutBuildTrigger(sourceBindingIdentity("https://github.com/example/project", "1", "2", "refs/heads/main", commandInvocationSHA)),
			wantID:    "windlass.verify.error.build-trigger-mismatch",
		},
		{
			name:      "conflicting consumer Build Config URI pin rejected",
			statement: dispatchStatement,
			policy:    sourceBindingPolicy("https://github.com/example/project", "refs/tags/v1.2.3", commandBuiltSHA),
			identity:  withBuildConfigURI(sourceBindingIdentity("https://github.com/example/project", "1", "2", "refs/heads/main", commandInvocationSHA), "https://github.com/example/project/.github/workflows/other.yml@refs/heads/main"),
			wantID:    "windlass.verify.error.build-config-uri-mismatch",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths := writeSourceBindingInputs(t, test.policy, test.identity, test.statement)
			var output bytes.Buffer
			result := NewDispatcher(NewVerifyAttestationCommand()).Dispatch(context.Background(), paths.args(), &output)
			requireSourceBindingDiagnostic(t, output.Bytes(), result.ExitCode, test.wantID)
		})
	}
}

func TestVerifyAttestationKeepsValidationCheckField(t *testing.T) {
	t.Parallel()
	raw := sourceBindingStatement(t, sourceBindingStatementOptions{})
	var statement provenance.Statement
	if err := json.Unmarshal(raw, &statement); err != nil {
		t.Fatal(err)
	}
	for index := range statement.Predicate.BuildDefinition.ResolvedDependencies {
		dependency := &statement.Predicate.BuildDefinition.ResolvedDependencies[index]
		if dependency.Name == "runner-image" {
			delete(dependency.Annotations, "operating_system")
		}
	}
	encoded, err := statement.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	paths := writeSourceBindingInputs(t,
		sourceBindingPolicy("https://github.com/example/project", "refs/tags/v1.2.3", commandBuiltSHA),
		sourceBindingIdentity("https://github.com/example/project", "1", "2", "refs/heads/main", commandInvocationSHA),
		encoded)
	var output bytes.Buffer
	result := NewDispatcher(NewVerifyAttestationCommand()).Dispatch(context.Background(), paths.args(), &output)
	if result.ExitCode != 1 {
		t.Fatalf("exit code = %d, want 1; output=%s", result.ExitCode, output.String())
	}
	var report diagnostic.Report
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.PrimaryID == nil || *report.PrimaryID != "windlass.verify.error.resolved-dependencies-runner-image" {
		t.Fatalf("primary ID = %v, want runner-image rejection", report.PrimaryID)
	}
	if len(report.Diagnostics) == 0 || report.Diagnostics[0].Check != "resolvedDependencies.runner-image" {
		t.Fatalf("check field = %#v, want the failing descriptor path, not a generic label", report.Diagnostics)
	}
}

func requireSourceBindingDiagnostic(t *testing.T, output []byte, exitCode int, wantID string) {
	t.Helper()
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1; output=%s", exitCode, output)
	}
	var report Report
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("decode report: %v; output=%s", err, output)
	}
	if report.PrimaryID == nil {
		t.Fatalf("primary ID = nil, want %s", wantID)
	}
	if *report.PrimaryID != wantID {
		t.Fatalf("primary ID = %s, want %s", *report.PrimaryID, wantID)
	}
}

type sourceBindingStatementOptions struct {
	repository    string
	invocationRef string
}

func sourceBindingStatement(t *testing.T, options sourceBindingStatementOptions) []byte {
	t.Helper()
	predicateBytes, err := os.ReadFile(filepath.Join("..", "..", "testdata", "npm", "provenance", "npm-predicate-source-ref-dispatch-retry.jcs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var predicate provenance.Predicate
	if err := json.Unmarshal(predicateBytes, &predicate); err != nil {
		t.Fatal(err)
	}
	parameters, err := npmprofile.DecodeExternalParameters(predicate.BuildDefinition.ExternalParameters)
	if err != nil {
		t.Fatal(err)
	}
	predicate.BuildDefinition.InternalParameters = json.RawMessage(`{"github":{"event_name":"workflow_dispatch","repository_id":"1","repository_owner_id":"2"}}`)
	if options.repository != "" {
		for index := range predicate.BuildDefinition.ResolvedDependencies {
			d := &predicate.BuildDefinition.ResolvedDependencies[index]
			d.URI = strings.Replace(d.URI, "git+"+parameters.Workflow.Repository+"@", "git+"+options.repository+"@", 1)
		}
		parameters.Workflow.Repository = options.repository
	}
	if options.invocationRef != "" {
		parameters.Workflow.Ref = options.invocationRef
		for index := range predicate.BuildDefinition.ResolvedDependencies {
			if predicate.BuildDefinition.ResolvedDependencies[index].Name == "invocation-source" {
				predicate.BuildDefinition.ResolvedDependencies[index].URI = "git+" + parameters.Workflow.Repository + "@" + options.invocationRef
			}
		}
	}
	predicate.BuildDefinition.ExternalParameters, err = npmprofile.EncodeExternalParameters(parameters)
	if err != nil {
		t.Fatal(err)
	}
	_, tarball := signSingleIdentityMetadata(t)
	statement := provenance.Statement{
		Type:          provenance.StatementType,
		Subject:       []provenance.Subject{{Name: "pkg:npm/%40windlass/slsa-builder@1.2.3", Digest: map[string]string{"sha256": digest.SumSHA256(tarball).String(), "sha512": digest.SumSHA512(tarball).String()}}},
		PredicateType: provenance.PredicateType,
		Predicate:     predicate,
	}
	encoded, err := statement.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func sourceBindingPolicy(repository, ref, revision string) policy.ExplicitPolicy {
	return policy.ExplicitPolicy{
		SchemaVersion: "1",
		Source: policy.SourcePolicy{
			RepositoryURI: repository, RepositoryID: "1", RepositoryOwnerID: "2", Digest: revision, Ref: ref,
		},
		Producer: policy.ProducerPolicy{
			WorkflowPath: npmprofile.NPMWorkflowPath, WorkflowSHA: commandBuiltSHA, RunnerEnvironment: "github-hosted",
		},
		TrustRoot: policy.TrustRoot{Mode: "tuf", Instance: "sigstore-public-good"},
	}
}

func sourceBindingIdentity(repository, repositoryID, ownerID, ref, revision string) attestation.IdentityExpectation {
	return attestation.IdentityExpectation{
		Issuer:                  "https://token.actions.githubusercontent.com",
		SignerURI:               "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@refs/heads/main",
		WorkflowSHA:             commandBuiltSHA,
		SourceRepositoryURI:     repository,
		SourceRepositoryID:      repositoryID,
		SourceRepositoryOwnerID: ownerID,
		SourceDigest:            revision,
		SourceRef:               ref,
		BuildTrigger:            "workflow_dispatch",
		BuildConfigURI:          repository + "/.github/workflows/release.yml@" + ref,
		RunnerEnvironment:       "github-hosted",
		RunInvocationURI:        "https://github.com/example/project/actions/runs/123456789/attempts/1",
	}
}

func withoutBuildTrigger(identity attestation.IdentityExpectation) attestation.IdentityExpectation {
	identity.BuildTrigger = ""
	return identity
}

func withBuildConfigURI(identity attestation.IdentityExpectation, value string) attestation.IdentityExpectation {
	identity.BuildConfigURI = value
	return identity
}

func writeSourceBindingInputs(t *testing.T, explicit policy.ExplicitPolicy, identity attestation.IdentityExpectation, statement []byte) attestationInputPaths {
	t.Helper()
	policyJSON, err := json.Marshal(explicit)
	if err != nil {
		t.Fatal(err)
	}
	identityJSON, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	paths := writeAttestationInputs(t, string(policyJSON), string(identityJSON))
	if err := os.WriteFile(paths.statement, statement, 0o600); err != nil {
		t.Fatal(err)
	}
	return paths
}
