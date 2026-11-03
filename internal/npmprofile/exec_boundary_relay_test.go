package npmprofile

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestOperationalStderrRelayFilter(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, input, want string
		secrets           []string
		truncated         bool
	}{
		{name: "empty"},
		{name: "empty lines and final line", input: "\nfirst\n\nlast", want: "first\nlast\n"},
		{name: "controls", input: "a\r\x00\x1b[31m\x7fb\t\u0080\u009f\u061c\u200e\u200f\u202a\u202e\u2066\u2069c\n", want: "a[31mb\tc\n"},
		{name: "invalid UTF8", input: "a\xff\xc0\xe2\x82", want: "a����\n"},
		{name: "unicode", input: "한글🙂\n", want: "한글🙂\n"},
		{name: "raw redaction first", input: "x a\x1bb a\x1bb y", secrets: []string{"a\x1bb"}, want: "x [REDACTED] [REDACTED] y\n"},
		{name: "multiple secrets", input: "abc ab xyz abc", secrets: []string{"ab", "abc", "xyz", "abc"}, want: "[REDACTED] [REDACTED] [REDACTED] [REDACTED]\n"},
		{name: "excluded secrets", input: "a\nb\rc", secrets: []string{"", "a\nb", "b\rc"}, want: "a\nbc\n"},
		{name: "partial secret at EOF", input: "toke", secrets: []string{"token"}, want: "toke\n"},
		{name: "exact bytes", input: strings.Repeat("x", 65536), want: strings.Repeat("x", 65536) + "\n"},
		{name: "byte overflow", input: strings.Repeat("x", 65537), want: strings.Repeat("x", 65536) + "\n", truncated: true},
		{name: "UTF8 bound", input: strings.Repeat("x", 65535) + "🙂tail", want: strings.Repeat("x", 65535) + "\n", truncated: true},
		{name: "exact lines", input: strings.Repeat("x\n", 512), want: strings.Repeat("x\n", 512)},
		{name: "line overflow", input: strings.Repeat("x\n", 513), want: strings.Repeat("x\n", 512), truncated: true},
		{name: "empty lines do not consume budget", input: strings.Repeat("\x00\n", 700) + "end", want: "end\n"},
		{name: "stripping before bound", input: strings.Repeat("\x00", 70000) + "end", want: "end\n"},
		{name: "redaction before bound", input: strings.Repeat("long-secret-value", 5000), secrets: []string{"long-secret-value"}, want: strings.Repeat("[REDACTED]", 5000) + "\n"},
		{name: "replacement expansion", input: strings.Repeat("s", 7000), secrets: []string{"s"}, want: strings.Repeat("[REDACTED]", 6553) + "[REDAC\n", truncated: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for _, chunk := range []int{1, 3, 4096, len(test.input) + 1} {
				got := relayForTest(t, test.input, test.secrets, chunk)
				want := ""
				for line := range strings.SplitSeq(strings.TrimSuffix(test.want, "\n"), "\n") {
					if line != "" {
						want += "windlass operational: operation=npm stderr: " + line + "\n"
					}
				}
				if test.truncated {
					want += "windlass operational: operation=npm stderr_truncated=true\n"
				}
				if got != want {
					t.Fatalf("chunk %d: relay differs (got %d bytes, want %d); got prefix %q", chunk, len(got), len(want), got[:min(len(got), 200)])
				}
			}
		})
	}
}

func relayForTest(t *testing.T, input string, secrets []string, chunk int) string {
	t.Helper()
	filter := newOperationalStderrRelay(secrets)
	var output bytes.Buffer
	ctx := WithOperationalDebugOutput(t.Context(), &output)
	for offset := 0; offset < len(input); {
		end := min(offset+chunk, len(input))
		n, err := filter.Write([]byte(input[offset:end]))
		if n != end-offset || err != nil {
			t.Fatalf("Write = %d, %v", n, err)
		}
		maxSecret := 0
		for _, secret := range secrets {
			maxSecret = max(maxSecret, len(secret))
		}
		if len(filter.pending) > maxSecret || len(filter.utf8Pending) >= utf8.UTFMax || filter.payload.Len() > 65536+512 {
			t.Fatal("retained state exceeds secret lookahead and payload bounds")
		}
		offset = end
		if output.Len() != 0 {
			t.Fatal("relay emitted before completion")
		}
	}
	filter.emit(ctx, "npm")
	return output.String()
}

