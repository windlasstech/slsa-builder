package policy_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/policy"
)

const (
	testSourceObject = `"source":{"repository_uri":"https://github.com/example/acme-widget",` +
		`"repository_id":"123456789","repository_owner_id":"9876543",` +
		`"digest":"0123456789abcdef0123456789abcdef01234567","ref":"refs/tags/v1.2.3"}`
	testProducerObject = `"producer":{"workflow_path":".github/workflows/js-ts-npm-package-slsa3.yml",` +
		`"workflow_sha":"89abcdef0123456789abcdef0123456789abcdef","runner_environment":"github-hosted"}`
	testTUFTrustRootObject = `"trust_root":{"mode":"tuf","instance":"sigstore-public-good"}`
	testPinnedTrustRoot    = `"trust_root":{"mode":"pinned","instance":"sigstore-public-good",` +
		`"path":"trusted_root.json",` +
		`"sha256":"7c222fb2927d828af22f592134e8932480637c0d3f9c2072e82716801567e69f",` +
		`"tuf_repository":"https://tuf-repo-cdn.sigstore.dev",` +
		`"revalidated_at":"2026-08-01T00:00:00Z","refresh_before":"2026-08-08T00:00:00Z"}`

	validTUFExplicitPolicyDocument = `{"schema_version":"1",` + testSourceObject + `,` +
		testProducerObject + `,` + testTUFTrustRootObject + `}`
	validPinnedExplicitPolicyDocument = `{"schema_version":"1",` + testSourceObject + `,` +
		testProducerObject + `,` + testPinnedTrustRoot + `}`

	testManifestIdentityObject = `"release_manifest":{"source_repository_uri":"https://github.com/windlasstech/slsa-builder",` +
		`"source_repository_id":"102030405","source_repository_owner_id":"5060708",` +
		`"workflow_path":".github/workflows/release-manifest.yml",` +
		`"workflow_sha":"89abcdef0123456789abcdef0123456789abcdef"}`
	testProducerProfileObject = `"producer_profile":{"profile":"js-ts-npm-package",` +
		`"workflow_path":".github/workflows/js-ts-npm-package-slsa3.yml",` +
		`"workflow_sha":"89abcdef0123456789abcdef0123456789abcdef"}`

	validManifestExpectationDocument = `{"schema_version":"1",` + testManifestIdentityObject + `,` +
		testProducerProfileObject + `}`
)

func TestValidationErrorFormat(t *testing.T) {
	t.Parallel()

	t.Run("with field", func(t *testing.T) {
		t.Parallel()
		err := &policy.ValidationError{Diagnostic: diagnostic.Diagnostic{
			ID:      policySchemaInvalidID,
			Field:   "source.digest",
			Message: "digest malformed",
		}}
		const want = "windlass.verify.error.policy-schema-invalid: source.digest: digest malformed"
		if got := err.Error(); got != want {
			t.Fatalf("ValidationError.Error() = %q, want %q", got, want)
		}
	})

	t.Run("without field", func(t *testing.T) {
		t.Parallel()
		err := &policy.ValidationError{Diagnostic: diagnostic.Diagnostic{
			ID:      policySchemaInvalidID,
			Message: "policy unreadable",
		}}
		const want = "windlass.verify.error.policy-schema-invalid: policy unreadable"
		if got := err.Error(); got != want {
			t.Fatalf("ValidationError.Error() = %q, want %q", got, want)
		}
	})
}

func TestExplicitPolicySourceRejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		anchor      string
		replacement string
		wantField   string
	}{
		{
			name:        "repository URI not canonical",
			anchor:      `"repository_uri":"https://github.com/example/acme-widget"`,
			replacement: `"repository_uri":"https://github.com/Example/acme-widget"`,
			wantField:   "source.repository_uri",
		},
		{
			name:        "repository ID zero",
			anchor:      `"repository_id":"123456789"`,
			replacement: `"repository_id":"0"`,
			wantField:   "source.repository_id",
		},
		{
			name:        "repository ID not decimal",
			anchor:      `"repository_id":"123456789"`,
			replacement: `"repository_id":"abc"`,
			wantField:   "source.repository_id",
		},
		{
			name:        "repository owner ID zero",
			anchor:      `"repository_owner_id":"9876543"`,
			replacement: `"repository_owner_id":"0"`,
			wantField:   "source.repository_owner_id",
		},
		{
			name:        "source digest shortened",
			anchor:      `"digest":"0123456789abcdef0123456789abcdef01234567"`,
			replacement: `"digest":"0123456789abcdef"`,
			wantField:   "source.digest",
		},
		{
			name:        "source digest uppercase",
			anchor:      `"digest":"0123456789abcdef0123456789abcdef01234567"`,
			replacement: `"digest":"0123456789ABCDEF0123456789ABCDEF01234567"`,
			wantField:   "source.digest",
		},
		{
			name:        "source ref not a release tag",
			anchor:      `"ref":"refs/tags/v1.2.3"`,
			replacement: `"ref":"refs/heads/main"`,
			wantField:   "source.ref",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			document := mutateDocument(t, validTUFExplicitPolicyDocument, test.anchor, test.replacement)
			_, err := policy.DecodeExplicitPolicy([]byte(document))
			requirePolicyDiagnostic(t, err, policySchemaInvalidID, test.wantField, "")
		})
	}
}

func TestExplicitPolicyProducerAndTrustRootRejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		anchor      string
		replacement string
		wantField   string
	}{
		{
			name:        "schema version unsupported",
			anchor:      `"schema_version":"1"`,
			replacement: `"schema_version":"2"`,
			wantField:   "schema_version",
		},
		{
			name:        "producer workflow path nested below workflows directory",
			anchor:      `"workflow_path":".github/workflows/js-ts-npm-package-slsa3.yml"`,
			replacement: `"workflow_path":".github/workflows/nested/build.yml"`,
			wantField:   "producer",
		},
		{
			name:        "producer workflow SHA not immutable",
			anchor:      `"workflow_sha":"89abcdef0123456789abcdef0123456789abcdef"`,
			replacement: `"workflow_sha":"v1.2.3"`,
			wantField:   "producer",
		},
		{
			name:        "runner environment self-hosted",
			anchor:      `"runner_environment":"github-hosted"`,
			replacement: `"runner_environment":"self-hosted"`,
			wantField:   "producer.runner_environment",
		},
		{
			name:        "trust root instance unrecognized",
			anchor:      `"instance":"sigstore-public-good"`,
			replacement: `"instance":"private-sigstore"`,
			wantField:   "trust_root.instance",
		},
		{
			name:        "trust root mode unrecognized",
			anchor:      `"mode":"tuf"`,
			replacement: `"mode":"file"`,
			wantField:   "trust_root.mode",
		},
		{
			name:        "TUF trust root carries pinned-root member",
			anchor:      testTUFTrustRootObject,
			replacement: `"trust_root":{"mode":"tuf","instance":"sigstore-public-good","path":"trusted_root.json"}`,
			wantField:   "trust_root",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			document := mutateDocument(t, validTUFExplicitPolicyDocument, test.anchor, test.replacement)
			_, err := policy.DecodeExplicitPolicy([]byte(document))
			requirePolicyDiagnostic(t, err, policySchemaInvalidID, test.wantField, "")
		})
	}
}

