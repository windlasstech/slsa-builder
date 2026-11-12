package fixture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalContractRequirementRegistry(t *testing.T) {
	t.Parallel()
	spec, err := os.ReadFile(filepath.Join("..", "..", "docs", "architecture", "verification-policy-and-fixtures.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{
		"ADR-0103.workflow-invocation-identity", "ADR-0104.canonical-builder-identity",
		"ADR-0105.invocation-source-descriptor", "ADR-0105.built-source-descriptor",
		"ADR-0106.source-selector-bindings", "ADR-0107.github-internal-parameters",
		"ADR-0108.distribution-interface", "ADR-0109.external-parameters-interface",
		"ADR-0109.package-manifest-descriptor", "ADR-0109.package-manifest-evidence",
		"ADR-0109.build-script-disposition", "ADR-0109.builder-version",
		"ADR-0109.publish-resolution", "ADR-0109.preexisting-diagnostics",
		"ADR-0110.dependency-media-types", "ADR-0111.runner-reported-identity", "ADR-0112.runner-operating-system",
	} {
		if !IsRegisteredRequirement(id) || !requirementMatchesPhase(id, "provenance-inputs") {
			t.Errorf("unregistered provenance requirement %q", id)
		}
		if !strings.Contains(string(spec), "`"+id+"`") {
			t.Errorf("requirement %q is absent from the specification mapping", id)
		}
	}
	for _, category := range []string{"build-config-uri-mismatch", "build-trigger-mismatch", "resolved-dependencies-invocation-source", "resolved-dependencies-built-source", "resolved-dependencies-package-manifest", "package-manifest-evidence", "build-script-disposition-mismatch", "resolved-dependencies-runner-image", "input-unavailable"} {
		if _, ok := failureCategoryRegistry[category]; !ok {
			t.Errorf("unregistered failure category %q", category)
		}
	}
}
