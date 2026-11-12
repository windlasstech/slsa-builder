package command

import (
	"io"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func writePublishInputRejection(out io.Writer, reportPath string, cause error) error {
	id := diagnosticIDOf(cause)
	if id == "" {
		id = npmprofile.IDPackageManifestEvidence
	}
	entry, err := diagnostic.New(id, "npm.publish.metadata", RedactSecrets(cause.Error()))
	if err != nil {
		return err
	}
	report, err := diagnostic.Build(nil, []diagnostic.Diagnostic{entry}, nil)
	if err != nil {
		return err
	}
	encoded, err := report.CanonicalJSON()
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(reportPath, encoded, 0o600); err != nil {
		return err
	}
	if err := WriteReport(out, report); err != nil {
		return err
	}
	return ErrVerificationFailure
}
