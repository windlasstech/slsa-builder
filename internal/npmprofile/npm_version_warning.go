package npmprofile

import "github.com/windlasstech/slsa-builder/internal/diagnostic"

// npmDeclaredVersionWarning implements ADR 0095: a declared npm descriptor
// version is accepted but non-authoritative. When it provably matches the
// observed toolchain npm there is nothing to report; otherwise the build
// carries one npm-version-mismatch warning and never fails on this account.
func npmDeclaredVersionWarning(manager ManagerSelection, observedNPMVersion string) ([]diagnostic.Diagnostic, error) {
	if manager.Name != ManagerNPM || manager.DeclaredVersion == "" || npmDeclaredMatchesActual(manager.DeclaredVersion, observedNPMVersion) {
		return nil, nil
	}
	warning, err := diagnostic.New(IDNPMVersionMismatch, "package_manager.version", "The declared npm version does not match the toolchain npm; the toolchain npm remains authoritative.")
	if err != nil {
		return nil, err
	}
	warning.Field = "package_manager.version"
	warning.Actual = diagnostic.JSONValue(manager.DeclaredVersion)
	return []diagnostic.Diagnostic{warning}, nil
}
