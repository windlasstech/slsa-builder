package npmprofile

type contractFixtureSpec struct {
	name, requirement, category, mutation string
	manager                               Manager
	scenario                              string
	producer                              bool
}

var finalContractFixtures = []contractFixtureSpec{
	{"npm-workflow-invocation-valid", "ADR-0103.workflow-invocation-identity", "", "", ManagerPNPM, "retry", false},
	{"npm-canonical-builder-identity-valid", "ADR-0104.canonical-builder-identity", "", "", ManagerNPM, "tag", false},
	{"npm-invocation-source-valid", "ADR-0105.invocation-source-descriptor", "", "", ManagerPNPM, "retry", true},
	{"npm-built-source-valid", "ADR-0105.built-source-descriptor", "", "", ManagerPNPM, "retry", true},
	{"npm-source-selector-bindings-valid", "ADR-0106.source-selector-bindings", "", "", ManagerPNPM, "retry", true},
	{"npm-internal-parameters-github-valid", "ADR-0107.github-internal-parameters", "", "", ManagerNPM, "tag", false},
	{"npm-distribution-interface-valid", "ADR-0108.distribution-interface", "", "", ManagerNPM, "tag", false},
	{"npm-external-parameters-minimized-valid", "ADR-0109.external-parameters-interface", "", "", ManagerNPM, "tag", false},
	{"npm-package-manifest-descriptor-valid", "ADR-0109.package-manifest-descriptor", "", "", ManagerNPM, "workspace", true},
	{"npm-package-manifest-evidence-valid", "ADR-0109.package-manifest-evidence", "", "", ManagerNPM, "tag", true},
	{"npm-build-script-disposition-executed-valid", "ADR-0109.build-script-disposition", "", "", ManagerNPM, "tag", false},
	{"npm-build-script-disposition-skipped-absent-valid", "ADR-0109.build-script-disposition", "", "", ManagerNPM, "skipped", false},
	{"npm-builder-version-direct-npm-valid", "ADR-0109.builder-version", "", "", ManagerNPM, "tag", false},
	{"npm-builder-version-corepack-valid", "ADR-0109.builder-version", "", "", ManagerPNPM, "tag", false},
	{"npm-publish-resolution-recompute-valid", "ADR-0109.publish-resolution", "", "", ManagerNPM, "publish", false},
	{"diagnostic-publish-preexisting-valid", "ADR-0109.preexisting-diagnostics", "", "diagnostics", ManagerNPM, "tag", false},
	{"npm-media-type-matrix-valid", "ADR-0110.dependency-media-types", "", "matrix", ManagerNPM, "tag", true},
	{"npm-build-config-uri-mismatch", "ADR-0103.workflow-invocation-identity", "build-config-uri-mismatch", "caller-path", ManagerNPM, "tag", false},
	{"npm-canonical-builder-id-rejected", "ADR-0104.canonical-builder-identity", "wrong-builder-id", "builder-id", ManagerNPM, "tag", false},
	{"npm-invocation-source-missing", "ADR-0105.invocation-source-descriptor", "resolved-dependencies-invocation-source", "missing:invocation-source", ManagerNPM, "tag", true},
	{"npm-invocation-source-duplicate", "ADR-0105.invocation-source-descriptor", "resolved-dependencies-invocation-source", "duplicate:invocation-source", ManagerNPM, "tag", true},
	{"npm-invocation-source-commit-mismatch", "ADR-0105.invocation-source-descriptor", "resolved-dependencies-invocation-source", "commit:invocation-source", ManagerPNPM, "retry", true},
	{"npm-built-source-missing", "ADR-0105.built-source-descriptor", "resolved-dependencies-built-source", "missing:built-source", ManagerNPM, "tag", true},
	{"npm-built-source-commit-mismatch", "ADR-0105.built-source-descriptor", "resolved-dependencies-built-source", "commit:built-source", ManagerNPM, "tag", true},
	{"npm-built-source-classifier-mismatch", "ADR-0106.source-selector-bindings", "resolved-dependencies-built-source", "classifier", ManagerNPM, "tag", true},
	{"npm-source-selector-conflict", "ADR-0106.source-selector-bindings", "source-ref-invalid", "selector-conflict", ManagerNPM, "tag", false},
	{"npm-source-selector-whitespace", "ADR-0106.source-selector-bindings", "source-ref-invalid", "selector-whitespace", ManagerNPM, "tag", false},
	{"npm-build-trigger-mismatch", "ADR-0107.github-internal-parameters", "build-trigger-mismatch", "event", ManagerNPM, "tag", false},
	{"npm-github-internal-parameters-actor-rejected", "ADR-0107.github-internal-parameters", "unexpected-internal-parameters", "actor", ManagerNPM, "tag", false},
	{"npm-distribution-removed-member-rejected", "ADR-0108.distribution-interface", "unexpected-external-parameters", "sidecar", ManagerNPM, "tag", false},
	{"npm-external-parameters-removed-members-rejected", "ADR-0109.external-parameters-interface", "unexpected-external-parameters", "removed", ManagerNPM, "tag", false},
	{"npm-package-manifest-descriptor-missing", "ADR-0109.package-manifest-descriptor", "resolved-dependencies-package-manifest", "missing:package-manifest", ManagerNPM, "tag", true},
	{"npm-package-manifest-path-mismatch", "ADR-0109.package-manifest-descriptor", "resolved-dependencies-package-manifest", "manifest-path", ManagerNPM, "tag", true},
	{"npm-package-manifest-evidence-missing", "ADR-0109.package-manifest-evidence", "package-manifest-evidence", "evidence-missing", ManagerNPM, "tag", false},
	{"npm-package-manifest-evidence-noncanonical", "ADR-0109.package-manifest-evidence", "package-manifest-evidence", "evidence-base64", ManagerNPM, "tag", false},
	{"npm-package-manifest-evidence-digest-mismatch", "ADR-0109.package-manifest-evidence", "package-manifest-evidence", "evidence-digest", ManagerNPM, "tag", false},
	{"npm-package-manifest-evidence-path-mismatch", "ADR-0109.package-manifest-evidence", "package-manifest-evidence", "evidence-path", ManagerNPM, "tag", false},
	{"npm-package-manifest-evidence-duplicate-member", "ADR-0109.package-manifest-evidence", "package-manifest-evidence", "evidence-duplicate", ManagerNPM, "tag", false},
	{"npm-build-script-disposition-missing", "ADR-0109.build-script-disposition", "build-script-disposition-mismatch", "disposition-missing", ManagerNPM, "tag", false},
	{"npm-build-script-disposition-inconsistent", "ADR-0109.build-script-disposition", "build-script-disposition-mismatch", "disposition-mismatch", ManagerNPM, "tag", false},
	{"npm-builder-version-npm-missing", "ADR-0109.builder-version", "builder-version-mismatch", "npm-missing", ManagerNPM, "tag", false},
	{"npm-invocation-source-media-type-rejected", "ADR-0110.dependency-media-types", "resolved-dependencies-invocation-source", "media:invocation-source", ManagerNPM, "tag", true},
	{"npm-built-source-media-type-rejected", "ADR-0110.dependency-media-types", "resolved-dependencies-built-source", "media:built-source", ManagerNPM, "tag", true},
	{"npm-manifest-media-type-rejected", "ADR-0110.dependency-media-types", "resolved-dependencies-package-manifest", "media:package-manifest", ManagerNPM, "tag", true},
	{"npm-lockfile-media-type-rejected", "ADR-0110.dependency-media-types", "resolved-dependencies-lockfile", "media:lockfile", ManagerNPM, "tag", true},
	{"npm-runtime-media-type-rejected", "ADR-0110.dependency-media-types", "resolved-dependencies-node-runtime", "media:node-runtime", ManagerNPM, "tag", true},
	{"npm-distribution-media-type-rejected", "ADR-0110.dependency-media-types", "resolved-dependencies-package-manager-distribution", "media:package-manager-distribution", ManagerYarn, "tag", true},
}
