package command

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

func TestSelectionCommandAcceptsValidPackage(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs("../../testdata/npm/packages/npm-root-valid")
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = NewNPMProfileSelectCommand().Execute(t.Context(), []string{"--repository-root", root, "--package-directory", ".", "--observed-repository", "windlasstech/slsa-builder"}, &output)
	if err != nil {
		t.Fatalf("selection = %v, report %s", err, output.String())
	}
	report, err := decodeTypedJSON[diagnostic.Report](bytes.TrimSpace(output.Bytes()), nil)
	if err != nil || report.Result != "pass" || report.ExitCode != 0 || report.PrimaryID != nil {
		t.Fatalf("report = %#v, %v", report, err)
	}
}

func TestSelectionCommandPropagatesWriterFailure(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs("../../testdata/npm/packages/rejected/private-package")
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("selection report sink failed")
	err = NewNPMProfileSelectCommand().Execute(t.Context(), []string{"--repository-root", root, "--package-directory", ".", "--observed-repository", "windlasstech/slsa-builder"}, &stagedWriter{err: cause})
	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want writer cause", err)
	}
}

func TestSelectionAndFixtureRejectPositionals(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		command Command
		args    []string
	}{
		{NewNPMProfileSelectCommand(), []string{"--repository-root", ".", "--package-directory", ".", "--observed-repository", "windlasstech/slsa-builder", "unexpected"}},
		{NewFixtureCheckCommand(), []string{"--index", "unused", "unexpected"}},
	} {
		t.Run(test.command.Name(), func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			err := test.command.Execute(t.Context(), test.args, &output)
			if err == nil || err.Error() != "unexpected positional arguments: [unexpected]" || output.Len() != 0 {
				t.Fatalf("positional rejection = %v, %q", err, output.String())
			}
		})
	}
}

func TestWriteReportRejectsInvalidReportAndPropagatesNewlineFailure(t *testing.T) {
	t.Parallel()
	t.Run("invalid report", func(t *testing.T) {
		t.Parallel()
		var output bytes.Buffer
		if err := WriteReport(&output, Report{}); err == nil || output.Len() != 0 {
			t.Fatalf("invalid report error = %v, output %q", err, output.String())
		}
	})
	t.Run("newline write", func(t *testing.T) {
		t.Parallel()
		report, err := diagnostic.Build(nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		cause := errors.New("newline write failed")
		if err := WriteReport(&stagedWriter{remaining: 1, err: cause}, report); !errors.Is(err, cause) {
			t.Fatalf("error = %v, want newline writer cause", err)
		}
	})
}
