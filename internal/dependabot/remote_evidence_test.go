package dependabot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRemoteRejectsUnavailableModuleBeforeGeneration(t *testing.T) {
	t.Parallel()
	// Given syntactically valid PR checksums with no authenticated module objects.
	f := newRemoteFixture(t)
	var generated atomic.Int64
	f.remote.planner.Generate = func(context.Context, Generation) ([]Change, error) {
		generated.Add(1)
		return nil, ErrInput
	}
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.Contains(r.URL.Path, "/git/commits/") {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		return false
	}
	// When remote evidence cannot be authenticated.
	_, err := f.remote.Plan(t.Context(), f.inputs)
	// Then generation and mutation are unreachable.
	if !errors.Is(err, ErrGitHub) || generated.Load() != 0 || f.mutations.Load() != 0 {
		t.Fatalf("err=%v generated=%d mutations=%d", err, generated.Load(), f.mutations.Load())
	}
}

func TestRemotePreimagePreservesFetchFailure(t *testing.T) {
	t.Parallel()
	// Given an unavailable preimage, not a fetched digest mismatch.
	api := testAPI(t, func(w http.ResponseWriter, r *http.Request) { _ = r; w.WriteHeader(503) })
	plan := Plan{HeadSHA: strings.Repeat("b", 40), Files: []FileHash{{Path: "go.mod", SHA256: hash(nil)}}}
	// When checking preimages.
	err := api.checkPreimages(t.Context(), plan)
	// Then transport evidence retains its class.
	if !errors.Is(err, ErrGitHub) || errors.Is(err, ErrInput) {
		t.Fatalf("err=%v", err)
	}
}

func TestRemoteMutationDeterministicRejection(t *testing.T) {
	t.Parallel()
	for _, status := range []int{400, 401, 403, 404, 422} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			// Given a deterministic HTTP rejection of the sole mutation.
			var calls atomic.Int64
			api := testAPI(t, func(w http.ResponseWriter, r *http.Request) { _ = r; calls.Add(1); w.WriteHeader(status) })
			// When submitting the mutation.
			err := api.commitAndReconcile(t.Context(), commitInput{})
			// Then it is not reported as a possibly successful mutation.
			if !errors.Is(err, ErrGitHub) || errors.Is(err, ErrIndeterminate) || calls.Load() != 1 {
				t.Fatalf("err=%v calls=%d", err, calls.Load())
			}
		})
	}
}

func TestReleaseRequiresExplicitFlags(t *testing.T) {
	t.Parallel()
	for _, flags := range []string{``, `,"draft":false`, `,"prerelease":false`, `,"draft":null,"prerelease":false`, `,"draft":false,"prerelease":null`, `,"draft":true,"prerelease":false`, `,"draft":false,"prerelease":true`} {
		t.Run(flags, func(t *testing.T) {
			// Given published metadata with absent or disqualifying flags.
			api := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/releases/") {
					fmt.Fprintf(w, `{"tag_name":"v1.999.0","published_at":"2026-11-10T00:00:00Z"%s}`, flags)
					return
				}
				fmt.Fprintf(w, `{"ref":"refs/tags/v1.999.0","object":{"type":"commit","sha":%q}}`, strings.Repeat("c", 40))
			})
			// When resolving release evidence.
			_, err := api.resolveRelease(t.Context(), "v1.999.0")
			// Then missing/null flags cannot masquerade as false.
			if !errors.Is(err, ErrInput) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
