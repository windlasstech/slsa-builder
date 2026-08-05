package identity

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const (
	workflowPath = ".github/workflows/js-ts-npm-package-slsa3.yml"
	workflowSHA  = "89abcdef0123456789abcdef0123456789abcdef"
	sourceSHA    = "0123456789abcdef0123456789abcdef01234567"
)

func TestCanonicalRepository(t *testing.T) {
	valid := map[string]string{
		"owner/repository":                                       "https://github.com/owner/repository",
		"github:WindlassTech/Example":                            "https://github.com/windlasstech/example",
		"https://github.com/WindlassTech/Example":                "https://github.com/windlasstech/example",
		"git+https://github.com/WindlassTech/Example.git":        "https://github.com/windlasstech/example",
		"git://github.com/WindlassTech/Example/":                 "https://github.com/windlasstech/example",
		"git@github.com:WindlassTech/Example.git":                "https://github.com/windlasstech/example",
		"ssh://git@github.com/WindlassTech/Example.git":          "https://github.com/windlasstech/example",
		"https://github.com/WindlassTech/repository_with-dashes": "https://github.com/windlasstech/repository_with-dashes",
	}
	for input, want := range valid {
		t.Run(input, func(t *testing.T) {
			got, err := CanonicalRepository(input)
			if err != nil {
				t.Fatalf("CanonicalRepository() error = %v", err)
			}
			if got != want {
				t.Fatalf("CanonicalRepository() = %q, want %q", got, want)
			}
		})
	}

	invalid := []string{
		"",
		"gitlab:WindlassTech/Example",
		"https://gitlab.com/WindlassTech/Example",
		"https://github.com/WindlassTech/Example/releases",
		"https://github.com/WindlassTech/Example?tab=readme",
		"https://token@github.com/WindlassTech/Example",
		"https://github.com:443/WindlassTech/Example",
		"https://github.com/WindlassTech/%2e%2e/Example",
		"https://github.com/WindlassTech//Example",
		"WindlassTech\\Example",
		"WindlassTech/Example.git",
		"github:WindlassTech/Example.git",
		"git@github.com:WindlassTech/Example",
		"git@github.com:WindlassTech/Example.git/",
		"ssh://git@github.com/WindlassTech/Example",
	}
	for _, input := range invalid {
		t.Run("reject_"+input, func(t *testing.T) {
			_, err := CanonicalRepository(input)
			requireDiagnostic(t, err, IDPackageRepositoryIdentityMismatch)
		})
	}

	if err := ValidateCanonicalRepositoryURI("https://github.com/example/acme-widget"); err != nil {
		t.Fatalf("ValidateCanonicalRepositoryURI() error = %v", err)
	}
	requireDiagnostic(t,
		ValidateCanonicalRepositoryURI("https://github.com/Example/acme-widget"),
		IDSourceIdentityMismatch,
	)
}

