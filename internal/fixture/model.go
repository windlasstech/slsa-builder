// Package fixture loads and validates the executable conformance fixture registry.
package fixture

// Index is the closed top-level fixture registry document.
type Index struct {
	Fixtures []Manifest `json:"fixtures"`
}

// Manifest describes one accepted or rejected conformance fixture.
type Manifest struct {
	Name                    string   `json:"name"`
	Type                    string   `json:"type"`
	Surface                 string   `json:"surface"`
	Artifact                string   `json:"artifact"`
	Provenance              string   `json:"provenance"`
	ReleaseManifest         *string  `json:"release-manifest"`
	ExpectedResult          string   `json:"expected-result"`
	ExpectedFailureCategory *string  `json:"expected-failure-category"`
	ExpectedPrimaryID       *string  `json:"expected-primary-id"`
	ExpectedSecondaryIDs    []string `json:"expected-secondary-ids"`
	CoveredRequirement      string   `json:"covered-requirement"`
}
