package fixture

import (
	"sort"
	"testing"
)

func TestValidFixturePhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		phase string
		want  bool
	}{
		{phase: "build-pack", want: true},
		{phase: "provenance-inputs", want: true},
		{phase: "", want: false},
		{phase: "signing", want: false},
		{phase: "BUILD-PACK", want: false},
	}

	for _, test := range tests {
		t.Run("phase "+test.phase, func(t *testing.T) {
			t.Parallel()

			if got := validFixturePhase(test.phase); got != test.want {
				t.Errorf("validFixturePhase(%q) = %v, want %v", test.phase, got, test.want)
			}
		})
	}
}

func TestRequirementMatchesPhaseUnknownPhase(t *testing.T) {
	t.Parallel()

	if requirementMatchesPhase("ADR-0014.package-manager-matrix", "signing") {
		t.Error("requirementMatchesPhase() = true for an unregistered phase, want false")
	}
}

func TestRequirementIDsSortedAndComplete(t *testing.T) {
	t.Parallel()

	ids := RequirementIDs()

	if !sort.StringsAreSorted(ids) {
		t.Errorf("RequirementIDs() = %v, want sorted output", ids)
	}
	if len(ids) != len(requirementRegistry) {
		t.Fatalf("len(RequirementIDs()) = %d, want %d", len(ids), len(requirementRegistry))
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, duplicate := seen[id]; duplicate {
			t.Errorf("RequirementIDs() contains duplicate %q", id)
		}
		seen[id] = struct{}{}
		if !IsRegisteredRequirement(id) {
			t.Errorf("IsRegisteredRequirement(%q) = false for a listed ID", id)
		}
	}
	for id := range requirementRegistry {
		if _, ok := seen[id]; !ok {
			t.Errorf("RequirementIDs() omits registered requirement %q", id)
		}
	}
}

func TestRequirementIDsMutationIsolation(t *testing.T) {
	t.Parallel()

	ids := RequirementIDs()
	if len(ids) == 0 {
		t.Fatal("RequirementIDs() is empty, want the registered requirement IDs")
	}
	original := ids[0]

	// Mutating the returned slice must not corrupt the registry or later calls.
	ids[0] = "ADR-9999.corrupted"

	fresh := RequirementIDs()
	if fresh[0] != original {
		t.Fatalf("RequirementIDs()[0] = %q after mutating a previous result, want %q", fresh[0], original)
	}
	if _, ok := requirementRegistry[original]; !ok {
		t.Fatalf("requirementRegistry lost %q after mutating a returned slice", original)
	}
	if IsRegisteredRequirement("ADR-9999.corrupted") {
		t.Fatal("IsRegisteredRequirement() = true for the injected mutation, want false")
	}
}
