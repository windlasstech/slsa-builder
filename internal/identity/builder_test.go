package identity

import (
	"errors"
	"testing"
)

func TestValidationErrorFormat(t *testing.T) {
	t.Parallel()

	t.Run("with field", func(t *testing.T) {
		t.Parallel()
		err := &ValidationError{ID: IDBuilderIDNotImmutable, Field: "builder.id", Message: "workflow path rejected"}
		const want = "windlass.verify.error.builder-id-not-immutable: builder.id: workflow path rejected"
		if got := err.Error(); got != want {
			t.Fatalf("ValidationError.Error() = %q, want %q", got, want)
		}
	})

	t.Run("without field", func(t *testing.T) {
		t.Parallel()
		err := &ValidationError{ID: IDBuilderIDNotImmutable, Message: "builder identity rejected"}
		const want = "windlass.verify.error.builder-id-not-immutable: builder identity rejected"
		if got := err.Error(); got != want {
			t.Fatalf("ValidationError.Error() = %q, want %q", got, want)
		}
	})
}

func TestNewBuilderIDComposition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "yml extension", path: ".github/workflows/js-ts-npm-package-slsa3.yml"},
		{name: "yaml extension", path: ".github/workflows/js-ts-npm-package-slsa3.yaml"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			want := "https://github.com/windlasstech/slsa-builder/" + test.path + "@" + workflowSHA
			got, err := NewBuilderID(test.path, workflowSHA)
			if err != nil {
				t.Fatalf("NewBuilderID() error = %v", err)
			}
			if got != want {
				t.Fatalf("NewBuilderID() = %q, want %q", got, want)
			}
		})
	}
}

func TestNewBuilderIDRejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		path      string
		sha       string
		wantField string
	}{
		{name: "path outside workflows directory", path: "workflows/build.yml", sha: workflowSHA, wantField: "builder.id"},
		{name: "path percent-encoded", path: ".github/workflows/%2e%2e.yml", sha: workflowSHA, wantField: "builder.id"},
		{name: "path with backslash separator", path: `.github/workflows\nested\build.yml`, sha: workflowSHA, wantField: "builder.id"},
		{name: "path nested below workflows directory", path: ".github/workflows/nested/build.yml", sha: workflowSHA, wantField: "builder.id"},
		{name: "empty filename", path: ".github/workflows/", sha: workflowSHA, wantField: "builder.id"},
		{name: "dotfile filename", path: ".github/workflows/.build.yml", sha: workflowSHA, wantField: "builder.id"},
		{name: "missing extension", path: ".github/workflows/build", sha: workflowSHA, wantField: "builder.id"},
		{name: "unsupported extension", path: ".github/workflows/build.yaml.bak", sha: workflowSHA, wantField: "builder.id"},
		{name: "space in extension stem", path: ".github/workflows/bad stem.yml", sha: workflowSHA, wantField: "builder.id"},
		{name: "uppercase SHA", path: workflowPath, sha: "89ABCDEF0123456789ABCDEF0123456789ABCDEF", wantField: "workflow_sha"},
		{name: "short SHA", path: workflowPath, sha: "89abcde", wantField: "workflow_sha"},
		{name: "tag reference instead of SHA", path: workflowPath, sha: "v1.2.3", wantField: "workflow_sha"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewBuilderID(test.path, test.sha)
			requireDiagnosticField(t, err, IDBuilderIDNotImmutable, test.wantField)
		})
	}
}

