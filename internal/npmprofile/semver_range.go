package npmprofile

import (
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

// npmDeclaredMatchesActual reports whether a declared npm descriptor version
// provably matches the toolchain npm's actual version. ADR 0095 makes a
// declared npm version accepted but non-authoritative: the build compares it
// with the observed npm CLI version and emits the
// windlass.verify.warning.npm-version-mismatch warning on mismatch, never
// failing the build. An exact declared version matches only when it equals
// the actual version; a SemVer range matches only when it includes the actual
// version; anything unparseable is unmatchable. The parser is a strict,
// deterministic trust-boundary parser: it never panics and fails toward
// false.
//
// The grammar is the node-semver subset the npm CLI itself reads: exact
// versions with an optional leading v or =, comparator sets with partial
// completion (>=, >, <=, <, =, bare), X-ranges, caret and tilde ranges,
// hyphen ranges, and || unions. The actual version is always a plain
// three-part release version, so node-semver's prerelease gating for
// candidate versions never applies; prerelease bounds evaluate by ordinary
// SemVer precedence. Two corners are deliberately stricter than node-semver
// because false only warns: an empty or unparseable union arm makes the whole
// declaration unmatchable, and build metadata (+...) is unparseable.
func npmDeclaredMatchesActual(declared, actual string) bool {
	actualVersion, ok := parseNPMRangeActual(actual)
	if !ok {
		return false
	}
	trimmed := strings.TrimSpace(declared)
	if trimmed == "" {
		return false
	}
	matched := false
	for _, arm := range strings.Split(trimmed, "||") {
		armMatched, armOK := npmRangeArmMatches(arm, actualVersion)
		if !armOK {
			return false
		}
		matched = matched || armMatched
	}
	return matched
}

// npmSemver is a fully specified SemVer 2.0.0 version.
type npmSemver struct {
	major  uint64
	minor  uint64
	patch  uint64
	pre    string
	hasPre bool
}

// compare returns the SemVer precedence of v relative to other.
func (v npmSemver) compare(other npmSemver) int {
	return semver.Compare(v.canonical(), other.canonical())
}

// canonical renders v in the strict v-prefixed form x/mod/semver requires;
// v is always strictly parsed, so the result is always valid input.
func (v npmSemver) canonical() string {
	version := "v" + strconv.FormatUint(v.major, 10) + "." + strconv.FormatUint(v.minor, 10) + "." + strconv.FormatUint(v.patch, 10)
	if v.hasPre {
		version += "-" + v.pre
	}
	return version
}

// npmPartial is a parsed node-semver partial version: a prefix of numeric
// components with the remainder wildcarded by x, X, or *, and an optional
// prerelease that is valid only on a full three-component version.
type npmPartial struct {
	nums      [3]uint64
	specified [3]bool
	pre       string
	hasPre    bool
}

// full reports whether all three components are given.
func (p npmPartial) full() bool {
	return p.specified[0] && p.specified[1] && p.specified[2]
}

// wildcard reports whether no component is given.
func (p npmPartial) wildcard() bool {
	return !p.specified[0]
}

// zeroFilled completes the partial with zeros, keeping the prerelease of a
// full version.
func (p npmPartial) zeroFilled() npmSemver {
	version := npmSemver{major: p.nums[0], minor: p.nums[1], patch: p.nums[2]}
	if p.full() && p.hasPre {
		version.pre = p.pre
		version.hasPre = true
	}
	return version
}

// incrementLastSpecified bumps the last given component and zeros the rest:
// the partial-completion upper bound of node-semver, where 1.2 completes to
// 1.3.0 and 1 completes to 2.0.0.
func (p npmPartial) incrementLastSpecified() npmSemver {
	nums := p.nums
	switch {
	case p.specified[2]:
		nums[2]++
	case p.specified[1]:
		nums[1]++
		nums[2] = 0
	default:
		nums[0]++
		nums[1] = 0
		nums[2] = 0
	}
	return npmSemver{major: nums[0], minor: nums[1], patch: nums[2]}
}

// parseNPMRangeActual parses the observed npm --version output, always a
// plain three-part release version.
func parseNPMRangeActual(actual string) (npmSemver, bool) {
	partial, ok := parseNPMPartial(actual)
	if !ok || !partial.full() || partial.hasPre {
		return npmSemver{}, false
	}
	return partial.zeroFilled(), true
}

// parseNPMPartial parses a node-semver partial version with an optional
// leading v. Wildcards form the tail of the version: a numeric component
// after a wildcard (1.x.2) is unparseable. Build metadata is not part of the
// grammar: any + makes the input unparseable.
func parseNPMPartial(text string) (npmPartial, bool) {
	var partial npmPartial
	if text == "" {
		return partial, false
	}
	text = strings.TrimPrefix(text, "v")
	if text == "" || strings.Contains(text, "+") {
		return partial, false
	}
	core, pre, hasPre := strings.Cut(text, "-")
	if hasPre && !validNPMPrerelease(pre) {
		return partial, false
	}
	parts := strings.Split(core, ".")
	if len(parts) > 3 {
		return partial, false
	}
	wildcardSeen := false
	for i, part := range parts {
		if part == "x" || part == "X" || part == "*" {
			wildcardSeen = true
			continue
		}
		number, ok := parseNPMVersionNumber(part)
		if !ok || wildcardSeen {
			return npmPartial{}, false
		}
		partial.nums[i] = number
		partial.specified[i] = true
	}
	if hasPre {
		if !partial.full() {
			return npmPartial{}, false
		}
		partial.pre = pre
		partial.hasPre = true
	}
	return partial, true
}

// parseNPMVersionNumber parses one numeric version component: digits only, no
// leading zeros, bounded well beyond any realistic version.
func parseNPMVersionNumber(text string) (uint64, bool) {
	if text == "" {
		return 0, false
	}
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return 0, false
		}
	}
	if len(text) > 1 && text[0] == '0' {
		return 0, false
	}
	number, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		return 0, false
	}
	return number, true
}

