package workflowcheck

import "strings"

// toolchainInvocationText removes shell quote delimiters and shell-removable
// escapes for detection only. Parameter expansions and command substitutions are
// not evaluated: this is a lexical conformance scan, not a shell interpreter.
// Exact command and argument checks must continue to use the original text.
func toolchainInvocationText(run string) string {
	var result strings.Builder
	var quote byte
	for index := 0; index < len(run); index++ {
		char := run[index]
		if char == '\\' && quote != '\'' && index+1 < len(run) &&
			(quote == 0 || strings.ContainsRune("$`\"\\\n", rune(run[index+1]))) {
			index++
			if run[index] != '\n' {
				result.WriteByte(run[index])
			}
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				result.WriteByte(char)
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
		} else {
			result.WriteByte(char)
		}
	}
	return result.String()
}

// A final command terminator does not change the required command's argv.
func exactToolchainCommand(run string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(run), ";"))
}
