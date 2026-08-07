package command

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

var npmProfileReportOutputs = NewOutputAllowlist("report-path", "result")

type npmProfileReportCommand struct{}

// NewNPMProfileReportCommand creates the always-run persistent report validator.
func NewNPMProfileReportCommand() Command { return npmProfileReportCommand{} }

func (npmProfileReportCommand) Name() string { return "npm-profile-report" }

func (npmProfileReportCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet("npm-profile-report", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	reportPath := flags.String("report-path", "", "persistent report path")
	githubOutput := flags.String("github-output", os.Getenv("GITHUB_OUTPUT"), "GitHub Actions output file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *reportPath == "" || *githubOutput == "" {
		return errors.New("--report-path and --github-output are required")
	}
	report, err := ReadTypedJSON[diagnostic.Report](*reportPath, nil)
	if err != nil {
		entry, diagnosticErr := diagnostic.New(diagnostic.IDInputUnavailable, "npm.publish.report", "publish outcome report is unavailable")
		if diagnosticErr != nil {
			return diagnosticErr
		}
		report, diagnosticErr = diagnostic.Build(nil, []diagnostic.Diagnostic{entry}, nil)
		if diagnosticErr != nil {
			return diagnosticErr
		}
	}
	encoded, err := report.CanonicalJSON()
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(*reportPath, encoded, 0o600); err != nil {
		return err
	}
	result := "pass"
	if report.PrimaryID != nil {
		result = "fail"
	}
	if err := WriteGitHubOutputs(*githubOutput, npmProfileReportOutputs, map[string]string{"report-path": *reportPath, "result": result}); err != nil {
		return err
	}
	return WriteReport(out, report)
}
