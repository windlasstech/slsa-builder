package npmprofile

import "testing"

// TestIncrementLastSpecifiedFull pins the partial-completion upper bound for
// a fully specified partial: the patch component bumps and nothing zeroes.
// The public range grammar only increments non-full partials, so this unit
// exercises the full-partial case directly.
func TestIncrementLastSpecifiedFull(t *testing.T) {
	t.Parallel()
	partial := npmPartial{nums: [3]uint64{1, 2, 3}, specified: [3]bool{true, true, true}}
	if got, want := partial.incrementLastSpecified(), (npmSemver{major: 1, minor: 2, patch: 4}); got != want {
		t.Fatalf("incrementLastSpecified() = %#v, want %#v", got, want)
	}
}

// TestParseNPMPartialEmpty pins the empty-input rejection of the strict
// partial parser: an empty partial is unparseable, never a wildcard.
func TestParseNPMPartialEmpty(t *testing.T) {
	t.Parallel()
	if partial, ok := parseNPMPartial(""); ok {
		t.Fatalf("parseNPMPartial(\"\") = %#v, true; want unparseable", partial)
	}
}

// TestValidNPMPrereleaseCharacters pins the [0-9A-Za-z-] identifier alphabet
// of the strict prerelease grammar: any other byte makes the prerelease
// malformed.
func TestValidNPMPrereleaseCharacters(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		pre  string
		want bool
	}{
		{name: "underscore identifier", pre: "rc_1", want: false},
		{name: "plus identifier", pre: "rc+1", want: false},
		{name: "unicode identifier", pre: "rcé", want: false},
		{name: "hyphen identifier", pre: "rc-1", want: true},
		{name: "single hyphen", pre: "-", want: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := validNPMPrerelease(test.pre); got != test.want {
				t.Fatalf("validNPMPrerelease(%q) = %v, want %v", test.pre, got, test.want)
			}
		})
	}
}

// TestNPMOperatorBoundsUnknownOperator pins the fail-closed fall-through: an
// operator outside the closed comparison set yields the never-satisfied
// interval rather than a panic or an open bound.
func TestNPMOperatorBoundsUnknownOperator(t *testing.T) {
	t.Parallel()
	partial := npmPartial{nums: [3]uint64{1}, specified: [3]bool{true}}
	if got := npmOperatorBounds("~", partial); !got.never {
		t.Fatalf("npmOperatorBounds() = %#v, want the never-satisfied bounds", got)
	}
}

// TestNPMDeclaredMatchesActualExtra pins the range-grammar corners the main
// matrix does not reach: unparseable tilde and caret operands, unparseable
// hyphen endpoints, and the caret upper bound of a fully specified all-zero
// partial.
func TestNPMDeclaredMatchesActualExtra(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		declared string
		actual   string
		want     bool
	}{
		{name: "tilde build metadata operand unparseable", declared: "~1.2.3+build", actual: "1.2.3", want: false},
		{name: "caret build metadata operand unparseable", declared: "^1.2.3+build", actual: "1.2.3", want: false},
		{name: "hyphen unparseable left endpoint", declared: "v - 11.6.0", actual: "11.5.1", want: false},
		{name: "hyphen unparseable right endpoint", declared: "11.5.1 - v", actual: "11.5.1", want: false},
		{name: "caret all-zero full partial at bound", declared: "^0.0.0", actual: "0.0.0", want: true},
		{name: "caret all-zero full partial upper excluded", declared: "^0.0.0", actual: "0.0.1", want: false},
		{name: "equals wildcard", declared: "=x", actual: "11.5.1", want: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := npmDeclaredMatchesActual(test.declared, test.actual); got != test.want {
				t.Fatalf("npmDeclaredMatchesActual(%q, %q) = %v, want %v", test.declared, test.actual, got, test.want)
			}
		})
	}
}
