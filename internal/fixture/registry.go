package fixture

import (
	"regexp"
	"sort"
)

var requirementIDPattern = regexp.MustCompile(`^(?:ADR-[0-9]{4}|ARCH-[a-z0-9]+(?:-[a-z0-9]+)*)\.[a-z0-9]+(?:-[a-z0-9]+)*$`)

var requirementRegistry = map[string]string{
	"ADR-0014.package-manager-matrix":                               "npm, pnpm, and Yarn build-pack fixtures",
	"ADR-0017.exact-package-manager-version":                        "pnpm and Yarn require exact release versions",
	"ADR-0018.single-workspace-package":                             "one explicitly selected workspace package per run",
	"ADR-0019.package-publish-metadata":                             "package identity and private publish intent validation",
	"ADR-0078.pnpm-settings-only-root-package":                      "settings-only pnpm metadata selects only the standalone root package",
	"ADR-0079.source-ref-rejections":                                "invalid, unresolvable, version-mismatched, or invocation-conflicting source-ref values fail closed",
	"ADR-0056.stale-lockfile-diagnostics":                           "non-selected lockfiles are recorded stale diagnostics",
	"ADR-0056.stale-lockfile-rejections":                            "ambiguous and missing selected lockfiles are rejected",
	"ADR-0063.yarn-v4-selection":                                    "Yarn requires top-level exact Berry v4 packageManager metadata",
	"ADR-0064.npm-purl-subject-digests":                             "npm PURL subjects require SHA-512 and SHA-256",
	"ARCH-js-ts-npm-provenance-publish.signing-inputs":              "npm provenance signing inputs use the closed profile predicate contract",
	"ARCH-slsa-provenance-v1.subject-cardinality":                   "SLSA provenance statements contain exactly one subject",
	"ARCH-verification-policy-and-fixtures.fixture-manifest-schema": "Fixture manifests use the closed schema and consistent expectations",
}

var failureCategoryRegistry = map[string]struct{}{
	"diagnostics-contract-invalid":     {},
	"duplicate-json-member":            {},
	"package-manager-conflict":         {},
	"package-manager-version-required": {},
	"package-metadata-required":        {},
	"package-private":                  {},
	"package-resolution-invalid":       {},
	"policy-schema-invalid":            {},
	"required-lockfile-missing":        {},
	"source-ref-invalid":               {},
	"yarn-selection-invalid":           {},
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
	},
	"provenance-inputs": {
		"ARCH-js-ts-npm-provenance-publish.signing-inputs": {},
		"ADR-0079.source-ref-rejections":                   {},
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
