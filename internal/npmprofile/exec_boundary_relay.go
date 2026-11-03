package npmprofile

import (
	"context"
	"io"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	maxOperationalStderrBytes = 65536
	maxOperationalStderrLines = 512
)

// operationalStderrRelay retains only bounded filtered text and an undecided secret
// prefix. It never calls the operational writer while the child is running.
type operationalStderrRelay struct {
	secrets           []string
	pending           []byte
	utf8Pending       []byte
	payload           strings.Builder
	bytes, lines      int
	inLine, truncated bool
}

func newOperationalStderrRelay(values []string) *operationalStderrRelay {
	r := &operationalStderrRelay{}
	for _, value := range values {
		if value != "" && !strings.ContainsAny(value, "\r\n") {
			r.secrets = append(r.secrets, value)
		}
	}
	return r
}

func (r *operationalStderrRelay) Write(p []byte) (int, error) {
	for _, b := range p {
		if r.truncated {
			break
		}
		r.pending = append(r.pending, b)
		r.redact(false)
	}
	return len(p), nil
}

// Wait while the raw suffix could extend to a secret; longest matches win at
// the same position. Replacement bytes bypass matching, so markers are not redacted.
func (r *operationalStderrRelay) redact(final bool) {
	for len(r.pending) > 0 && !r.truncated {
		matched, partial := 0, false
		for _, secret := range r.secrets {
			if len(r.pending) < len(secret) {
				partial = partial || strings.HasPrefix(secret, string(r.pending))
			} else if strings.HasPrefix(string(r.pending), secret) {
				matched = max(matched, len(secret))
			}
		}
		if partial && !final {
			return
		}
		if matched > 0 {
			for i := range len(redactedSecret) {
				r.decode(redactedSecret[i], false)
			}
			r.pending = r.pending[matched:]
		} else {
			r.decode(r.pending[0], false)
			r.pending = r.pending[1:]
		}
	}
}

func (r *operationalStderrRelay) decode(b byte, final bool) {
	if !final {
		r.utf8Pending = append(r.utf8Pending, b)
	}
	for len(r.utf8Pending) > 0 && (final || utf8.FullRune(r.utf8Pending)) {
		value, size := utf8.DecodeRune(r.utf8Pending)
		r.utf8Pending = r.utf8Pending[size:]
		r.collect(value)
	}
}

func (r *operationalStderrRelay) collect(value rune) {
	if r.truncated || value < 32 && value != '\n' && value != '\t' ||
		value >= 127 && value <= 159 || value == '\u200e' || value == '\u200f' ||
		value >= '\u202a' && value <= '\u202e' || value >= '\u2066' && value <= '\u2069' {
		return
	}
	if value == '\n' {
		if r.inLine {
			r.payload.WriteByte('\n')
			r.inLine = false
		}
		return
	}
	size := utf8.RuneLen(value)
	if r.bytes+size > maxOperationalStderrBytes || !r.inLine && r.lines == maxOperationalStderrLines {
		r.truncated = true
		return
	}
	if !r.inLine {
		r.lines++
		r.inLine = true
	}
	r.payload.WriteRune(value)
	r.bytes += size
}

func (r *operationalStderrRelay) emit(ctx context.Context, operation string) {
	r.redact(true)
	r.decode(0, true)
	for line := range strings.SplitSeq(r.payload.String(), "\n") {
		if line != "" {
			writeOperationalDebug(ctx, "operation="+operation+" stderr: "+line)
		}
	}
	if r.truncated {
		writeOperationalDebug(ctx, "operation="+operation+" stderr_truncated=true")
	}
}

// Both os/exec copy goroutines share this lock, including the unchanged recognizer.
type lockedCommandWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *lockedCommandWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}
