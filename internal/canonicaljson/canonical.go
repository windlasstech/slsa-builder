package canonicaljson

import (
	"bytes"
	"fmt"

	jsoncanonicalizer "github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
)

// Canonicalize validates data and returns its RFC 8785 JCS canonical UTF-8 serialization.
func Canonicalize(data []byte) ([]byte, error) {
	if err := Validate(data); err != nil {
		return nil, err
	}

	// The vetted reference transformer accepts an object or array at its root. Wrapping the complete
	// strict value in an array preserves its JCS representation while supporting primitive roots.
	// Build by append only: sizing the allocation with len(data)+2 trips CodeQL's
	// go/allocation-size-overflow rule.
	wrapper := append([]byte{'['}, data...)
	wrapper = append(wrapper, ']')

	canonical, err := jsoncanonicalizer.Transform(wrapper)
	if err != nil {
		return nil, fmt.Errorf("canonicalize JSON with RFC 8785 JCS: %w", err)
	}
	if len(canonical) < 2 || canonical[0] != '[' || canonical[len(canonical)-1] != ']' {
		return nil, errorsUnexpectedCanonicalWrapper(canonical)
	}
	return bytes.Clone(canonical[1 : len(canonical)-1]), nil
}

// Equal reports recursive JSON value equality after strict duplicate-aware parsing.
func Equal(left, right []byte) (bool, error) {
	leftValue, err := parseStrict(left)
	if err != nil {
		return false, fmt.Errorf("parse left JSON value: %w", err)
	}
	rightValue, err := parseStrict(right)
	if err != nil {
		return false, fmt.Errorf("parse right JSON value: %w", err)
	}
	return equalValues(leftValue, rightValue), nil
}

func equalValues(left, right any) bool {
	switch left := left.(type) {
	case nil:
		return right == nil
	case bool:
		right, ok := right.(bool)
		return ok && left == right
	case float64:
		right, ok := right.(float64)
		return ok && left == right
	case string:
		right, ok := right.(string)
		return ok && left == right
	case []any:
		right, ok := right.([]any)
		if !ok || len(left) != len(right) {
			return false
		}
		for index := range left {
			if !equalValues(left[index], right[index]) {
				return false
			}
		}
		return true
	case map[string]any:
		right, ok := right.(map[string]any)
		if !ok || len(left) != len(right) {
			return false
		}
		for member, leftValue := range left {
			rightValue, exists := right[member]
			if !exists || !equalValues(leftValue, rightValue) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func errorsUnexpectedCanonicalWrapper(canonical []byte) error {
	return fmt.Errorf("RFC 8785 transformer returned invalid wrapped output %q", canonical)
}
