package npmprofile

import "testing"

// TestNPMDeclaredMatchesActual pins the ADR 0095 comparison contract: an
// exact declared version matches only when it equals the actual version, a
// SemVer range matches only when it includes the actual version, and anything
// unparseable is unmatchable (the caller warns on false, never fails).
func TestNPMDeclaredMatchesActual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		declared string
		actual   string
		want     bool
	}{
		// Exact versions.
		{name: "exact match", declared: "11.5.1", actual: "11.5.1", want: true},
		{name: "exact mismatch patch", declared: "11.5.0", actual: "11.5.1", want: false},
		{name: "exact mismatch minor", declared: "11.4.1", actual: "11.5.1", want: false},
		{name: "exact mismatch major", declared: "10.5.1", actual: "11.5.1", want: false},
		{name: "v prefix match", declared: "v11.5.1", actual: "11.5.1", want: true},
		{name: "v prefix mismatch", declared: "v11.5.0", actual: "11.5.1", want: false},
		{name: "equals prefix match", declared: "=11.5.1", actual: "11.5.1", want: true},
		{name: "equals prefix mismatch", declared: "=11.5.0", actual: "11.5.1", want: false},
		{name: "equals v prefix match", declared: "=v11.5.1", actual: "11.5.1", want: true},
		{name: "exact prerelease never equals plain actual", declared: "11.5.1-rc.1", actual: "11.5.1", want: false},
		{name: "exact prerelease on other version", declared: "11.5.0-rc.1", actual: "11.5.1", want: false},
		{name: "build metadata unparseable", declared: "11.5.1+build", actual: "11.5.1", want: false},
		{name: "leading zero major", declared: "011.5.1", actual: "11.5.1", want: false},
		{name: "leading zero minor", declared: "11.05.1", actual: "11.5.1", want: false},
		{name: "four components", declared: "11.5.1.2", actual: "11.5.1", want: false},
		{name: "trailing dot", declared: "11.5.", actual: "11.5.1", want: false},
		{name: "overflow component", declared: "4294967296.0.0", actual: "11.5.1", want: false},
		{name: "prerelease numeric leading zero", declared: "11.5.1-rc.01", actual: "11.5.1", want: false},
		{name: "surrounding whitespace", declared: "  11.5.1  ", actual: "11.5.1", want: true},

		// Comparator sets.
		{name: "gte exact match", declared: ">=11.5.1", actual: "11.5.1", want: true},
		{name: "gte lower", declared: ">=11.5.0", actual: "11.5.1", want: true},
		{name: "gte higher", declared: ">=11.5.2", actual: "11.5.1", want: false},
		{name: "gt lower", declared: ">11.5.0", actual: "11.5.1", want: true},
		{name: "gt equal", declared: ">11.5.1", actual: "11.5.1", want: false},
		{name: "lte equal", declared: "<=11.5.1", actual: "11.5.1", want: true},
		{name: "lte lower", declared: "<=11.5.0", actual: "11.5.1", want: false},
		{name: "lt higher", declared: "<11.5.2", actual: "11.5.1", want: true},
		{name: "lt equal", declared: "<11.5.1", actual: "11.5.1", want: false},
		{name: "bare major partial match", declared: "11", actual: "11.5.1", want: true},
		{name: "bare major partial mismatch", declared: "12", actual: "11.5.1", want: false},
		{name: "bare minor partial match", declared: "11.5", actual: "11.5.1", want: true},
		{name: "bare minor partial low", declared: "11.4", actual: "11.5.1", want: false},
		{name: "bare minor partial high", declared: "11.6", actual: "11.5.1", want: false},
		{name: "gte partial major", declared: ">=11", actual: "11.5.1", want: true},
		{name: "gte partial major high", declared: ">=12", actual: "11.5.1", want: false},
		{name: "gte partial minor", declared: ">=11.5", actual: "11.5.1", want: true},
		{name: "gt partial major means next major", declared: ">11", actual: "11.5.1", want: false},
		{name: "gt partial minor means next minor", declared: ">11.5", actual: "11.5.1", want: false},
		{name: "gt partial minor low", declared: ">11.4", actual: "11.5.1", want: true},
		{name: "lt partial major", declared: "<12", actual: "11.5.1", want: true},
		{name: "lt partial major boundary", declared: "<11", actual: "11.5.1", want: false},
		{name: "lt partial minor", declared: "<11.6", actual: "11.5.1", want: true},
		{name: "lt partial minor boundary", declared: "<11.5", actual: "11.5.1", want: false},
		{name: "lte partial major", declared: "<=11", actual: "11.5.1", want: true},
		{name: "lte partial major low", declared: "<=10", actual: "11.5.1", want: false},
		{name: "lte partial minor", declared: "<=11.5", actual: "11.5.1", want: true},
		{name: "lte partial minor low", declared: "<=11.4", actual: "11.5.1", want: false},
		{name: "set both hold", declared: ">=11 <12", actual: "11.5.1", want: true},
		{name: "set exact lower and upper", declared: ">=11.5.1 <12", actual: "11.5.1", want: true},
		{name: "set lower fails", declared: ">=11.5.2 <12", actual: "11.5.1", want: false},
		{name: "set upper fails", declared: ">=11 <11.5.1", actual: "11.5.1", want: false},
		{name: "set contradictory partials", declared: ">11 <12", actual: "11.5.1", want: false},
		{name: "operator separated by space", declared: ">= 11.5.1", actual: "11.5.1", want: true},
		{name: "operator separated in set", declared: ">= 11 < 12", actual: "11.5.1", want: true},
		{name: "padded set", declared: "  >=11.5.1   <12  ", actual: "11.5.1", want: true},
		{name: "tab separated set", declared: ">=11\t<12", actual: "11.5.1", want: true},
		{name: "newline separated set", declared: ">=11\n<12", actual: "11.5.1", want: true},
		{name: "comparator with v", declared: ">=v11.5.1", actual: "11.5.1", want: true},
		{name: "double operator", declared: ">>11.5.1", actual: "11.5.1", want: false},
		{name: "equals then operator", declared: "=>11.5.1", actual: "11.5.1", want: false},
		{name: "spaced double operator", declared: "> =11.5.1", actual: "11.5.1", want: false},
		{name: "prerelease gte bound below actual", declared: ">=11.5.1-rc.1", actual: "11.5.1", want: true},
		{name: "prerelease gt bound below actual", declared: ">11.5.1-rc.1", actual: "11.5.1", want: true},
		{name: "prerelease lt bound below actual", declared: "<11.5.1-rc.1", actual: "11.5.1", want: false},
		{name: "prerelease lte bound below actual", declared: "<=11.5.1-rc.1", actual: "11.5.1", want: false},
		{name: "prerelease bound above actual", declared: ">=11.5.2-rc.1", actual: "11.5.1", want: false},

		// X-ranges.
		{name: "star", declared: "*", actual: "11.5.1", want: true},
		{name: "x lowercase", declared: "x", actual: "11.5.1", want: true},
		{name: "x uppercase", declared: "X", actual: "11.5.1", want: true},
		{name: "major x", declared: "11.x", actual: "11.5.1", want: true},
		{name: "major X", declared: "11.X", actual: "11.5.1", want: true},
		{name: "major star", declared: "11.*", actual: "11.5.1", want: true},
		{name: "major x mismatch", declared: "12.x", actual: "11.5.1", want: false},
		{name: "minor x", declared: "11.5.x", actual: "11.5.1", want: true},
		{name: "minor x mismatch", declared: "11.4.x", actual: "11.5.1", want: false},
		{name: "minor star", declared: "11.5.*", actual: "11.5.1", want: true},
		{name: "numeric after wildcard unparseable", declared: "11.x.2", actual: "11.5.1", want: false},
		{name: "numeric after wildcard major unparseable", declared: "x.5.1", actual: "11.5.1", want: false},
		{name: "gte wildcard", declared: ">=*", actual: "11.5.1", want: true},
		{name: "lte wildcard", declared: "<=*", actual: "11.5.1", want: true},
		{name: "gt wildcard never satisfied", declared: ">*", actual: "11.5.1", want: false},
		{name: "lt wildcard never satisfied", declared: "<*", actual: "11.5.1", want: false},
		{name: "gte wildcard minor", declared: ">=11.x", actual: "11.5.1", want: true},
		{name: "lt wildcard minor", declared: "<11.x", actual: "11.5.1", want: false},
		{name: "lte wildcard minor", declared: "<=11.x", actual: "11.5.1", want: true},
		{name: "never satisfied arm stays parseable", declared: ">* || 11.5.1", actual: "11.5.1", want: true},

		// Caret ranges.
		{name: "caret same", declared: "^11.5.1", actual: "11.5.1", want: true},
		{name: "caret lower same minor", declared: "^11.5.0", actual: "11.5.1", want: true},
		{name: "caret higher patch", declared: "^11.5.2", actual: "11.5.1", want: false},
		{name: "caret higher minor", declared: "^11.6.0", actual: "11.5.1", want: false},
		{name: "caret previous major", declared: "^10.0.0", actual: "11.5.1", want: false},
		{name: "caret major only", declared: "^11", actual: "11.5.1", want: true},
		{name: "caret major only high", declared: "^12", actual: "11.5.1", want: false},
		{name: "caret minor x", declared: "^11.5.x", actual: "11.5.1", want: true},
		{name: "caret minor x high", declared: "^11.6.x", actual: "11.5.1", want: false},
		{name: "caret major x", declared: "^11.x", actual: "11.5.1", want: true},
		{name: "caret major x high", declared: "^12.x", actual: "11.5.1", want: false},
		{name: "caret any", declared: "^x", actual: "11.5.1", want: true},
		{name: "caret bare invalid", declared: "^", actual: "11.5.1", want: false},
		{name: "caret with v", declared: "^v11.5.1", actual: "11.5.1", want: true},
		{name: "caret prerelease lower bound", declared: "^11.5.1-rc.1", actual: "11.5.1", want: true},
		{name: "caret zero minor included", declared: "^0.2.3", actual: "0.2.9", want: true},
		{name: "caret zero minor upper excluded", declared: "^0.2.3", actual: "0.3.0", want: false},
		{name: "caret zero minor lower excluded", declared: "^0.2.3", actual: "0.2.2", want: false},
		{name: "caret double zero patch included", declared: "^0.0.3", actual: "0.0.3", want: true},
		{name: "caret double zero patch upper excluded", declared: "^0.0.3", actual: "0.0.4", want: false},
		{name: "caret double zero x", declared: "^0.0.x", actual: "0.0.9", want: true},
		{name: "caret double zero x upper excluded", declared: "^0.0.x", actual: "0.1.0", want: false},
		{name: "caret zero x", declared: "^0.x", actual: "0.9.9", want: true},
		{name: "caret zero x upper excluded", declared: "^0.x", actual: "1.0.0", want: false},
		{name: "caret zero", declared: "^0", actual: "0.0.0", want: true},
		{name: "caret zero upper excluded", declared: "^0", actual: "1.0.0", want: false},
		{name: "caret zero zero", declared: "^0.0", actual: "0.0.5", want: true},
		{name: "caret zero zero upper excluded", declared: "^0.0", actual: "0.1.0", want: false},

		// Tilde ranges.
		{name: "tilde same", declared: "~11.5.1", actual: "11.5.1", want: true},
		{name: "tilde lower patch", declared: "~11.5.0", actual: "11.5.1", want: true},
		{name: "tilde higher patch", declared: "~11.5.2", actual: "11.5.1", want: false},
		{name: "tilde upper minor excluded", declared: "~11.5.0", actual: "11.6.0", want: false},
		{name: "tilde lower minor", declared: "~11.4.9", actual: "11.5.1", want: false},
		{name: "tilde partial minor", declared: "~11.5", actual: "11.5.1", want: true},
		{name: "tilde partial minor low", declared: "~11.4", actual: "11.5.1", want: false},
		{name: "tilde partial major", declared: "~11", actual: "11.5.1", want: true},
		{name: "tilde partial major high", declared: "~12", actual: "11.5.1", want: false},
		{name: "tilde minor x", declared: "~11.5.x", actual: "11.5.1", want: true},
		{name: "tilde minor x high", declared: "~11.6.x", actual: "11.5.1", want: false},
		{name: "tilde any", declared: "~x", actual: "11.5.1", want: true},
		{name: "tilde bare invalid", declared: "~", actual: "11.5.1", want: false},
		{name: "tilde with v", declared: "~v11.5.1", actual: "11.5.1", want: true},
		{name: "tilde zero minor", declared: "~0.2.3", actual: "0.2.9", want: true},
		{name: "tilde zero minor upper excluded", declared: "~0.2.3", actual: "0.3.0", want: false},

		// Hyphen ranges.
		{name: "hyphen inclusive both", declared: "11.5.1 - 11.5.1", actual: "11.5.1", want: true},
		{name: "hyphen within", declared: "11.0.0 - 11.6.0", actual: "11.5.1", want: true},
		{name: "hyphen at upper", declared: "11.0.0 - 11.5.1", actual: "11.5.1", want: true},
		{name: "hyphen at lower", declared: "11.5.1 - 11.6.0", actual: "11.5.1", want: true},
		{name: "hyphen above upper", declared: "11.5.2 - 11.6.0", actual: "11.5.1", want: false},
		{name: "hyphen below lower", declared: "11.0.0 - 11.5.0", actual: "11.5.1", want: false},
		{name: "hyphen partial minor upper", declared: "11.2 - 11.5", actual: "11.5.1", want: true},
		{name: "hyphen partial minor upper excluded", declared: "11.2 - 11.5", actual: "11.6.0", want: false},
		{name: "hyphen partial major upper", declared: "11.2 - 11", actual: "11.5.1", want: true},
		{name: "hyphen partial major upper excluded", declared: "11.2 - 10", actual: "11.5.1", want: false},
		{name: "hyphen partial lower", declared: "11.5 - 11.9", actual: "11.5.1", want: true},
		{name: "hyphen partial lower excluded", declared: "11.6 - 11.9", actual: "11.5.1", want: false},
		{name: "hyphen other range", declared: "1.2.3 - 2.3.4", actual: "11.5.1", want: false},
		{name: "hyphen without spaces is a prerelease", declared: "11.5.1-11.6.0", actual: "11.5.1", want: false},
		{name: "hyphen missing right", declared: "11.5.1 -", actual: "11.5.1", want: false},
		{name: "hyphen missing left", declared: "- 11.5.1", actual: "11.5.1", want: false},
		{name: "double hyphen", declared: "11.5.1 - 11.6.0 - 11.7.0", actual: "11.5.1", want: false},
		{name: "hyphen wildcard left", declared: "* - 11.6.0", actual: "11.5.1", want: false},
		{name: "hyphen wildcard right", declared: "11.5.1 - *", actual: "11.5.1", want: false},
		{name: "hyphen extra spacing", declared: "11.0.0  -  11.6.0", actual: "11.5.1", want: true},
		{name: "hyphen with v prefixes", declared: "v11.0.0 - v11.6.0", actual: "11.5.1", want: true},
		{name: "hyphen lower prerelease", declared: "11.5.1-rc.1 - 11.6.0", actual: "11.5.1", want: true},
		{name: "hyphen upper prerelease below actual", declared: "11.0.0 - 11.5.1-rc.1", actual: "11.5.1", want: false},

		// Unions.
		{name: "union left", declared: "^10.0.0 || ^11.0.0", actual: "11.5.1", want: true},
		{name: "union right", declared: "^11.0.0 || ^12.0.0", actual: "11.5.1", want: true},
		{name: "union none", declared: "^10.0.0 || ^12.0.0", actual: "11.5.1", want: false},
		{name: "union exact arms", declared: "11.5.0 || 11.5.1", actual: "11.5.1", want: true},
		{name: "union exact arms none", declared: "11.5.0 || 11.5.2", actual: "11.5.1", want: false},
		{name: "union without spaces", declared: "11.5.1||11.5.2", actual: "11.5.1", want: true},
		{name: "union leading empty arm poisons", declared: "|| 11.5.1", actual: "11.5.2", want: false},
		{name: "union trailing empty arm poisons", declared: "11.5.1 ||", actual: "11.5.2", want: false},
		{name: "union empty arm poisons a matching arm", declared: "|| 11.5.1", actual: "11.5.1", want: false},
		{name: "union only separators", declared: "||", actual: "11.5.1", want: false},
		{name: "union garbage arm poisons left", declared: "garbage || 11.5.1", actual: "11.5.1", want: false},
		{name: "union garbage arm poisons right", declared: "11.5.1 || garbage", actual: "11.5.1", want: false},

		// Unparseable declarations.
		{name: "empty", declared: "", actual: "11.5.1", want: false},
		{name: "whitespace only", declared: "   ", actual: "11.5.1", want: false},
		{name: "latest tag", declared: "latest", actual: "11.5.1", want: false},
		{name: "npm name", declared: "npm", actual: "11.5.1", want: false},
		{name: "npm at version", declared: "npm@11.5.1", actual: "11.5.1", want: false},
		{name: "double gt", declared: ">>1", actual: "11.5.1", want: false},
		{name: "lone v", declared: "v", actual: "11.5.1", want: false},
		{name: "dangling prerelease", declared: "1.2.3-", actual: "11.5.1", want: false},
		{name: "url", declared: "https://example.com/npm-11.5.1.tgz", actual: "11.5.1", want: false},
		{name: "git url", declared: "git+https://example.com/repo.git#11.5.1", actual: "11.5.1", want: false},

		// Other actuals.
		{name: "actual 10 exact", declared: "10.0.0", actual: "10.0.0", want: true},
		{name: "actual 10 range", declared: ">=10 <11", actual: "10.0.0", want: true},
		{name: "actual 10 mismatch", declared: "11.5.1", actual: "10.0.0", want: false},
		{name: "actual zero", declared: "0.0.0", actual: "0.0.0", want: true},
		{name: "unparseable actual", declared: "*", actual: "garbage", want: false},
		{name: "prerelease actual", declared: "*", actual: "11.5.1-rc.1", want: false},
		{name: "partial actual", declared: "*", actual: "11.5", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := npmDeclaredMatchesActual(test.declared, test.actual); got != test.want {
				t.Fatalf("npmDeclaredMatchesActual(%q, %q) = %v, want %v", test.declared, test.actual, got, test.want)
			}
		})
	}
}
