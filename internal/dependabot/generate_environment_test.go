package dependabot

import (
	"strings"
	"testing"
)

func TestTrustedEnvironmentOmitsAmbientProxies(t *testing.T) {
	// Given distinct ambient proxy values, including lowercase alternatives.
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "all_proxy", "no_proxy"} {
		t.Setenv(name, "ambient-"+name+".invalid")
	}

	// When constructing the environment used by trusted compilation and generation.
	env := trustedEnvironment()

	// Then no proxy configuration crosses the trusted subprocess boundary.
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(name) {
		case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY":
			t.Errorf("trusted environment inherited %s", name)
		}
	}
}
