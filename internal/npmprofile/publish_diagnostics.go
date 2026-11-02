package npmprofile

import "strings"

const (
	maxNPMFailureCodeLength            = 64
	maxNPMFailurePredicatePathLength   = 256
	maxNPMFailureSummaryLength         = 8192
	maxNPMFailureRecognizerState       = 4096
	maxNPMFailureHTTPStatusLength      = 3
	maxNPMFailurePendingLineLength     = len("npm error ") + maxNPMFailureHTTPStatusLength + 1 + maxNPMFailurePredicatePathLength + 1
	maxNPMFailurePredicatePathsState   = maxNPMFailureRecognizerState - maxNPMFailurePendingLineLength - maxNPMFailureCodeLength - maxNPMFailureHTTPStatusLength
	npmFailureOperationalSummaryPrefix = "windlass operational: npm_failure"
)

// npmFailureRecognizer retains only bounded, allowlisted observations from npm output.
type npmFailureRecognizer struct {
	pending            []byte
	predicatePaths     []string
	code               string
	httpStatus         string
	predicatePathsSize int
	discarding         bool
}

func newNPMFailureRecognizer() *npmFailureRecognizer {
	return &npmFailureRecognizer{pending: make([]byte, 0, maxNPMFailurePendingLineLength)}
}

// Write consumes every byte and never surfaces an npm diagnostic error to the command.
func (r *npmFailureRecognizer) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\n' {
			r.observeLine(string(r.pending), !r.discarding)
			r.pending = r.pending[:0]
			r.discarding = false
			continue
		}
		if r.discarding {
			continue
		}
		if len(r.pending) == maxNPMFailurePendingLineLength {
			r.discarding = true
			continue
		}
		r.pending = append(r.pending, b)
	}
	return len(p), nil
}

// Summary also observes an unterminated final line without changing stream state.
func (r *npmFailureRecognizer) Summary() string {
	snapshot := &npmFailureRecognizer{
		predicatePaths:     append([]string(nil), r.predicatePaths...),
		code:               r.code,
		httpStatus:         r.httpStatus,
		predicatePathsSize: r.predicatePathsSize,
	}
	snapshot.observeLine(string(r.pending), !r.discarding)
	return snapshot.summary()
}

// StateSize reports variable retained bytes; fixed-size scalar storage is excluded.
func (r *npmFailureRecognizer) StateSize() int {
	return len(r.pending) + len(r.code) + len(r.httpStatus) + r.predicatePathsSize
}

func (r *npmFailureRecognizer) observeLine(line string, complete bool) {
	if complete {
		line = strings.TrimSuffix(line, "\r")
	}
	var rest string
	switch {
	case strings.HasPrefix(line, "npm error "):
		rest = line[len("npm error "):]
	case strings.HasPrefix(line, "npm err! "):
		rest = line[len("npm err! "):]
	default:
		return
	}

	if strings.HasPrefix(rest, "code ") {
		code := rest[len("code "):]
		if complete && r.code == "" && validNPMFailureCode(code) {
			r.code = strings.Clone(code)
		}
		return
	}
	if len(rest) < maxNPMFailureHTTPStatusLength || !npmFailureDigits(rest[:maxNPMFailureHTTPStatusLength]) {
		return
	}
	status := rest[:maxNPMFailureHTTPStatusLength]
	if status < "400" || status > "599" {
		return
	}
	if len(rest) > maxNPMFailureHTTPStatusLength && rest[maxNPMFailureHTTPStatusLength] != ' ' {
		return
	}
	if r.httpStatus == "" {
		r.httpStatus = strings.Clone(status)
	}
	if len(rest) == maxNPMFailureHTTPStatusLength {
		return
	}

	field := rest[maxNPMFailureHTTPStatusLength+1:]
	end := len(field)
	delimited := false
	for i := 0; i < len(field); i++ {
		switch field[i] {
		case ' ', '\t', '=':
			end = i
			delimited = true
		}
		if delimited {
			break
		}
	}
	if delimited || complete {
		r.addPredicatePath(field[:end])
	}
}

func (r *npmFailureRecognizer) addPredicatePath(path string) {
	if !validNPMFailurePredicatePath(path) {
		return
	}
	for _, existing := range r.predicatePaths {
		if existing == path {
			return
		}
	}
	if r.predicatePathsSize+len(path) > maxNPMFailurePredicatePathsState {
		return
	}
	r.predicatePaths = append(r.predicatePaths, strings.Clone(path))
	r.predicatePathsSize += len(path)
}

func (r *npmFailureRecognizer) summary() string {
	if r.code == "" && r.httpStatus == "" && len(r.predicatePaths) == 0 {
		return ""
	}
	var summary strings.Builder
	summary.WriteString(npmFailureOperationalSummaryPrefix)
	if r.code != "" {
		summary.WriteString(" code=")
		summary.WriteString(r.code)
	}
	if r.httpStatus != "" {
		summary.WriteString(" http_status=")
		summary.WriteString(r.httpStatus)
	}
	if len(r.predicatePaths) > 0 {
		summary.WriteString(" predicate_paths=")
		summary.WriteString(strings.Join(r.predicatePaths, ","))
	}
	return summary.String()
}

func validNPMFailureCode(code string) bool {
	if len(code) == 0 || len(code) > maxNPMFailureCodeLength || code[0] < 'A' || code[0] > 'Z' {
		return false
	}
	for i := 1; i < len(code); i++ {
		if (code[i] < 'A' || code[i] > 'Z') && (code[i] < '0' || code[i] > '9') && code[i] != '_' {
			return false
		}
	}
	return true
}

func validNPMFailurePredicatePath(path string) bool {
	if len(path) == 0 || len(path) > maxNPMFailurePredicatePathLength {
		return false
	}
	root := ""
	for _, candidate := range [...]string{
		"predicate.buildDefinition.buildType",
		"predicate.buildDefinition.externalParameters",
		"predicate.buildDefinition.resolvedDependencies",
		"predicate.runDetails.builder",
		"predicate.runDetails.metadata",
	} {
		if path == candidate {
			return true
		}
		if len(path) > len(candidate) && strings.HasPrefix(path, candidate) &&
			(path[len(candidate)] == '.' || path[len(candidate)] == '[') {
			root = candidate
			break
		}
	}
	if root == "" {
		return false
	}
	for i := len(root); i < len(path); {
		switch path[i] {
		case '.':
			i++
			start := i
			for i < len(path) && npmFailurePathFieldByte(path[i]) {
				i++
			}
			if i == start || !npmFailurePathFieldStart(path[start]) {
				return false
			}
		case '[':
			i++
			start := i
			for i < len(path) && path[i] >= '0' && path[i] <= '9' {
				i++
			}
			if i == start || i == len(path) || path[i] != ']' {
				return false
			}
			i++
		default:
			return false
		}
	}
	return true
}

func npmFailureDigits(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func npmFailurePathFieldStart(b byte) bool {
	return b == '_' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
}

func npmFailurePathFieldByte(b byte) bool {
	return npmFailurePathFieldStart(b) || b >= '0' && b <= '9'
}