// validNPMPrerelease reports whether pre is a well-formed SemVer 2.0.0
// prerelease: non-empty dot-separated [0-9A-Za-z-] identifiers, numeric
// identifiers without leading zeros.
func validNPMPrerelease(pre string) bool {
	for _, identifier := range strings.Split(pre, ".") {
		if identifier == "" {
			return false
		}
		numeric := true
		for i := 0; i < len(identifier); i++ {
			c := identifier[i]
			switch {
			case c >= '0' && c <= '9':
			case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '-':
				numeric = false
			default:
				return false
			}
		}
		if numeric && len(identifier) > 1 && identifier[0] == '0' {
			return false
		}
	}
	return true
}

// npmVersionBounds is one version interval. The zero value matches
// everything; never matches nothing.
type npmVersionBounds struct {
	lower          npmSemver
	hasLower       bool
	lowerInclusive bool
	upper          npmSemver
	hasUpper       bool
	upperInclusive bool
	never          bool
}

// contains reports whether actual falls within the bounds.
func (b npmVersionBounds) contains(actual npmSemver) bool {
	if b.never {
		return false
	}
	if b.hasLower {
		c := actual.compare(b.lower)
		if c < 0 || (c == 0 && !b.lowerInclusive) {
			return false
		}
	}
	if b.hasUpper {
		c := actual.compare(b.upper)
		if c > 0 || (c == 0 && !b.upperInclusive) {
			return false
		}
	}
	return true
}

// npmRangeArmMatches evaluates one ||-separated range arm: a hyphen range, or
// a set of comparators that must all hold. The second return is false when
// the arm is unparseable.
func npmRangeArmMatches(arm string, actual npmSemver) (bool, bool) {
	fields := strings.Fields(arm)
	if len(fields) == 0 {
		return false, false
	}
	hyphen := -1
	for i, field := range fields {
		if field == "-" {
			if hyphen >= 0 {
				return false, false
			}
			hyphen = i
		}
	}
	if hyphen >= 0 {
		if len(fields) != 3 || hyphen != 1 {
			return false, false
		}
		bounds, ok := npmHyphenBounds(fields[0], fields[2])
		if !ok {
			return false, false
		}
		return bounds.contains(actual), true
	}
	var tokens []string
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		if !isNPMRangeLoneOperator(field) {
			tokens = append(tokens, field)
			continue
		}
		if i+1 >= len(fields) || isNPMRangeOperatorByte(fields[i+1][0]) {
			return false, false
		}
		tokens = append(tokens, field+fields[i+1])
		i++
	}
	for _, token := range tokens {
		bounds, ok := npmComparatorBounds(token)
		if !ok || !bounds.contains(actual) {
			return false, ok
		}
	}
	return true, true
}

// isNPMRangeLoneOperator reports whether a range field is a standalone
// operator, which binds to the version token that follows it.
func isNPMRangeLoneOperator(field string) bool {
	switch field {
	case "<", "<=", ">", ">=", "=", "~", "^":
		return true
	}
	return false
}

// isNPMRangeOperatorByte reports whether c opens a range operator.
func isNPMRangeOperatorByte(c byte) bool {
	return c == '<' || c == '>' || c == '=' || c == '~' || c == '^'
}

// npmComparatorBounds desugars one comparator token into an interval.
func npmComparatorBounds(token string) (npmVersionBounds, bool) {
	if token == "*" || token == "x" || token == "X" {
		return npmVersionBounds{}, true
	}
	if strings.HasPrefix(token, "~") {
		partial, ok := parseNPMPartial(token[1:])
		if !ok {
			return npmVersionBounds{}, false
		}
		return npmTildeBounds(partial), true
	}
	if strings.HasPrefix(token, "^") {
		partial, ok := parseNPMPartial(token[1:])
		if !ok {
			return npmVersionBounds{}, false
		}
		return npmCaretBounds(partial), true
	}
	op := "="
	rest := token
	for _, prefix := range []string{">=", "<=", ">", "<", "="} {
		if strings.HasPrefix(rest, prefix) {
			op = prefix
			rest = rest[len(prefix):]
			break
		}
	}
	if rest == "" || isNPMRangeOperatorByte(rest[0]) {
		return npmVersionBounds{}, false
	}
	partial, ok := parseNPMPartial(rest)
	if !ok {
		return npmVersionBounds{}, false
	}
	return npmOperatorBounds(op, partial), true
}

