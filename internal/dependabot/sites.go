package dependabot

import (
	"bytes"
	"errors"
	"regexp"
	"slices"
	"strings"
)

var sourceSites = []string{
	"internal/provenance/builder_dependencies.go",
	"internal/provenance/builder_dependencies_test.go",
	"internal/provenance/signing_adapter_graph_test.go",
	"internal/provenance/signing_adapter_candidate_test.go",
	"internal/npmprofile/publish_cli_builder_test.go",
	"internal/npmprofile/publish_cli_consumer_test.go",
	"internal/command/verify_attestation_publish_cli_test.go",
	"cmd/slsa-builder-internal/signing_adapter_test.go",
	"internal/toolchainpins/signing_adapter_test.go",
	"docs/dependency-selection.md",
	"docs/architecture/core-profile-contract.md",
	"docs/architecture/slsa-provenance-v1.md",
	"docs/architecture/release-manifest.md",
	"docs/architecture/verification-policy-and-fixtures.md",
	"docs/architecture/js-ts-npm-provenance-publish.md",
	"CHANGELOG.md",
}

var commonFixtures = []string{
	"valid-statement.json", "valid-statement-corepack.json", "valid-clock-skew.json", "valid-predicate.jcs.json",
	"reject-statement-type.json", "reject-predicate-type.json", "reject-internal-parameters.json", "reject-zero-subjects.json", "reject-multiple-subjects.json",
	"reject-timestamp-format.json", "reject-timestamp-ordering.json", "reject-builder-version.json", "reject-run-invocation.json",
}

func allowedOutput(path string) bool {
	if strings.Contains(path, "..") || strings.ContainsAny(path, "\\\x00\r\n") {
		return false
	}
	if slices.Contains(sourceSites, path) {
		return true
	}
	if name, ok := strings.CutPrefix(path, "testdata/provenance/"); ok {
		return slices.Contains(commonFixtures, name)
	}
	if name, ok := strings.CutPrefix(path, "testdata/npm/provenance/"); ok {
		return !strings.Contains(name, "/") && ((strings.HasPrefix(name, "contract-") && strings.HasSuffix(name, ".json")) || name == "npm-predicate.jcs.json" || name == "npm-predicate-source-ref-dispatch-retry.jcs.json")
	}
	return false
}

func editSite(path string, raw []byte, prepared prepared) ([]byte, error) {
	old, next := prepared.transition.old, prepared.transition.candidate
	text := string(raw)
	prefix, suffix := "", ""
	if path == "CHANGELOG.md" {
		start := strings.Index(text, "## [Unreleased]")
		if start < 0 {
			return nil, errors.New("missing current changelog section")
		}
		prefix, text = text[:start], text[start:]
		if end := strings.Index(text[1:], "\n## ["); end >= 0 {
			end++
			text, suffix = text[:end], text[end:]
		}
	}
	if path == "internal/toolchainpins/signing_adapter_test.go" {
		// The table below this test exercises arbitrary versions, not current pins.
		end := strings.Index(text, "\nfunc TestParse")
		if end < 0 {
			return nil, errors.New("missing parser test boundary")
		}
		text, suffix = text[:end], text[end:]
	}
	replacer := strings.NewReplacer(old.Version, next.Version, old.DirHash1, next.DirHash1, strings.ToUpper(old.DirHash1), strings.ToUpper(next.DirHash1), strings.TrimPrefix(old.Sum, "h1:"), strings.TrimPrefix(next.Sum, "h1:"))
	if path == "CHANGELOG.md" {
		reference := regexp.MustCompile(`(?m)(sigstore-go(?:` + "`" + `)?(?: from v[0-9]+\.[0-9]+\.[0-9]+ to | to |@|[ \t])(?:` + "`" + `)?)` + regexp.QuoteMeta(old.Version) + `\b`)
		updated := reference.ReplaceAllString(text, "${1}"+next.Version)
		if updated != text {
			text = updated
		} else {
			entry := "- Updated sigstore-go from " + old.Version + " to " + next.Version + "."
			if start := strings.Index(text, "\n### Changed\n"); start >= 0 {
				bodyStart := start + len("\n### Changed\n")
				bodyEnd := len(text)
				if nextHeading := strings.Index(text[bodyStart:], "\n### "); nextHeading >= 0 {
					bodyEnd = bodyStart + nextHeading
				}
				section := strings.TrimRight(text[bodyStart:bodyEnd], "\n")
				text = text[:bodyStart] + section + "\n" + entry + "\n" + text[bodyEnd:]
			} else {
				block := "### Changed\n\n" + entry + "\n"
				insert := len(text)
				if start := strings.Index(text, "\n### Added\n"); start >= 0 {
					bodyStart := start + len("\n### Added\n")
					if nextHeading := strings.Index(text[bodyStart:], "\n### "); nextHeading >= 0 {
						insert = bodyStart + nextHeading
					}
				} else if nextHeading := strings.Index(text, "\n### "); nextHeading >= 0 {
					insert = nextHeading
				}
				if insert == len(text) {
					text = strings.TrimRight(text, "\n") + "\n\n" + block
				} else {
					text = text[:insert] + "\n" + block + text[insert:]
				}
			}
		}
	} else {
		text = replacer.Replace(text)
	}
	if path == "docs/dependency-selection.md" {
		date := regexp.MustCompile(`(was released on\n[ \t]*)1[0-9]{4}-[0-9]{2}-[0-9]{2}`)
		goRequirement := regexp.MustCompile(`declares Go [0-9]+\.[0-9]+(?:\.[0-9]+)?, which is compatible`)
		if len(date.FindAllStringIndex(text, -1)) != 1 || len(goRequirement.FindAllStringIndex(text, -1)) != 1 {
			return nil, errors.New("release documentation site drift")
		}
		text = date.ReplaceAllString(text, "${1}1"+prepared.release.ReleaseDate)
		text = goRequirement.ReplaceAllString(text, "declares Go "+prepared.release.GoVersion+", which is compatible")
	}
	result := []byte(prefix + text + suffix)
	if bytes.Equal(raw, result) {
		return nil, errors.New("current-version site missing or companion already present")
	}
	return result, nil
}
