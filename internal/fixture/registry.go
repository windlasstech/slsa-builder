package fixture

import (
	"regexp"
	"sort"
)

var requirementIDPattern = regexp.MustCompile(`^(?:ADR-[0-9]{4}|ARCH-[a-z0-9]+(?:-[a-z0-9]+)*)\.[a-z0-9]+(?:-[a-z0-9]+)*$`)

var requirementRegistry = map[string]string{
	"ADR-0103.workflow-invocation-identity":                         "caller workflow ref, repository, and path identify the invocation",
	"ADR-0104.canonical-builder-identity":                           "builder identity is recorded only in runDetails.builder.id",
	"ADR-0105.invocation-source-descriptor":                         "invocation-source binds the workflow origin and occupies producer index zero",
	"ADR-0105.built-source-descriptor":                              "built-source records the independently resolved release commit",
	"ADR-0106.source-selector-bindings":                             "optional source selector derives the effective built ref without invocation fallback",
	"ADR-0107.github-internal-parameters":                           "closed GitHub event and numeric identities bind to invocation claims",
	"ADR-0108.distribution-interface":                               "distribution inputs contain only release mode and linked metadata booleans",
	"ADR-0109.external-parameters-interface":                        "external parameters contain only actual interface inputs",
	"ADR-0109.package-manifest-descriptor":                          "original package manifest is a source-bound dependency",
	"ADR-0109.package-manifest-evidence":                            "bounded offline original manifest bytes match the signed descriptor",
	"ADR-0109.build-script-disposition":                             "signed build disposition agrees with the original manifest",
	"ADR-0109.builder-version":                                      "builder versions record nodejs, bundled npm, and conditional corepack",
	"ADR-0109.publish-resolution":                                   "publish settings are recomputed from raw inputs and authenticated manifest",
	"ADR-0109.preexisting-diagnostics":                              "registry pre-existence observations are unsigned diagnostics only",
	"ADR-0110.dependency-media-types":                               "media types describe the actual digest-covered representation",
	"ADR-0111.runner-reported-identity":                             "runner URI and full version retain platform-reported identity",
	"ADR-0112.runner-operating-system":                              "runner OS detail comes from the bounded on-disk snapshot",
	"ADR-0014.package-manager-matrix":                               "npm, pnpm, and Yarn build-pack fixtures",
	"ADR-0017.exact-package-manager-version":                        "pnpm and Yarn require exact release versions",
	"ADR-0018.single-workspace-package":                             "one explicitly selected workspace package per run",
	"ADR-0019.package-publish-metadata":                             "package identity and private publish intent validation",
	"ADR-0078.pnpm-settings-only-root-package":                      "settings-only pnpm metadata selects only the standalone root package",
	"ADR-0082.publish-npm-purl-compat":                              "scoped and unscoped npm PURLs for each publish-stage npm allowlist version",
	"ADR-0085.build-npm-purl-compat":                                "scoped and unscoped npm PURLs for the pinned build-stage npm pair",
	"ADR-0079.source-ref-dispatch-retry":                            "Tags-only source-ref selects the built release tag while preserving invocation context.",
	"ADR-0079.source-ref-rejections":                                "invalid, unresolvable, version-mismatched, or invocation-conflicting source-ref values fail closed",
	"ADR-0080.invocation-record-binding":                            "Certificate source claims bind the signed invocation record while policy binds signed built-source fields.",
	"ADR-0056.stale-lockfile-diagnostics":                           "non-selected lockfiles are recorded stale diagnostics",
	"ADR-0056.stale-lockfile-rejections":                            "ambiguous and missing selected lockfiles are rejected",
	"ADR-0063.yarn-v4-selection":                                    "Yarn requires top-level exact Berry v4 packageManager metadata",
	"ADR-0088.corepack-window-version-bounds":                       "Corepack-window pnpm 10.x/11.x and Yarn v4/v5 version bounds",
	"ADR-0090.pnpm-10x-support":                                     "pnpm 10.x pins are accepted from both manifest selection sources",
	"ADR-0091.uniform-selection-source":                             "any supported package manager is selectable from devEngines.packageManager",
	"ADR-0092.descriptor-integrity-digests":                         "pnpm and Yarn descriptors may declare an integrity digest reconciled fail-closed",
	"ADR-0092.descriptor-digest-rejections":                         "malformed descriptor digest suffixes and npm digest suffixes are rejected",
	"ADR-0095.npm-declared-version":                                 "npm version declarations are non-authoritative with a mismatch warning",
	"ADR-0096.corepack-env-neutralization":                          "a project .corepack.env is never loaded under the COREPACK_ENV_FILE=0 pin, and its presence warns",
	"ADR-0099.node-runtime-descriptor":                              "the digest-verified Node.js runtime is recorded as an exact closed resolvedDependencies descriptor",
	"ADR-0099.node-runtime-rejections":                              "missing, duplicate, malformed, forbidden, and unknown resolvedDependencies entries fail with narrow diagnostics",
	"ADR-0064.npm-purl-subject-digests":                             "npm PURL subjects require SHA-512 and SHA-256",
	"ARCH-js-ts-npm-provenance-publish.signing-inputs":              "npm provenance signing inputs use the closed profile predicate contract",
	"ARCH-slsa-provenance-v1.subject-cardinality":                   "SLSA provenance statements contain exactly one subject",
	"ARCH-verification-policy-and-fixtures.fixture-manifest-schema": "Fixture manifests use the closed schema and consistent expectations",
}

