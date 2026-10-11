package fixture

import (
	"strings"
	"testing"
)

func selectTestManifest(name, surface, requirement string) Manifest {
	return Manifest{
		Name:                 name,
		Type:                 "accepted",
		Surface:              surface,
		Artifact:             "testdata/" + surface + "/" + name + "/artifact.tgz",
		Provenance:           "testdata/" + surface + "/" + name + "/provenance.json",
		ExpectedResult:       "pass",
		ExpectedSecondaryIDs: []string{},
		CoveredRequirement:   requirement,
	}
}

func selectTestIndex() Index {
	return Index{Fixtures: []Manifest{
		selectTestManifest("npm-build", "npm", "ADR-0014.package-manager-matrix"),
		selectTestManifest("npm-provenance", "npm", "ARCH-js-ts-npm-provenance-publish.signing-inputs"),
		selectTestManifest("publisher-build", "publisher", "ADR-0017.exact-package-manager-version"),
		selectTestManifest("npm-schema", "npm", "ARCH-verification-policy-and-fixtures.fixture-manifest-schema"),
	}}
}

func selectedNames(t *testing.T, index Index) []string {
	t.Helper()

	names := make([]string, 0, len(index.Fixtures))
	for _, manifest := range index.Fixtures {
		names = append(names, manifest.Name)
	}
	return names
}

func TestSelectFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		surface   string
		phase     string
		wantNames []string
	}{
		{
			name:      "no filters returns all fixtures",
			wantNames: []string{"npm-build", "npm-provenance", "publisher-build", "npm-schema"},
		},
		{
			name:      "surface only",
			surface:   "npm",
			wantNames: []string{"npm-build", "npm-provenance", "npm-schema"},
		},
		{
			name:      "phase only",
			phase:     "build-pack",
			wantNames: []string{"npm-build", "publisher-build"},
		},
		{
			name:      "combined surface and phase",
			surface:   "npm",
			phase:     "provenance-inputs",
			wantNames: []string{"npm-provenance"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			selected, err := Select(selectTestIndex(), test.surface, test.phase)
			if err != nil {
				t.Fatalf("Select() error = %v", err)
			}
			got := selectedNames(t, selected)
			if len(got) != len(test.wantNames) {
				t.Fatalf("Select() names = %v, want %v", got, test.wantNames)
			}
			for position, want := range test.wantNames {
				if got[position] != want {
					t.Fatalf("Select() names = %v, want %v", got, test.wantNames)
				}
			}
		})
	}
}

func TestSelectRejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		index   Index
		surface string
		phase   string
		wantErr string
	}{
		{
			name:    "unknown surface",
			index:   selectTestIndex(),
			surface: "maven",
			wantErr: `surface "maven" is not registered`,
		},
		{
			name:    "unknown phase",
			index:   selectTestIndex(),
			phase:   "signing",
			wantErr: `phase "signing" is not registered`,
		},
		{
			name:    "empty selection",
			index:   selectTestIndex(),
			phase:   "provenance-inputs",
			surface: "publisher",
			wantErr: `no fixtures match surface "publisher" and phase "provenance-inputs"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Select(test.index, test.surface, test.phase)
			if err == nil {
				t.Fatalf("Select() error = nil, want %q", test.wantErr)
			}
			if err.Error() != test.wantErr {
				t.Fatalf("Select() error = %q, want %q", err.Error(), test.wantErr)
			}
		})
	}
}

func TestSelectRejectsPathsOutsideSurface(t *testing.T) {
	t.Parallel()

	escapingArtifact := selectTestManifest("escaping-artifact", "npm", "ADR-0014.package-manager-matrix")
	escapingArtifact.Artifact = "testdata/publisher/shared/artifact.tgz"
	escapingProvenance := selectTestManifest("escaping-provenance", "npm", "ADR-0014.package-manager-matrix")
	escapingProvenance.Provenance = "testdata/publisher/shared/provenance.json"
	escapingReleaseManifest := selectTestManifest("escaping-release-manifest", "npm", "ADR-0014.package-manager-matrix")
	releaseManifest := "testdata/release-manifest/shared/manifest.json"
	escapingReleaseManifest.ReleaseManifest = &releaseManifest

	tests := []struct {
		name     string
		manifest Manifest
	}{
		{name: "artifact escapes surface", manifest: escapingArtifact},
		{name: "provenance escapes surface", manifest: escapingProvenance},
		{name: "release manifest escapes surface", manifest: escapingReleaseManifest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			index := Index{Fixtures: []Manifest{test.manifest}}
			_, err := Select(index, "npm", "")
			want := `fixture "` + test.manifest.Name + `" paths must stay under testdata/npm/`
			if err == nil {
				t.Fatalf("Select() error = nil, want %q", want)
			}
			if err.Error() != want {
				t.Fatalf("Select() error = %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestSelectDoesNotMutateInputIndex(t *testing.T) {
	t.Parallel()

	index := selectTestIndex()
	wantNames := selectedNames(t, index)

	selected, err := Select(index, "", "")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	// Mutating the returned selection must leave the input index untouched.
	selected.Fixtures[0].Name = "mutated"
	selected.Fixtures = append(selected.Fixtures, selectTestManifest("appended", "npm", "ADR-0014.package-manager-matrix"))

	gotNames := selectedNames(t, index)
	if len(gotNames) != len(wantNames) {
		t.Fatalf("input index names = %v, want %v", gotNames, wantNames)
	}
	for position, want := range wantNames {
		if gotNames[position] != want {
			t.Fatalf("input index mutated: names = %v, want %v", gotNames, wantNames)
		}
	}

	// A second selection from the same input must observe the original fixtures.
	reselected, err := Select(index, "npm", "")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	for _, manifest := range reselected.Fixtures {
		if strings.HasPrefix(manifest.Name, "mutated") || manifest.Name == "appended" {
			t.Fatalf("reselection observed mutated fixture %q", manifest.Name)
		}
	}
}
