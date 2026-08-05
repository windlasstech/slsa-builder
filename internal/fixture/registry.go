package fixture

import (
	"regexp"
	"sort"
)

var requirementIDPattern = regexp.MustCompile(`^(?:ADR-[0-9]{4}|ARCH-[a-z0-9]+(?:-[a-z0-9]+)*)\.[a-z0-9]+(?:-[a-z0-9]+)*$`)

var requirementRegistry = map[string]string{
	"ARCH-slsa-provenance-v1.subject-cardinality":                   "SLSA provenance statements contain exactly one subject",
	"ARCH-verification-policy-and-fixtures.fixture-manifest-schema": "Fixture manifests use the closed schema and consistent expectations",
}

var failureCategoryRegistry = map[string]struct{}{
	"diagnostics-contract-invalid": {},
	"duplicate-json-member":        {},
	"policy-schema-invalid":        {},
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
