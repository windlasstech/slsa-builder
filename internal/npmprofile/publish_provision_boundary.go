package npmprofile

import (
	"os"
	"path/filepath"
	"strings"
)

func publishProvisionedShimRoot(executable string) string {
	runnerTemp := os.Getenv("RUNNER_TEMP")
	if !filepath.IsAbs(runnerTemp) || !filepath.IsAbs(executable) || filepath.Clean(executable) != executable || filepath.Base(executable) != "npm" {
		return ""
	}
	runnerTemp, err := filepath.EvalSymlinks(runnerTemp)
	if err != nil {
		return ""
	}
	private := filepath.Dir(executable)
	suffix, ok := strings.CutPrefix(filepath.Base(private), "npm-provision-")
	if !ok || suffix == "" || filepath.Dir(private) != filepath.Join(runnerTemp, "windlass-npm-cli") {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil || resolved != executable {
		return ""
	}
	for _, name := range []string{private, executable} {
		info, err := os.Lstat(name)
		if err != nil || info.Mode().Perm() != 0o700 {
			return ""
		}
		if name == private && !info.IsDir() || name == executable && !info.Mode().IsRegular() {
			return ""
		}
	}
	// Only this invocation gains the workflow-owned private root; build commands do not.
	return private
}