func TestBuilderID(t *testing.T) {
	want := "https://github.com/windlasstech/slsa-builder/" + workflowPath + "@" + workflowSHA
	got, err := NewBuilderID(workflowPath, workflowSHA)
	if err != nil {
		t.Fatalf("NewBuilderID() error = %v", err)
	}
	if got != want {
		t.Fatalf("NewBuilderID() = %q, want %q", got, want)
	}
	if err := ValidateBuilderID(got); err != nil {
		t.Fatalf("ValidateBuilderID() error = %v", err)
	}

	invalidRefs := []string{"main", "v1", "v1.2.3", "89abcde", "89ABCDEF0123456789ABCDEF0123456789ABCDEF"}
	for _, ref := range invalidRefs {
		t.Run("reject_ref_"+ref, func(t *testing.T) {
			candidate := "https://github.com/windlasstech/slsa-builder/" + workflowPath + "@" + ref
			requireDiagnostic(t, ValidateBuilderID(candidate), IDBuilderIDNotImmutable)
		})
	}
	requireDiagnostic(t, ValidateFullSHA("v1.2.3"), IDBuilderIDNotImmutable)
	requireDiagnostic(t, ValidateFullSHA("89abcde"), IDBuilderIDNotImmutable)

	if err := ValidateReleaseRef("refs/tags/v1.2.3"); err != nil {
		t.Fatalf("ValidateReleaseRef() error = %v", err)
	}
	requireDiagnostic(t, ValidateReleaseRef("refs/heads/main"), IDSourceRefMismatch)

	buildType, err := NewBuildTypeURI("js-ts-npm-package", 1)
	if err != nil {
		t.Fatalf("NewBuildTypeURI() error = %v", err)
	}
	if buildType != "https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1" {
		t.Fatalf("NewBuildTypeURI() = %q", buildType)
	}
	if err := ValidateBuildTypeURI(buildType); err != nil {
		t.Fatalf("ValidateBuildTypeURI() error = %v", err)
	}
	for _, candidate := range []string{
		"https://windlasstech.github.io/slsa-builder/js-ts-npm-package/v1",
		"https://buildtype.dev/windlass/slsa-builder/js%2fts-npm-package/v1",
		"https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v01",
		"https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v0",
	} {
		requireDiagnostic(t, ValidateBuildTypeURI(candidate), IDBuildTypeNotCanonical)
	}
}

func TestMaximalOIDCBinding(t *testing.T) {
	fixture := loadFixture[maximalIdentityFixture](t, "npm-maximal-identity-valid.json")
	got, err := ValidateMaximalOIDCBinding(fixture.Claims, fixture.Policy)
	if err != nil {
		t.Fatalf("ValidateMaximalOIDCBinding() error = %v", err)
	}
	if got.BuilderID != fixture.BuilderID {
		t.Fatalf("BuilderID = %q, want %q", got.BuilderID, fixture.BuilderID)
	}
	if got.RunInvocationURI != fixture.RunInvocationURI {
		t.Fatalf("RunInvocationURI = %q, want %q", got.RunInvocationURI, fixture.RunInvocationURI)
	}

	mutationFiles := []string{
		"identity-issuer-mismatch.json",
		"identity-workflow-sha-mismatch.json",
		"identity-source-repository-id-mismatch.json",
		"identity-source-digest-mismatch.json",
		"identity-run-invocation-malformed.json",
		"identity-self-hosted-runner.json",
	}
	for _, filename := range mutationFiles {
		mutation := loadFixture[identityMutationFixture](t, filename)
		t.Run(filename, func(t *testing.T) {
			claims := fixture.Claims
			applyClaimMutation(t, &claims, mutation.Field, mutation.Value)
			_, err := ValidateMaximalOIDCBinding(claims, fixture.Policy)
			requireDiagnostic(t, err, mutation.ExpectedDiagnostic)
		})
	}

	t.Run("workflow_path_mismatch", func(t *testing.T) {
		claims := fixture.Claims
		claims.JobWorkflowRef = "windlasstech/slsa-builder/.github/workflows/other.yml@" + workflowSHA
		_, err := ValidateMaximalOIDCBinding(claims, fixture.Policy)
		requireDiagnostic(t, err, IDSignerWorkflowPathMismatch)
	})
	t.Run("workflow_ref_must_be_full_sha", func(t *testing.T) {
		claims := fixture.Claims
		claims.JobWorkflowRef = "windlasstech/slsa-builder/" + workflowPath + "@v1.2.3"
		_, err := ValidateMaximalOIDCBinding(claims, fixture.Policy)
		requireDiagnostic(t, err, IDSignerWorkflowSHAMismatch)
	})
	t.Run("owner_numeric_id_decides", func(t *testing.T) {
		claims := fixture.Claims
		claims.RepositoryOwnerID = "1234"
		_, err := ValidateMaximalOIDCBinding(claims, fixture.Policy)
		requireDiagnostic(t, err, IDSourceNumericIDMismatch)
	})
	t.Run("source_ref_mismatch", func(t *testing.T) {
		claims := fixture.Claims
		claims.Ref = "refs/tags/v9.9.9"
		_, err := ValidateMaximalOIDCBinding(claims, fixture.Policy)
		requireDiagnostic(t, err, IDSourceRefMismatch)
	})
	t.Run("ghes_is_explicitly_unsupported", func(t *testing.T) {
		policy := fixture.Policy
		policy.Platform = Platform("ghe.example.com")
		_, err := ValidateMaximalOIDCBinding(fixture.Claims, policy)
		requireDiagnostic(t, err, IDSignerIdentityClaimMissing)
	})
}

