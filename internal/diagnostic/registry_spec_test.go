package diagnostic

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

type specTaxonomyEntry struct {
	phase            Phase
	exitCode         int
	mutationPossible bool
}

var canonicalDiagnosticID = regexp.MustCompile(`windlass\.verify\.(?:error|warning)\.[a-z0-9]+(?:-[a-z0-9]+)*`)

func TestRegistryMatchesSpecTaxonomy(t *testing.T) {
	t.Parallel()

	specPaths, err := filepath.Glob(filepath.Join("..", "..", "docs", "architecture", "*.md"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	specs := make(map[string]string, len(specPaths))
	for _, specPath := range specPaths {
		markdown, readErr := os.ReadFile(specPath)
		if readErr != nil {
			t.Fatalf("ReadFile(%q) error = %v", specPath, readErr)
		}
		specs[filepath.Base(specPath)] = string(markdown)
	}

	want, err := parseSpecTaxonomy(specs)
	if err != nil {
		t.Fatalf("parseSpecTaxonomy() error = %v", err)
	}
	gotIDs := RegisteredIDs()
	if len(gotIDs) != len(want) {
		t.Errorf("registry contains %d IDs, spec taxonomy contains %d", len(gotIDs), len(want))
	}
	for _, id := range gotIDs {
		definition, ok := Lookup(id)
		if !ok {
			t.Fatalf("Lookup(%q) unexpectedly failed", id)
		}
		expected, ok := want[id]
		if !ok {
			t.Errorf("registry ID %q is absent from the spec taxonomy", id)
			continue
		}
		if definition.Phase != expected.phase || definition.ExitCode != expected.exitCode || definition.MutationPossible != expected.mutationPossible {
			t.Errorf("%s = phase %q, exit %d, mutation_possible %t; spec requires %q, %d, %t",
				id, definition.Phase, definition.ExitCode, definition.MutationPossible,
				expected.phase, expected.exitCode, expected.mutationPossible)
		}
		delete(want, id)
	}
	for id := range want {
		t.Errorf("spec taxonomy ID %q is absent from the registry", id)
	}
}

func parseSpecTaxonomy(specs map[string]string) (map[string]specTaxonomyEntry, error) {
	entries := make(map[string]specTaxonomyEntry)
	verificationSpec, ok := specs["verification-policy-and-fixtures.md"]
	if !ok {
		return nil, fmt.Errorf("verification policy specification not found")
	}

	warnings, err := markdownSection(verificationSpec, "The non-fatal warning IDs initially registered by this specification are:")
	if err != nil {
		return nil, err
	}
	for _, table := range markdownTables(warnings) {
		if !table.hasHeaders("Diagnostic ID", "Meaning") {
			continue
		}
		for _, row := range table.rows {
			for _, id := range canonicalDiagnosticID.FindAllString(row[table.column("Diagnostic ID")], -1) {
				if err := addSpecEntry(entries, id, specTaxonomyEntry{phase: PhaseVerification, exitCode: ExitCodePass}); err != nil {
					return nil, err
				}
			}
		}
		break
	}

	rejected, err := markdownSection(verificationSpec, "## Rejected fixture categories")
	if err != nil {
		return nil, err
	}
	for _, table := range markdownTables(rejected) {
		if !table.hasHeaders("Category", "Description") {
			continue
		}
		for _, row := range table.rows {
			category := stripMarkdownCode(row[table.column("Category")])
			id := "windlass.verify.error." + category
			if _, explicitlyAssigned := entries[id]; explicitlyAssigned {
				continue
			}
			if err := addSpecEntry(entries, id, specTaxonomyEntry{phase: PhaseVerification, exitCode: ExitCodePolicyFailure}); err != nil {
				return nil, err
			}
		}
		break
	}

	explicit := make(map[string]specTaxonomyEntry)
	metadataTables := 0
	for name, markdown := range specs {
		for _, table := range markdownTables(markdown) {
			if !table.hasHeaders("Diagnostic ID", "Phase", "Exit code", "Mutation possible") {
				continue
			}
			metadataTables++
			for _, row := range table.rows {
				ids := canonicalDiagnosticID.FindAllString(row[table.column("Diagnostic ID")], -1)
				if len(ids) == 0 {
					continue
				}
				phase := Phase(stripMarkdownCode(row[table.column("Phase")]))
				if !validPhase(phase) {
					return nil, fmt.Errorf("%s: invalid phase %q for %q", name, phase, ids)
				}
				exitCode, parseErr := strconv.Atoi(stripMarkdownCode(row[table.column("Exit code")]))
				if parseErr != nil {
					return nil, fmt.Errorf("%s: invalid exit code for %q: %w", name, ids, parseErr)
				}
				mutationPossible, parseErr := strconv.ParseBool(stripMarkdownCode(row[table.column("Mutation possible")]))
				if parseErr != nil {
					return nil, fmt.Errorf("%s: invalid mutation_possible for %q: %w", name, ids, parseErr)
				}
				entry := specTaxonomyEntry{phase: phase, exitCode: exitCode, mutationPossible: mutationPossible}
				for _, id := range ids {
					if previous, exists := explicit[id]; exists && previous != entry {
						return nil, fmt.Errorf("%s: contradictory explicit taxonomy for %q: %#v and %#v", name, id, previous, entry)
					}
					explicit[id] = entry
				}
			}
		}
	}
	if metadataTables == 0 {
		return nil, fmt.Errorf("no diagnostic metadata tables found")
	}
	for id, entry := range explicit {
		entries[id] = entry
	}

	return entries, nil
}

func addSpecEntry(entries map[string]specTaxonomyEntry, id string, entry specTaxonomyEntry) error {
	if _, exists := entries[id]; exists {
		return fmt.Errorf("duplicate diagnostic ID %q in spec taxonomy", id)
	}
	entries[id] = entry
	return nil
}

func validPhase(phase Phase) bool {
	return phase == PhaseInvocation || phase == PhasePolicy || phase == PhaseVerification || phase == PhasePreMutation || phase == PhaseMutation
}

func markdownSection(markdown, heading string) (string, error) {
	start := strings.Index(markdown, heading)
	if start < 0 {
		return "", fmt.Errorf("heading %q not found", heading)
	}
	section := markdown[start+len(heading):]
	for _, prefix := range []string{"\n## ", "\n### "} {
		if end := strings.Index(section, prefix); end >= 0 {
			section = section[:end]
		}
	}
	return section, nil
}

type markdownTable struct {
	headings []string
	rows     [][]string
}

func (table markdownTable) hasHeaders(headings ...string) bool {
	for _, heading := range headings {
		if table.column(heading) < 0 {
			return false
		}
	}
	return true
}

func (table markdownTable) column(heading string) int {
	want := normalizeHeading(heading)
	for index, candidate := range table.headings {
		if normalizeHeading(candidate) == want {
			return index
		}
	}
	return -1
}

func markdownTables(markdown string) []markdownTable {
	lines := strings.Split(markdown, "\n")
	var tables []markdownTable
	for index := 0; index+1 < len(lines); index++ {
		headings := markdownRow(lines[index])
		separator := markdownRow(lines[index+1])
		if len(headings) == 0 || len(headings) != len(separator) || !isTableSeparator(separator) {
			continue
		}
		table := markdownTable{headings: headings}
		index += 2
		for index < len(lines) {
			row := markdownRow(lines[index])
			if len(row) != len(headings) {
				break
			}
			table.rows = append(table.rows, row)
			index++
		}
		tables = append(tables, table)
		index--
	}
	return tables
}

func markdownRow(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
		return nil
	}
	parts := strings.Split(strings.Trim(line, "|"), "|")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}

func isTableSeparator(row []string) bool {
	for _, cell := range row {
		cell = strings.Trim(cell, " :")
		if len(cell) < 3 || strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return true
}

func stripMarkdownCode(value string) string {
	return strings.Trim(strings.TrimSpace(value), "`")
}

func normalizeHeading(value string) string {
	value = stripMarkdownCode(value)
	value = strings.NewReplacer("-", "_", " ", "_").Replace(value)
	return strings.ToLower(value)
}