// npmOperatorBounds applies node-semver partial completion to a comparison
// operator: missing components complete the bound, so <=1.2 means <1.3.0 and
// >1.2 means >=1.3.0. A bare partial is equality with the same completion.
func npmOperatorBounds(op string, partial npmPartial) npmVersionBounds {
	if partial.wildcard() {
		switch op {
		case "=", ">=", "<=":
			return npmVersionBounds{}
		default:
			return npmVersionBounds{never: true}
		}
	}
	zero := partial.zeroFilled()
	switch op {
	case "=":
		if partial.full() {
			return npmVersionBounds{lower: zero, hasLower: true, lowerInclusive: true, upper: zero, hasUpper: true, upperInclusive: true}
		}
		return npmVersionBounds{lower: zero, hasLower: true, lowerInclusive: true, upper: partial.incrementLastSpecified(), hasUpper: true}
	case ">=":
		return npmVersionBounds{lower: zero, hasLower: true, lowerInclusive: true}
	case ">":
		if partial.full() {
			return npmVersionBounds{lower: zero, hasLower: true}
		}
		return npmVersionBounds{lower: partial.incrementLastSpecified(), hasLower: true, lowerInclusive: true}
	case "<":
		return npmVersionBounds{upper: zero, hasUpper: true}
	case "<=":
		if partial.full() {
			return npmVersionBounds{upper: zero, hasUpper: true, upperInclusive: true}
		}
		return npmVersionBounds{upper: partial.incrementLastSpecified(), hasUpper: true}
	}
	return npmVersionBounds{never: true}
}

// npmTildeBounds desugars ~: patch-level changes when a minor is given,
// minor-level otherwise.
func npmTildeBounds(partial npmPartial) npmVersionBounds {
	if partial.wildcard() {
		return npmVersionBounds{}
	}
	bounds := npmVersionBounds{lower: partial.zeroFilled(), hasLower: true, lowerInclusive: true, hasUpper: true}
	upper := partial.nums
	if partial.specified[1] {
		upper[1]++
		upper[2] = 0
	} else {
		upper[0]++
		upper[1] = 0
		upper[2] = 0
	}
	bounds.upper = npmSemver{major: upper[0], minor: upper[1], patch: upper[2]}
	return bounds
}

// npmCaretBounds desugars ^: the upper bound changes the leftmost non-zero
// given component, or the last given component when all given components are
// zero.
func npmCaretBounds(partial npmPartial) npmVersionBounds {
	if partial.wildcard() {
		return npmVersionBounds{}
	}
	var upper [3]uint64
	switch {
	case partial.nums[0] > 0:
		upper = [3]uint64{partial.nums[0] + 1, 0, 0}
	case partial.specified[1] && partial.nums[1] > 0:
		upper = [3]uint64{partial.nums[0], partial.nums[1] + 1, 0}
	case partial.specified[2] && partial.nums[2] > 0:
		upper = [3]uint64{partial.nums[0], partial.nums[1], partial.nums[2] + 1}
	case partial.specified[2]:
		upper = [3]uint64{partial.nums[0], partial.nums[1], 1}
	case partial.specified[1]:
		upper = [3]uint64{partial.nums[0], 1, 0}
	default:
		upper = [3]uint64{1, 0, 0}
	}
	return npmVersionBounds{
		lower: partial.zeroFilled(), hasLower: true, lowerInclusive: true,
		upper: npmSemver{major: upper[0], minor: upper[1], patch: upper[2]}, hasUpper: true,
	}
}

// npmHyphenBounds desugars a hyphen range: the lower partial completes with
// zeros inclusively; the upper is inclusive when fully given and otherwise
// completes to its next version exclusively. Wildcards are not valid hyphen
// endpoints.
func npmHyphenBounds(left, right string) (npmVersionBounds, bool) {
	lower, ok := parseNPMPartial(left)
	if !ok {
		return npmVersionBounds{}, false
	}
	upper, ok := parseNPMPartial(right)
	if !ok {
		return npmVersionBounds{}, false
	}
	if lower.wildcard() || upper.wildcard() {
		return npmVersionBounds{}, false
	}
	bounds := npmVersionBounds{lower: lower.zeroFilled(), hasLower: true, lowerInclusive: true, hasUpper: true}
	if upper.full() {
		bounds.upper = upper.zeroFilled()
		bounds.upperInclusive = true
	} else {
		bounds.upper = upper.incrementLastSpecified()
	}
	return bounds, true
}