func TestNewBuildTypeURIComposition(t *testing.T) {
	t.Parallel()

	t.Run("valid composition", func(t *testing.T) {
		t.Parallel()
		got, err := NewBuildTypeURI("js-ts-npm-package", 1)
		if err != nil {
			t.Fatalf("NewBuildTypeURI() error = %v", err)
		}
		const want = "https://buildtype.dev/windlass/slsa-builder/js-ts-npm-package/v1"
		if got != want {
			t.Fatalf("NewBuildTypeURI() = %q, want %q", got, want)
		}
	})

	t.Run("valid composition with multi-digit major", func(t *testing.T) {
		t.Parallel()
		got, err := NewBuildTypeURI("npm", 12)
		if err != nil {
			t.Fatalf("NewBuildTypeURI() error = %v", err)
		}
		const want = "https://buildtype.dev/windlass/slsa-builder/npm/v12"
		if got != want {
			t.Fatalf("NewBuildTypeURI() = %q, want %q", got, want)
		}
	})

	rejections := []struct {
		name    string
		profile string
		major   uint64
	}{
		{name: "profile with path separator", profile: "js/ts", major: 1},
		{name: "empty profile", profile: "", major: 1},
		{name: "zero major version", profile: "js-ts-npm-package", major: 0},
	}
	for _, test := range rejections {
		t.Run("reject_"+test.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewBuildTypeURI(test.profile, test.major)
			requireDiagnosticField(t, err, IDBuildTypeNotCanonical, "build_type")
		})
	}
}

func TestCanonicalRepositorySchemeRejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "ssh with non-git user", input: "ssh://root@github.com/WindlassTech/Example.git"},
		{name: "ssh with password", input: "ssh://git:secret@github.com/WindlassTech/Example.git"},
		{name: "unsupported scheme", input: "ftp://github.com/WindlassTech/Example"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := CanonicalRepository(test.input)
			requireDiagnosticField(t, err, IDPackageRepositoryIdentityMismatch, "repository")
		})
	}
}

func TestRunInvocationURIValidation(t *testing.T) {
	t.Parallel()

	const repository = "https://github.com/example/acme-widget"

	t.Run("construct rejects non-canonical repository", func(t *testing.T) {
		t.Parallel()
		_, err := NewRunInvocationURI("https://github.com/Example/acme-widget", "1", "1")
		requireDiagnosticField(t, err, IDRunInvocationURIInvalid, "run_invocation")
	})

	t.Run("parse rejects non-canonical expected repository", func(t *testing.T) {
		t.Parallel()
		_, err := ParseRunInvocationURI(repository+"/actions/runs/1/attempts/2", "https://github.com/Example/acme-widget")
		requireDiagnosticField(t, err, IDRunInvocationURIInvalid, "run_invocation")
	})

	t.Run("parse rejects percent-encoded invocation URI", func(t *testing.T) {
		t.Parallel()
		_, err := ParseRunInvocationURI(repository+"/actions/runs/1/attempts/2%2f", repository)
		requireDiagnosticField(t, err, IDRunInvocationURIInvalid, "run_invocation")
	})

	t.Run("parse rejects whitespace-padded invocation URI", func(t *testing.T) {
		t.Parallel()
		_, err := ParseRunInvocationURI(" "+repository+"/actions/runs/1/attempts/2", repository)
		requireDiagnosticField(t, err, IDRunInvocationURIInvalid, "run_invocation")
	})
}

func TestMaximalOIDCBindingRepositoryMismatch(t *testing.T) {
	t.Parallel()

	fixture := loadFixture[maximalIdentityFixture](t, "npm-maximal-identity-valid.json")

	t.Run("claims repository canonicalizes to another URI", func(t *testing.T) {
		t.Parallel()
		claims := fixture.Claims
		claims.Repository = "example/other-widget"
		_, err := ValidateMaximalOIDCBinding(claims, fixture.Policy)
		requireDiagnosticField(t, err, IDSourceIdentityMismatch, "repository")
	})

	t.Run("claims repository is not a supported locator", func(t *testing.T) {
		t.Parallel()
		claims := fixture.Claims
		claims.Repository = "https://gitlab.com/example/acme-widget"
		_, err := ValidateMaximalOIDCBinding(claims, fixture.Policy)
		requireDiagnosticField(t, err, IDSourceIdentityMismatch, "repository")
	})
}

func requireDiagnosticField(t *testing.T, err error, wantID DiagnosticID, wantField string) {
	t.Helper()
	requireDiagnostic(t, err, wantID)
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if validationError.Field != wantField {
		t.Fatalf("diagnostic field = %q, want %q: %v", validationError.Field, wantField, err)
	}
}
