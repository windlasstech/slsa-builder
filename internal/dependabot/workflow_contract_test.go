package dependabot

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

type companionWorkflowViolation struct {
	path   string
	reason string
}

func (e *companionWorkflowViolation) Error() string { return e.path + ": " + e.reason }

// Dynamic YAML values stay inside this test-only boundary checker. Typed struct
// decoding would discard unknown keys or coerce scalar shapes before comparison.
func checkCompanionWorkflow(raw []byte) error {
	var document any
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("workflow: decode YAML: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return &companionWorkflowViolation{"workflow", "expected exactly one YAML document"}
	}
	return compareCompanionWorkflow("workflow", document, reviewedCompanionWorkflow())
}

func compareCompanionWorkflow(path string, got, want any) error {
	fail := &companionWorkflowViolation{path, "value or type differs from reviewed contract"}
	switch expected := want.(type) {
	case companionDisplayLabel:
		if _, ok := got.(string); !ok {
			return fail
		}
	case map[string]any:
		actual, ok := got.(map[string]any)
		if !ok || actual == nil {
			return fail
		}
		for _, key := range slices.Sorted(maps.Keys(actual)) {
			if _, ok := expected[key]; !ok {
				return &companionWorkflowViolation{path + "." + key, "unreviewed key"}
			}
		}
		for _, key := range slices.Sorted(maps.Keys(expected)) {
			value, ok := actual[key]
			if !ok {
				return &companionWorkflowViolation{path + "." + key, "required key missing"}
			}
			if err := compareCompanionWorkflow(path+"."+key, value, expected[key]); err != nil {
				return err
			}
		}
	case []any:
		actual, ok := got.([]any)
		if !ok || len(actual) != len(expected) {
			return fail
		}
		for i, value := range expected {
			if err := compareCompanionWorkflow(fmt.Sprintf("%s[%d]", path, i), actual[i], value); err != nil {
				return err
			}
		}
	case string:
		actual, ok := got.(string)
		if !ok {
			return fail
		}
		switch {
		case strings.HasSuffix(path, ".allowed-endpoints"):
			actualEndpoints, expectedEndpoints := strings.Fields(actual), strings.Fields(expected)
			slices.Sort(actualEndpoints)
			slices.Sort(expectedEndpoints)
			if !slices.Equal(actualEndpoints, expectedEndpoints) {
				return fail
			}
		case strings.HasSuffix(path, ".if"):
			if normalizeCompanionCondition(actual) != normalizeCompanionCondition(expected) {
				return fail
			}
		default:
			if actual != expected {
				return fail
			}
		}
	default:
		if !reflect.DeepEqual(got, want) {
			return fail
		}
	}
	return nil
}

func normalizeCompanionCondition(value string) string {
	// Preserve quoted literal bytes; only whitespace outside literals is cosmetic.
	parts := strings.Split(value, "'")
	spaces := regexp.MustCompile(`[ \t\r\n]+`)
	for i := 0; i < len(parts); i += 2 {
		parts[i] = spaces.ReplaceAllString(parts[i], " ")
	}
	return strings.TrimSpace(strings.Join(parts, "'"))
}
