package dependabot_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var errChildNetwork = errors.New("child attempted network request")

func runOfflineChild(t *testing.T, cmd *exec.Cmd) ([]byte, error) {
	t.Helper()
	var requests atomic.Int64
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r
		requests.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer proxy.Close()
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = nil
	for _, value := range env {
		key, _, _ := strings.Cut(value, "=")
		switch strings.ToUpper(key) {
		case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "GOPROXY", "GOSUMDB", "GOTOOLCHAIN", "GOVCS", "GOWORK", "GOENV", "GIT_OPTIONAL_LOCKS":
		default:
			cmd.Env = append(cmd.Env, value)
		}
	}
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		cmd.Env = append(cmd.Env, key+"="+proxy.URL)
	}
	cmd.Env = append(cmd.Env, "NO_PROXY=", "no_proxy=", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOVCS=*:off", "GOWORK=off", "GOENV=off", "GIT_OPTIONAL_LOCKS=0")
	cmd.WaitDelay = time.Second
	output, err := cmd.CombinedOutput()
	if requests.Load() != 0 {
		return output, errChildNetwork
	}
	return output, err
}

func TestOfflineChildGuardRejectsIgnoredNetworkFailure(t *testing.T) {
	t.Parallel()
	for _, scheme := range []string{"http", "https"} {
		t.Run(scheme, func(t *testing.T) {
			t.Parallel()
			// Given a child that ignores network failures and inherited proxy bypasses.
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestOfflineChildProbe$")
			cmd.Env = append(os.Environ(), "COMPANION_OFFLINE_PROBE="+scheme, "NO_PROXY=*", "no_proxy=*")
			// When the child requests a remote URL, then the parent fails despite exit zero.
			_, err := runOfflineChild(t, cmd)
			if !errors.Is(err, errChildNetwork) {
				t.Fatalf("guard error=%v", err)
			}
		})
	}
}

func TestOfflineChildProbe(t *testing.T) {
	t.Parallel()
	scheme := os.Getenv("COMPANION_OFFLINE_PROBE")
	if scheme == "" {
		return
	}
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(scheme + "://offline-probe.invalid/never-forward")
	if err == nil {
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