func TestPinnedTrustRootValidation(t *testing.T) {
	t.Parallel()

	t.Run("valid pinned trust root decodes", func(t *testing.T) {
		t.Parallel()
		decoded, err := policy.DecodeExplicitPolicy([]byte(validPinnedExplicitPolicyDocument))
		if err != nil {
			t.Fatalf("DecodeExplicitPolicy() pinned trust root error = %v", err)
		}
		if decoded.TrustRoot.RefreshBefore == nil || *decoded.TrustRoot.RefreshBefore != "2026-08-08T00:00:00Z" {
			t.Fatalf("TrustRoot.RefreshBefore = %v, want %q", decoded.TrustRoot.RefreshBefore, "2026-08-08T00:00:00Z")
		}
	})

	tests := []struct {
		name        string
		anchor      string
		replacement string
		wantField   string
	}{
		{
			name:        "path missing",
			anchor:      `,"path":"trusted_root.json"`,
			replacement: "",
			wantField:   "trust_root.path",
		},
		{
			name:        "path empty",
			anchor:      `"path":"trusted_root.json"`,
			replacement: `"path":""`,
			wantField:   "trust_root.path",
		},
		{
			name:        "sha256 missing",
			anchor:      `,"sha256":"7c222fb2927d828af22f592134e8932480637c0d3f9c2072e82716801567e69f"`,
			replacement: "",
			wantField:   "trust_root.sha256",
		},
		{
			name:        "sha256 wrong length",
			anchor:      `"sha256":"7c222fb2927d828af22f592134e8932480637c0d3f9c2072e82716801567e69f"`,
			replacement: `"sha256":"7c222f"`,
			wantField:   "trust_root.sha256",
		},
		{
			name:        "sha256 uppercase",
			anchor:      `"sha256":"7c222fb2927d828af22f592134e8932480637c0d3f9c2072e82716801567e69f"`,
			replacement: `"sha256":"7C222FB2927D828AF22F592134E8932480637C0D3F9C2072E82716801567E69F"`,
			wantField:   "trust_root.sha256",
		},
		{
			name:        "tuf repository missing",
			anchor:      `,"tuf_repository":"https://tuf-repo-cdn.sigstore.dev"`,
			replacement: "",
			wantField:   "trust_root.tuf_repository",
		},
		{
			name:        "tuf repository not governed",
			anchor:      `"tuf_repository":"https://tuf-repo-cdn.sigstore.dev"`,
			replacement: `"tuf_repository":"https://tuf.example.com"`,
			wantField:   "trust_root.tuf_repository",
		},
		{
			name:        "revalidated_at missing",
			anchor:      `,"revalidated_at":"2026-08-01T00:00:00Z"`,
			replacement: "",
			wantField:   "trust_root.revalidated_at",
		},
		{
			name:        "revalidated_at not whole-second UTC",
			anchor:      `"revalidated_at":"2026-08-01T00:00:00Z"`,
			replacement: `"revalidated_at":"2026-08-01 00:00:00"`,
			wantField:   "trust_root.revalidated_at",
		},
		{
			name:        "revalidated_at impossible calendar date",
			anchor:      `"revalidated_at":"2026-08-01T00:00:00Z"`,
			replacement: `"revalidated_at":"2026-02-30T00:00:00Z"`,
			wantField:   "trust_root.revalidated_at",
		},
		{
			name:        "refresh_before missing",
			anchor:      `,"refresh_before":"2026-08-08T00:00:00Z"`,
			replacement: "",
			wantField:   "trust_root.refresh_before",
		},
		{
			name:        "refresh_before not whole-second UTC",
			anchor:      `"refresh_before":"2026-08-08T00:00:00Z"`,
			replacement: `"refresh_before":"2026-08-08"`,
			wantField:   "trust_root.refresh_before",
		},
		{
			name:        "refresh_before equals revalidated_at",
			anchor:      `"refresh_before":"2026-08-08T00:00:00Z"`,
			replacement: `"refresh_before":"2026-08-01T00:00:00Z"`,
			wantField:   "trust_root.refresh_before",
		},
		{
			name:        "refresh_before earlier than revalidated_at",
			anchor:      `"refresh_before":"2026-08-08T00:00:00Z"`,
			replacement: `"refresh_before":"2026-07-01T00:00:00Z"`,
			wantField:   "trust_root.refresh_before",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			document := mutateDocument(t, validPinnedExplicitPolicyDocument, test.anchor, test.replacement)
			_, err := policy.DecodeExplicitPolicy([]byte(document))
			requirePolicyDiagnostic(t, err, policySchemaInvalidID, test.wantField, "")
		})
	}
}