var failureCategoryRegistry = map[string]struct{}{
	"wrong-builder-id":                        {},
	"build-config-uri-mismatch":               {},
	"build-trigger-mismatch":                  {},
	"build-script-disposition-mismatch":       {},
	"resolved-dependencies-invocation-source": {},
	"resolved-dependencies-built-source":      {},
	"resolved-dependencies-package-manifest":  {},
	"package-manifest-evidence":               {},
	"builder-version-mismatch":                {},
	"builder-id-not-immutable":                {},
	"unexpected-internal-parameters":          {},
	"resolved-dependencies-lockfile":          {},
	"source-numeric-id-mismatch":              {},
	"diagnostics-contract-invalid":            {},
	"duplicate-json-member":                   {},
	"package-manager-conflict":                {},
	"package-manager-digest-malformed":        {},
	"package-manager-digest-mismatch":         {},
	"package-manager-version-required":        {},
	"package-metadata-required":               {},
	"package-private":                         {},
	"package-resolution-invalid":              {},
	"policy-schema-invalid":                   {},
	"pnpm-version-unsupported":                {},
	"required-lockfile-missing":               {},

	"resolved-dependencies-node-runtime":                 {},
	"resolved-dependencies-package-manager-distribution": {},
	"resolved-dependencies-unexpected-entry":             {},

	"source-digest-mismatch":         {},
	"source-ref-mismatch":            {},
	"source-ref-invalid":             {},
	"unexpected-external-parameters": {},
	"yarn-selection-invalid":         {},
	"yarn-version-unsupported":       {},
}

var fixturePhaseRequirements = map[string]map[string]struct{}{
	"build-pack": {
		"ADR-0014.package-manager-matrix":          {},
		"ADR-0017.exact-package-manager-version":   {},
		"ADR-0018.single-workspace-package":        {},
		"ADR-0019.package-publish-metadata":        {},
		"ADR-0056.stale-lockfile-diagnostics":      {},
		"ADR-0056.stale-lockfile-rejections":       {},
		"ADR-0063.yarn-v4-selection":               {},
		"ADR-0064.npm-purl-subject-digests":        {},
		"ADR-0078.pnpm-settings-only-root-package": {},
		"ADR-0082.publish-npm-purl-compat":         {},
		"ADR-0085.build-npm-purl-compat":           {},
		"ADR-0088.corepack-window-version-bounds":  {},
		"ADR-0090.pnpm-10x-support":                {},
		"ADR-0091.uniform-selection-source":        {},
		"ADR-0092.descriptor-integrity-digests":    {},
		"ADR-0092.descriptor-digest-rejections":    {},
		"ADR-0095.npm-declared-version":            {},
		"ADR-0096.corepack-env-neutralization":     {},
	},
	"provenance-inputs": {
		"ADR-0103.workflow-invocation-identity":            {},
		"ADR-0104.canonical-builder-identity":              {},
		"ADR-0105.invocation-source-descriptor":            {},
		"ADR-0105.built-source-descriptor":                 {},
		"ADR-0106.source-selector-bindings":                {},
		"ADR-0107.github-internal-parameters":              {},
		"ADR-0108.distribution-interface":                  {},
		"ADR-0109.external-parameters-interface":           {},
		"ADR-0109.package-manifest-descriptor":             {},
		"ADR-0109.package-manifest-evidence":               {},
		"ADR-0109.build-script-disposition":                {},
		"ADR-0109.builder-version":                         {},
		"ADR-0109.publish-resolution":                      {},
		"ADR-0109.preexisting-diagnostics":                 {},
		"ADR-0110.dependency-media-types":                  {},
		"ADR-0111.runner-reported-identity":                {},
		"ADR-0112.runner-operating-system":                 {},
		"ARCH-js-ts-npm-provenance-publish.signing-inputs": {},
		"ADR-0079.source-ref-dispatch-retry":               {},
		"ADR-0079.source-ref-rejections":                   {},
		"ADR-0080.invocation-record-binding":               {},
		"ADR-0099.node-runtime-descriptor":                 {},
		"ADR-0099.node-runtime-rejections":                 {},
	},
}

func validFixturePhase(phase string) bool {
	_, ok := fixturePhaseRequirements[phase]
	return ok
}

func requirementMatchesPhase(requirement, phase string) bool {
	requirements, ok := fixturePhaseRequirements[phase]
	if !ok {
		return false
	}
	_, ok = requirements[requirement]
	return ok
}

// RequirementIDs returns the stable requirement IDs known to the fixture harness.
func RequirementIDs() []string {
	ids := make([]string, 0, len(requirementRegistry))
	for id := range requirementRegistry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// IsRegisteredRequirement reports whether id has valid syntax and a registry mapping.
func IsRegisteredRequirement(id string) bool {
	if !requirementIDPattern.MatchString(id) {
		return false
	}
	_, ok := requirementRegistry[id]
	return ok
}
