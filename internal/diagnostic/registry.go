package diagnostic

import "strings"

var registry = buildRegistry()

func buildRegistry() map[string]Definition {
	definitions := make(map[string]Definition, len(errorCategories)+len(warningCategories))
	for _, category := range errorCategories {
		register(definitions, Definition{
			ID:         "windlass.verify.error." + category,
			Severity:   SeverityError,
			Category:   category,
			Phase:      PhaseVerification,
			ExitCode:   ExitCodePolicyFailure,
			precedence: 7,
		})
	}
	for _, category := range warningCategories {
		register(definitions, Definition{
			ID:         "windlass.verify.warning." + category,
			Severity:   SeverityWarning,
			Category:   category,
			Phase:      PhaseVerification,
			ExitCode:   ExitCodePass,
			precedence: 8,
		})
	}

	set(definitions, PhaseInvocation, ExitCodeInvocationFailure, false, 1,
		"verification-mode-invalid", "input-unavailable", "verifier-execution-failure")
	set(definitions, PhasePolicy, ExitCodePolicyFailure, false, 1,
		"policy-schema-invalid", "duplicate-json-member", "legacy-trust-root-override",
		// The npm producer failure matrix explicitly overrides the shared taxonomy default:
		// docs/architecture/js-ts-npm-build-pack.md:694-701,705.
		"package-manifest-invalid", "package-metadata-required", "package-private",
		"package-resolution-invalid", "package-manager-conflict", "package-manager-version-required",
		"yarn-selection-invalid", "required-lockfile-missing", "package-repository-identity-mismatch")

	setPrecedence(definitions, 1,
		"diagnostics-contract-invalid", "digest-encoding-invalid", "digest-mismatch",
		"handoff-schema-mismatch", "resolved-dependencies-lockfile", "subject-sha256-missing",
		"missing-subject-sha256", "missing-subject-sha512")
	setPrecedence(definitions, 2,
		"ungoverned-trust-root", "stale-pinned-trust-root", "verification-network-call")
	setPrecedence(definitions, 3,
		"signature-invalid", "signature-mismatch", "missing-sct", "missing-rekor-entry",
		"signature-time-violation")
	setPrecedence(definitions, 4,
		"issuer-mismatch", "signer-mismatch", "signer-identity-claim-missing",
		"signer-identity-untrusted", "signer-workflow-path-mismatch", "signer-workflow-sha-mismatch",
		"source-identity-mismatch", "source-numeric-id-mismatch", "source-digest-mismatch",
		"source-ref-mismatch", "source-repository-canonicalization-error", "run-invocation-uri-invalid",
		"self-hosted-runner", "wrong-producer-signer")
	setPrecedence(definitions, 5, "trusted-producer-policy-conflict")
	setPrecedence(definitions, 6,
		"statement-type-invalid", "predicate-type-invalid", "wrong-predicate-type",
		"wrong-manifest-predicate-type", "builder-id-not-immutable", "wrong-builder-id",
		"build-type-not-canonical", "wrong-build-type", "external-parameters-incomplete",
		"external-parameters-unexpected", "unexpected-external-parameters",
		"unexpected-internal-parameters", "subject-cardinality-error", "subject-cardinality-invalid",
		"subject-name-mismatch", "subject-digest-scope-invalid", "statement-assembly-mismatch")

	set(definitions, PhasePreMutation, ExitCodePolicyFailure, false, 7,
		"release-target-immutable", "handoff-schema-mismatch", "publisher-remote-digest-unproven",
		"npm-oidc-exchange-indeterminate", "oidc-capability-unavailable", "source-ref-invalid")
	set(definitions, PhaseMutation, ExitCodePolicyFailure, false, 7, "mutation-permission-denied")
	set(definitions, PhaseMutation, ExitCodePolicyFailure, true, 7,
		"publisher-indeterminate-primary-upload", "mutation-queue-overflow",
		"manifest-partial-json-uploaded", "manifest-indeterminate-json-upload",
		"manifest-remote-digest-unproven", "sidecar-upload-partial-failure",
		"duplicate-release-asset", "duplicate-sidecar-asset", "registry-linkage-mismatch",
		"custom-registry-tokenless-auth-failed", "custom-registry-provenance-submission-rejected",
		"custom-registry-linkage-metadata-absent", "custom-registry-digest-semantics-mismatch")

	return definitions
}

func register(definitions map[string]Definition, definition Definition) {
	if _, exists := definitions[definition.ID]; exists {
		panic("duplicate diagnostic registry entry: " + definition.ID)
	}
	definitions[definition.ID] = definition
}

func set(definitions map[string]Definition, phase Phase, exitCode int, mutationPossible bool, precedence int, categories ...string) {
	for _, category := range categories {
		id := "windlass.verify.error." + category
		definition, ok := definitions[id]
		if !ok {
			panic("diagnostic override is not registered: " + id)
		}
		definition.Phase = phase
		definition.ExitCode = exitCode
		definition.MutationPossible = mutationPossible
		definition.precedence = precedence
		definitions[id] = definition
	}
}