func TestRunInvocationURI(t *testing.T) {
	const repository = "https://github.com/example/acme-widget"
	want := repository + "/actions/runs/30745570800/attempts/2"
	got, err := NewRunInvocationURI(repository, "30745570800", "2")
	if err != nil {
		t.Fatalf("NewRunInvocationURI() error = %v", err)
	}
	if got != want {
		t.Fatalf("NewRunInvocationURI() = %q, want %q", got, want)
	}
	parsed, err := ParseRunInvocationURI(got, repository)
	if err != nil {
		t.Fatalf("ParseRunInvocationURI() error = %v", err)
	}
	if parsed.RunID != "30745570800" || parsed.Attempt != "2" {
		t.Fatalf("ParseRunInvocationURI() = %#v", parsed)
	}

	invalid := []string{
		repository + "/actions/runs/0/attempts/2",
		repository + "/actions/runs/+1/attempts/2",
		repository + "/actions/runs/01/attempts/2",
		repository + "/actions/runs/1/attempts/0",
		repository + "/actions/runs/1/attempts/2?download=1",
		"https://github.com/other/acme-widget/actions/runs/1/attempts/2",
		"https://user@github.com/example/acme-widget/actions/runs/1/attempts/2",
		"https://github.com:443/example/acme-widget/actions/runs/1/attempts/2",
	}
	for _, candidate := range invalid {
		t.Run(candidate, func(t *testing.T) {
			_, err := ParseRunInvocationURI(candidate, repository)
			requireDiagnostic(t, err, IDRunInvocationURIInvalid)
		})
	}
}

type maximalIdentityFixture struct {
	Claims           OIDCClaims    `json:"claims"`
	Policy           BindingPolicy `json:"policy"`
	BuilderID        string        `json:"builder_id"`
	RunInvocationURI string        `json:"run_invocation_uri"`
}

type identityMutationFixture struct {
	Field              string       `json:"field"`
	Value              string       `json:"value"`
	ExpectedDiagnostic DiagnosticID `json:"expected_diagnostic"`
}

func loadFixture[T any](t *testing.T, name string) T {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "identity", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var fixture T
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	return fixture
}

func applyClaimMutation(t *testing.T, claims *OIDCClaims, field, value string) {
	t.Helper()
	switch field {
	case "iss":
		claims.Issuer = value
	case "job_workflow_sha":
		claims.JobWorkflowSHA = value
	case "repository_id":
		claims.RepositoryID = value
	case "sha":
		claims.SHA = value
	case "run_attempt":
		claims.RunAttempt = value
	case "runner_environment":
		claims.RunnerEnvironment = value
	default:
		t.Fatalf("unsupported mutation field %q", field)
	}
}

func requireDiagnostic(t *testing.T, err error, want DiagnosticID) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected diagnostic %q, got nil", want)
	}
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if validationError.ID != want {
		t.Fatalf("diagnostic = %q, want %q: %v", validationError.ID, want, err)
	}
	if validationError.DiagnosticID() != string(want) {
		t.Fatalf("DiagnosticID() = %q, want %q", validationError.DiagnosticID(), want)
	}
}
