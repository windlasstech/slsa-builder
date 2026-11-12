package fixture

func finalContractExpectedCases() map[string]string {
	cases := make(map[string]string)
	for _, name := range []string{
		"npm-workflow-invocation-valid", "npm-canonical-builder-identity-valid", "npm-invocation-source-valid", "npm-built-source-valid", "npm-source-selector-bindings-valid",
		"npm-internal-parameters-github-valid", "npm-distribution-interface-valid", "npm-external-parameters-minimized-valid",
		"npm-package-manifest-descriptor-valid", "npm-package-manifest-evidence-valid",
		"npm-build-script-disposition-executed-valid", "npm-build-script-disposition-skipped-absent-valid",
		"npm-builder-version-direct-npm-valid", "npm-builder-version-corepack-valid", "npm-publish-resolution-recompute-valid",
		"diagnostic-publish-preexisting-valid", "npm-media-type-matrix-valid",
		"npm-runner-image-reported-identity-valid", "npm-runner-image-metadata-valid", "npm-runner-image-imageos-absent-valid",
	} {
		cases[name] = ""
	}
	for category, names := range map[string][]string{
		"build-config-uri-mismatch":                          {"npm-build-config-uri-mismatch"},
		"wrong-builder-id":                                   {"npm-canonical-builder-id-rejected"},
		"resolved-dependencies-invocation-source":            {"npm-invocation-source-missing", "npm-invocation-source-duplicate", "npm-invocation-source-commit-mismatch", "npm-invocation-source-media-type-rejected"},
		"resolved-dependencies-built-source":                 {"npm-built-source-missing", "npm-built-source-commit-mismatch", "npm-built-source-classifier-mismatch", "npm-built-source-media-type-rejected"},
		"source-ref-invalid":                                 {"npm-source-selector-conflict", "npm-source-selector-whitespace"},
		"build-trigger-mismatch":                             {"npm-build-trigger-mismatch"},
		"unexpected-internal-parameters":                     {"npm-github-internal-parameters-actor-rejected"},
		"unexpected-external-parameters":                     {"npm-distribution-removed-member-rejected", "npm-external-parameters-removed-members-rejected"},
		"resolved-dependencies-package-manifest":             {"npm-package-manifest-descriptor-missing", "npm-package-manifest-path-mismatch", "npm-manifest-media-type-rejected"},
		"package-manifest-evidence":                          {"npm-package-manifest-evidence-missing", "npm-package-manifest-evidence-noncanonical", "npm-package-manifest-evidence-digest-mismatch", "npm-package-manifest-evidence-path-mismatch", "npm-package-manifest-evidence-duplicate-member"},
		"build-script-disposition-mismatch":                  {"npm-build-script-disposition-missing", "npm-build-script-disposition-inconsistent"},
		"builder-version-mismatch":                           {"npm-builder-version-npm-missing"},
		"resolved-dependencies-lockfile":                     {"npm-lockfile-media-type-rejected"},
		"resolved-dependencies-node-runtime":                 {"npm-runtime-media-type-rejected"},
		"resolved-dependencies-package-manager-distribution": {"npm-distribution-media-type-rejected"},
		"resolved-dependencies-runner-image":                 {"npm-runner-image-image-os-rejected", "npm-runner-image-operating-system-annotation-missing", "npm-runner-image-additional-annotation-rejected", "npm-runner-image-projection-mismatch"},
		"input-unavailable": {
			"npm-runner-image-operating-system-group-missing", "npm-runner-image-operating-system-group-duplicate", "npm-runner-image-operating-system-group-malformed",
			"npm-runner-image-operating-system-detail-missing", "npm-runner-image-operating-system-detail-duplicate", "npm-runner-image-operating-system-detail-non-string",
			"npm-runner-image-operating-system-detail-empty", "npm-runner-image-operating-system-detail-whitespace",
		},
	} {
		for _, name := range names {
			cases[name] = "windlass.verify.error." + category
		}
	}
	return cases
}
