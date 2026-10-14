package toolchainpins

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// DriftInputs names the disk records and specifications compared by the CI check.
type DriftInputs struct {
	PublishPin  string
	PairRecord  string
	PublishSpec string
	BuildSpec   string
}

// DriftError indicates that disk pin records and specification tables cannot agree.
type DriftError struct{ message string }

func (err *DriftError) Error() string { return err.message }

// DiagnosticID identifies the registered pin parity failure.
func (*DriftError) DiagnosticID() string { return "windlass.verify.error.toolchain-pin-drift" }

// CheckDrift compares disk state, independently of the executable's embedded pins.
func CheckDrift(inputs DriftInputs) error {
	publishData, err := os.ReadFile(inputs.PublishPin)
	if err != nil {
		return &DriftError{message: "cannot read publish pin record"}
	}
	pin, err := decodePublishPin(publishData)
	if err != nil {
		return &DriftError{message: "invalid publish pin record"}
	}
	pairData, err := os.ReadFile(inputs.PairRecord)
	if err != nil {
		return &DriftError{message: "cannot read runtime pair record"}
	}
	pair, err := decodeNodeRuntimePair(pairData)
	if err != nil {
		return &DriftError{message: "invalid runtime pair record"}
	}
	var platform string
	var artifact NodeArtifact
	for key, value := range pair.Artifacts {
		platform, artifact = key, value
	}
	checks := []struct {
		path, heading   string
		headers, values []string
	}{
		{inputs.PublishSpec, "## Publish-stage npm CLI allowlist",
			[]string{"Version", "Tarball URL", "SHA-512 (SRI)"},
			[]string{pin.Version, pin.TarballURL, pin.Integrity}},
		{inputs.BuildSpec, "## Node.js and npm toolchain pair",
			[]string{"Node.js version", "Bundled npm version"},
			[]string{pair.NodeVersion, pair.NPMVersion}},
		{inputs.BuildSpec, "## Node.js and npm toolchain pair",
			[]string{"Platform", "Tarball URL", "SHA-256 (hex)"},
			[]string{platform, artifact.URL, artifact.SHA256}},
	}
	for _, check := range checks {
		data, err := os.ReadFile(check.path)
		if err != nil {
			return &DriftError{message: "cannot read specification for " + check.heading}
		}
		section, err := pinSection(string(data), check.heading)
		if err != nil {
			return err
		}
		if err := checkPinTable(section, check.headers, check.values); err != nil {
			return fmt.Errorf("%s: %w", check.heading, err)
		}
	}
	return nil
}

func pinSection(markdown, heading string) ([]string, error) {
	var section []string
	found, active := false, false
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if line == heading {
			if found {
				return nil, &DriftError{message: "duplicate specification section"}
			}
			found, active = true, true
			continue
		}
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ") {
			active = false
		}
		if active {
			section = append(section, line)
		}
	}
	if !found {
		return nil, &DriftError{message: "missing specification section " + heading}
	}
	return section, nil
}

func checkPinTable(lines, headers, values []string) error {
	tables := 0
	for index := 0; index < len(lines); index++ {
		if !slices.Equal(pinTableRow(lines[index]), headers) {
			continue
		}
		tables++
		if index+1 >= len(lines) || !pinTableSeparator(pinTableRow(lines[index+1]), len(headers)) {
			return &DriftError{message: "invalid pin table separator"}
		}
		index += 2
		rows := 0
		for ; index < len(lines) && strings.HasPrefix(lines[index], "|"); index++ {
			row := pinTableRow(lines[index])
			if len(row) != len(headers) {
				return &DriftError{message: "unparsable pin table row"}
			}
			if !slices.Equal(row, values) {
				return &DriftError{message: "pin table values differ from disk record"}
			}
			rows++
		}
		if rows != 1 {
			return &DriftError{message: "pin table must contain exactly one data row"}
		}
	}
	if tables != 1 {
		return &DriftError{message: "specification must contain exactly one pin table"}
	}
	return nil
}

func pinTableRow(line string) []string {
	line = strings.TrimSpace(line)
	if len(line) < 2 || !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
		return nil
	}
	cells := strings.Split(line[1:len(line)-1], "|")
	for index, cell := range cells {
		cell = strings.TrimSpace(cell)
		if len(cell) >= 2 && strings.HasPrefix(cell, "`") && strings.HasSuffix(cell, "`") {
			cell = cell[1 : len(cell)-1]
		}
		cells[index] = cell
	}
	return cells
}

func pinTableSeparator(cells []string, columns int) bool {
	if len(cells) != columns {
		return false
	}
	for _, cell := range cells {
		cell = strings.TrimSuffix(strings.TrimPrefix(cell, ":"), ":")
		if len(cell) < 3 || strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return true
}