func FuzzOperationalStderrRelayFilter(f *testing.F) {
	f.Add([]byte("##[error]bad ##secret"), "secret", uint8(1))
	f.Add([]byte("secret\nhello\x1b\u202e🙂"), "secret", uint8(1))
	f.Add([]byte{0xff, 0xc2, 0x80}, "", uint8(2))
	f.Add([]byte(strings.Repeat("x\n", 513)), "x", uint8(255))
	f.Fuzz(func(t *testing.T, input []byte, secret string, chunk uint8) {
		got := relayForTest(t, string(input), []string{secret}, int(chunk)+1)
		if strings.Contains(got, "##[") {
			t.Fatal("runner command marker survived")
		}
		if got != referenceRelay(string(input), secret) {
			t.Fatal("streaming relay differs from whole-input redaction/filter/bounds")
		}
		if got != relayForTest(t, string(input), []string{secret}, len(input)+1) {
			t.Fatal("chunk-dependent output")
		}
		payload, lines, markers := 0, 0, 0
		for line := range strings.SplitSeq(strings.TrimSuffix(got, "\n"), "\n") {
			if line == "" {
				continue
			}
			if line == "windlass operational: operation=npm stderr_truncated=true" {
				markers++
				continue
			}
			text, ok := strings.CutPrefix(line, "windlass operational: operation=npm stderr: ")
			if !ok || text == "" || !utf8.ValidString(text) {
				t.Fatal("invalid relay line")
			}
			payload += len(text)
			lines++
			for _, r := range text {
				if r < 32 && r != '\t' || r >= 127 && r <= 159 || r == '\u061c' || r == '\u200e' || r == '\u200f' || r >= '\u202a' && r <= '\u202e' || r >= '\u2066' && r <= '\u2069' {
					t.Fatal("prohibited control")
				}
			}
		}
		if payload > 65536 || lines > 512 || markers > 1 {
			t.Fatal("relay exceeds bounds")
		}
		// An isolated exact secret must become the marker even when split across writes.
		// Arbitrary payloads can reconstruct a secret after stripping, and the marker itself
		// can contain a short secret; neither is an unredacted raw-stream occurrence.
		if secret != "" && !strings.ContainsAny(secret, "\r\n") {
			redacted := relayForTest(t, secret, []string{secret}, int(chunk)+1)
			if redacted != "windlass operational: operation=npm stderr: [REDACTED]\n" {
				t.Fatal("raw secret not redacted")
			}
		}
	})
}

func referenceRelay(input, secret string) string {
	if secret != "" && !strings.ContainsAny(secret, "\r\n") {
		input = strings.ReplaceAll(input, secret, "[REDACTED]")
	}
	filtered := strings.Map(func(r rune) rune {
		switch {
		case r < 32 && r != '\n' && r != '\t', r >= 127 && r <= 159,
			r == '\u061c', r == '\u200e', r == '\u200f', r >= '\u202a' && r <= '\u202e', r >= '\u2066' && r <= '\u2069':
			return -1
		default:
			return r
		}
	}, input)
	for strings.Contains(filtered, "##[") {
		filtered = strings.ReplaceAll(filtered, "##[", "##")
	}
	var output strings.Builder
	remaining, count := 65536, 0
	for line := range strings.SplitSeq(filtered, "\n") {
		if line == "" {
			continue
		}
		truncated := count == 512 || len(line) > remaining
		if count < 512 {
			prefix := line[:min(len(line), remaining)]
			for !utf8.ValidString(prefix) {
				prefix = prefix[:len(prefix)-1]
			}
			if prefix != "" {
				output.WriteString("windlass operational: operation=npm stderr: " + prefix + "\n")
			}
			remaining -= len(prefix)
			count++
		}
		if truncated {
			output.WriteString("windlass operational: operation=npm stderr_truncated=true\n")
			break
		}
	}
	return output.String()
}
