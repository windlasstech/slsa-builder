package npmprofile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkflowDistributionCaptureMatchesSelection(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(*BuildPackResult)
	}{
		{"distribution version", func(build *BuildPackResult) {
			build.Toolchain.Distribution.PackageManagerVer = "11.28.4"
			build.Toolchain.Distribution.URL = "https://registry.npmjs.org/pnpm/-/pnpm-11.28.4.tgz"
		}},
		{"executed version", func(build *BuildPackResult) { build.Toolchain.PackageManagerVersion = "11.28.4" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			selection, build, config := workflowIntentInputs(t)
			selection.Manager.Name, selection.Manager.Version = ManagerPNPM, "11.28.3"
			selection.Manager.SelectedLockfilePath = "pnpm-lock.yaml"
			if err := os.WriteFile(filepath.Join(selection.Package.RealManagerRoot, "pnpm-lock.yaml"), []byte("lockfileVersion: '9.0'\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			build.Toolchain.PackageManagerVersion = "11.28.3"
			build.Toolchain.CorepackVersion = "0.34.0"
			build.Toolchain.Distribution = &DistributionCapture{
				URL: "https://registry.npmjs.org/pnpm/-/pnpm-11.28.3.tgz", SHA512: testSHA512,
				PackageManager: ManagerPNPM, PackageManagerVer: "11.28.3", DigestAuthority: "registry-integrity", AcquisitionSource: "corepack",
			}
			if _, err := FinalizeWorkflowBuildMetadata(selection, build, config); err != nil {
				t.Fatalf("valid control: %v", err)
			}
			test.mutate(&build)
			_, err := FinalizeWorkflowBuildMetadata(selection, build, config)
			requireNPMDiagnostic(t, err, IDResolvedDependenciesDistribution)
		})
	}
}