func TestReleaseManifestExpectationRejections(t *testing.T) {
	t.Parallel()

	registry := producerRegistry{"js-ts-npm-package": {}}

	tests := []struct {
		name        string
		anchor      string
		replacement string
		wantField   string
	}{
		{
			name:        "schema version unsupported",
			anchor:      `"schema_version":"1"`,
			replacement: `"schema_version":"2"`,
			wantField:   "schema_version",
		},
		{
			name:        "source repository URI not canonical",
			anchor:      `"source_repository_uri":"https://github.com/windlasstech/slsa-builder"`,
			replacement: `"source_repository_uri":"https://github.com/WindlassTech/slsa-builder"`,
			wantField:   "release_manifest.source_repository_uri",
		},
		{
			name:        "source repository ID zero",
			anchor:      `"source_repository_id":"102030405"`,
			replacement: `"source_repository_id":"0"`,
			wantField:   "release_manifest.source_repository_id",
		},
		{
			name:        "source repository owner ID not decimal",
			anchor:      `"source_repository_owner_id":"5060708"`,
			replacement: `"source_repository_owner_id":"owner"`,
			wantField:   "release_manifest.source_repository_owner_id",
		},
		{
			name:        "release manifest workflow path outside workflows directory",
			anchor:      `"workflow_path":".github/workflows/release-manifest.yml"`,
			replacement: `"workflow_path":"workflows/release-manifest.yml"`,
			wantField:   "release_manifest",
		},
		{
			name: "release manifest workflow SHA not immutable",
			anchor: `"workflow_path":".github/workflows/release-manifest.yml",` +
				`"workflow_sha":"89abcdef0123456789abcdef0123456789abcdef"`,
			replacement: `"workflow_path":".github/workflows/release-manifest.yml",` +
				`"workflow_sha":"89abcdef"`,
			wantField: "release_manifest",
		},
		{
			name:        "producer profile empty",
			anchor:      `"profile":"js-ts-npm-package"`,
			replacement: `"profile":""`,
			wantField:   "producer_profile.profile",
		},
		{
			name:        "producer profile unregistered",
			anchor:      `"profile":"js-ts-npm-package"`,
			replacement: `"profile":"rust-crate"`,
			wantField:   "producer_profile.profile",
		},
		{
			name:        "producer profile workflow path not canonical",
			anchor:      `"workflow_path":".github/workflows/js-ts-npm-package-slsa3.yml"`,
			replacement: `"workflow_path":".github/workflows/js-ts-npm-package-slsa3.txt"`,
			wantField:   "producer_profile",
		},
		{
			name:        "unknown top-level member rejected before validation",
			anchor:      `,"release_manifest":`,
			replacement: `,"unexpected":true,"release_manifest":`,
			wantField:   "policy",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			document := mutateDocument(t, validManifestExpectationDocument, test.anchor, test.replacement)
			_, err := policy.DecodeReleaseManifestExpectation([]byte(document), registry)
			requirePolicyDiagnostic(t, err, policySchemaInvalidID, test.wantField, "")
		})
	}

	t.Run("nil registry rejects any profile", func(t *testing.T) {
		t.Parallel()
		_, err := policy.DecodeReleaseManifestExpectation([]byte(validManifestExpectationDocument), nil)
		requirePolicyDiagnostic(t, err, policySchemaInvalidID, "producer_profile.profile", "")
	})
}

func mutateDocument(t *testing.T, document, anchor, replacement string) string {
	t.Helper()
	if !strings.Contains(document, anchor) {
		t.Fatalf("mutation anchor %q not present in document %q", anchor, document)
	}
	return strings.Replace(document, anchor, replacement, 1)
}

func requirePolicyDiagnostic(t *testing.T, err error, wantID, wantField, wantMessage string) {
	t.Helper()
	requirePolicyDiagnosticID(t, err, wantID)
	var validationError *policy.ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("error = %T, want *policy.ValidationError: %v", err, err)
	}
	if validationError.Diagnostic.Field != wantField {
		t.Fatalf("diagnostic field = %q, want %q: %v", validationError.Diagnostic.Field, wantField, err)
	}
	if wantMessage != "" && validationError.Diagnostic.Message != wantMessage {
		t.Fatalf("diagnostic message = %q, want %q: %v", validationError.Diagnostic.Message, wantMessage, err)
	}
}