func setPrecedence(definitions map[string]Definition, precedence int, categories ...string) {
	for _, category := range categories {
		id := "windlass.verify.error." + category
		definition, ok := definitions[id]
		if !ok {
			panic("diagnostic precedence entry is not registered: " + id)
		}
		definition.precedence = precedence
		definitions[id] = definition
	}
}

var warningCategories = []string{
	"custom-registry-preflight-inconclusive",
	"native-provenance-locator-missing",
	"stale-non-selected-lockfile",
	"timestamp-clock-skew",
}

var errorCategories = strings.Fields(`
actions-attest-adapter-contract
already-published-version
build-type-not-canonical
builder-dependencies-signing-adapter-mismatch
builder-id-not-immutable
builder-version-mismatch
bundle-byte-format-mismatch
composition-handoff-substitution
custom-registry-access-option-rejected
custom-registry-digest-semantics-mismatch
custom-registry-linkage-metadata-absent
custom-registry-provenance-submission-rejected
custom-registry-provenance-weakened
custom-registry-token-required
custom-registry-tokenless-auth-failed
diagnostics-contract-invalid
digest-encoding-invalid
digest-mismatch
duplicate-json-member
duplicate-release-asset
duplicate-sidecar-asset
empty-publish-input-fallback
excessive-publish-permission
external-parameters-incomplete
external-parameters-unexpected
handoff-schema-mismatch
input-unavailable
invalid-publish-input
issuer-mismatch
legacy-trust-root-override
linked-artifact-locator-mismatch
linked-artifact-settings-mismatch
manifest-caller-override
manifest-digest-mismatch
manifest-entry-order-mismatch
manifest-entrypoint-mismatch
manifest-generated-at-invalid
manifest-handoff-basename-mismatch
manifest-indeterminate-json-upload
manifest-partial-json-uploaded
manifest-predicate-mismatch
manifest-remote-digest-unproven
manifest-signing-input-mismatch
manifest-tag-peel-mismatch
manifest-trigger-mismatch
manifest-workflow-sha-mismatch
missing-producer-provenance
missing-rekor-entry
missing-sct
missing-subject-sha256
missing-subject-sha512
mutation-permission-denied
mutation-queue-overflow
native-locator-digest-mismatch
native-locator-malformed
npm-oidc-exchange-indeterminate
npm-purl-subject-mismatch
npm-version-too-old
oidc-capability-unavailable
package-directory-mismatch
package-identity-mismatch
package-manager-conflict
package-manager-manifest-shape-error
package-manager-selection-path-mismatch
package-manager-version-required
package-manifest-invalid
package-metadata-required
package-pack-failed
package-private
package-repository-identity-mismatch
package-resolution-invalid
package-url-mismatch
package-version-mismatch
packed-package-metadata-mismatch
policy-schema-invalid
predicate-type-invalid
prepublish-registry-metadata-required
private-package
publish-intent-conflict
publisher-handoff-field-error
publisher-indeterminate-primary-upload
publisher-permission-boundary-violation
publisher-remote-digest-unproven
publisher-workflow-schema-error
raw-artifact-bypass
registry-linkage-mismatch
release-asset-binding-mismatch
release-asset-mode-disabled-conflict
release-asset-mode-permission-error
release-asset-mode-schema-error
release-asset-target-error
release-manifest-mismatch
release-ref-mismatch
release-target-immutable
release-version-semver-mismatch
required-lockfile-missing
resolved-dependencies-lockfile
resolved-dependencies-package-manager-distribution
resolved-dependencies-runner-image
resolved-dependencies-unexpected-entry
run-invocation-uri-invalid
runtime-policy-mismatch
self-hosted-runner
sidecar-digest-mismatch
sidecar-mismatch
sidecar-upload-partial-failure
signature-invalid
signature-mismatch
signature-time-violation
signer-identity-claim-missing
signer-identity-untrusted
signer-mismatch
signer-workflow-path-mismatch
signer-workflow-sha-mismatch
source-digest-mismatch
source-identity-mismatch
source-numeric-id-mismatch
source-ref-invalid
source-ref-mismatch
source-repository-canonicalization-error
stale-pinned-trust-root
statement-assembly-mismatch
statement-type-invalid
subject-cardinality-error
subject-cardinality-invalid
subject-digest-scope-invalid
subject-name-mismatch
subject-sha256-missing
tarball-filename-subject-rejected
timestamp-format-invalid
timestamp-ordering-invalid
trusted-core-boundary-violation
trusted-producer-policy-conflict
trusted-publisher-mismatch
unexpected-external-parameters
unexpected-internal-parameters
ungoverned-trust-root
unregistered-producer-build-type
unsupported-initial-publication
unsupported-yarn-version
verification-mode-invalid
verification-network-call
verifier-execution-failure
workspace-command-mismatch
workspace-pattern-base-mismatch
workspace-resolution-mismatch
wrong-build-type
wrong-builder-id
wrong-manifest-predicate-type
wrong-predicate-type
wrong-producer-signer
yarn-selection-invalid
`)
