package npmprofile

import (
	"bytes"
	"os"
	"regexp"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

func TestBuilderContractMatrixCoverage(t *testing.T) {
	t.Parallel()
	// Given the normative matrix's machine-readable fixture names.
	raw, err := os.ReadFile("../../docs/architecture/verification-policy-and-fixtures.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := bytes.Cut(raw, []byte("### Builder dependency semantic fixtures"))
	if !ok {
		t.Fatal("missing builder matrix")
	}
	section, _, _ = bytes.Cut(section, []byte("\n### "))
	rows := regexp.MustCompile("\\| `(npm-builder-[^`]+)`").FindAllSubmatch(section, -1)
	if len(rows) != 27 {
		t.Fatalf("matrix rows = %d, want 27", len(rows))
	}
	// When matching the catalog to the matrix.
	catalog := map[string]bool{}
	for _, spec := range finalContractFixtures {
		if spec.requirement == "ADR-0113.builder-dependencies-common-shape" || spec.requirement == "ADR-0114.publish-cli-descriptor" {
			if catalog[spec.name] {
				t.Fatalf("duplicate catalog entry %s", spec.name)
			}
			catalog[spec.name] = true
		}
	}
	// Then every behavior has exactly one catalog entry.
	for _, row := range rows {
		name := string(row[1])
		if !catalog[name] {
			t.Errorf("missing catalog entry %s", name)
		}
		delete(catalog, name)
	}
	if len(catalog) != 0 {
		t.Fatalf("builder entries outside matrix: %v", catalog)
	}
}

func TestBuilderContractVariantsChangeInput(t *testing.T) {
	t.Parallel()
	for _, spec := range finalContractFixtures {
		if !bytes.HasPrefix([]byte(spec.name), []byte("npm-builder-")) || spec.mutation == "" {
			continue
		}
		t.Run(spec.name, func(t *testing.T) {
			t.Parallel()
			// Given the unmodified producer output.
			baseline := contractVariant(t, spec.manager, spec.scenario).Statement
			// When generating the mutation family.
			variants := contractVariants(t, spec)
			seen := map[string]bool{}
			for _, variant := range variants {
				// Then every variant changes the input, with no duplicate cases.
				raw := variant.Statement
				if spec.mutation == "builder-duplicate-json" {
					raw = []byte(variant.StatementJSON)
				} else {
					canonical, err := canonicaljson.Canonicalize(raw)
					if err != nil {
						t.Fatal(err)
					}
					raw = canonical
				}
				if seen[string(raw)] {
					t.Fatalf("duplicate variant %s", variant.Name)
				}
				seen[string(raw)] = true
				if bytes.Equal(raw, baseline) {
					t.Fatalf("no-op variant %s", variant.Name)
				}
				err := runContractVariant(t, spec, variant)
				if spec.category == "" {
					if err != nil {
						t.Fatalf("%s: %v", variant.Name, err)
					}
				} else {
					requireNPMDiagnostic(t, err, "windlass.verify.error."+spec.category)
				}
			}
		})
	}
}
