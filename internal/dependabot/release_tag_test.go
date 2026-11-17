package dependabot

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestRemoteReleaseRejectsMismatchedOrMovingTags(t *testing.T) {
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "true")
	for _, scenario := range []string{"candidate target mismatch", "old target mismatch", "candidate moved", "old moved", "wrong ref", "wrong tag object", "tag cycle", "tag depth", "unsupported object"} {
		t.Run(scenario, func(t *testing.T) {
			f := newRemoteFixture(t)
			reads := map[string]int{}
			f.override = func(w http.ResponseWriter, r *http.Request) bool {
				const prefix = "/repos/sigstore/sigstore-go/git/ref/tags/"
				if strings.HasPrefix(r.URL.Path, prefix) {
					reads[r.URL.Path]++
					candidate := strings.HasSuffix(r.URL.Path, "/v1.999.0")
					version := strings.TrimPrefix(r.URL.Path, prefix)
					sha, kind, ref := strings.Repeat("c", 40), "commit", "refs/tags/"+version
					switch {
					case scenario == "candidate target mismatch" && candidate, scenario == "old target mismatch" && !candidate:
						sha = strings.Repeat("e", 40)
					case scenario == "candidate moved" && candidate && reads[r.URL.Path] > 1, scenario == "old moved" && !candidate && reads[r.URL.Path] > 1:
						sha = strings.Repeat("e", 40)
					case scenario == "wrong ref":
						ref = "refs/heads/" + version
					case scenario == "wrong tag object", scenario == "tag cycle", scenario == "tag depth":
						kind = "tag"
					case scenario == "unsupported object":
						kind = "tree"
					default:
						return false
					}
					fmt.Fprintf(w, `{"ref":%q,"object":{"type":%q,"sha":%q}}`, ref, kind, sha)
					return true
				}
				if strings.Contains(r.URL.Path, "/git/tags/") {
					if scenario == "tag depth" {
						reads["tags"]++
						fmt.Fprintf(w, `{"sha":%q,"object":{"type":"tag","sha":"%040x"}}`, strings.TrimPrefix(r.URL.Path, "/repos/sigstore/sigstore-go/git/tags/"), reads["tags"])
						return true
					}
					sha := strings.Repeat("c", 40)
					if scenario == "wrong tag object" {
						sha = strings.Repeat("e", 40)
					}
					fmt.Fprintf(w, `{"sha":%q,"object":{"type":"tag","sha":%q}}`, sha, strings.Repeat("c", 40))
					return true
				}
				return false
			}
			_, err := f.remote.Apply(t.Context(), f.inputs, "companion")
			if err == nil || f.mutations.Load() != 0 {
				t.Fatalf("error=%v mutations=%d", err, f.mutations.Load())
			}
		})
	}
}

func TestRemoteReleasePeelsTagsAndReadsExactCommits(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"main", strings.Repeat("c", 40), ""} {
		t.Run("target-"+target, func(t *testing.T) {
			t.Parallel()
			f := newRemoteFixture(t)
			var tagReads, evidenceReads int
			licenseRefs := map[string]int{}
			objects := map[string]gitTarget{
				strings.Repeat("a", 40): {Type: "tag", SHA: strings.Repeat("b", 40)},
				strings.Repeat("b", 40): {Type: "commit", SHA: strings.Repeat("c", 40)},
				strings.Repeat("1", 40): {Type: "tag", SHA: strings.Repeat("2", 40)},
				strings.Repeat("2", 40): {Type: "commit", SHA: strings.Repeat("f", 40)},
			}
			f.override = func(w http.ResponseWriter, r *http.Request) bool {
				const upstream = "/repos/sigstore/sigstore-go"
				if !strings.HasPrefix(r.URL.Path, upstream) {
					return false
				}
				switch {
				case strings.Contains(r.URL.Path, "/releases/tags/"):
					version := strings.TrimPrefix(r.URL.Path, upstream+"/releases/tags/")
					commitish := target
					if version != "v1.999.0" && shaPattern.MatchString(commitish) {
						commitish = strings.Repeat("f", 40)
					}
					fmt.Fprintf(w, `{"tag_name":%q,"target_commitish":%q,"published_at":"2026-11-10T00:00:00Z","draft":false,"prerelease":false}`, version, commitish)
					return true
				case strings.Contains(r.URL.Path, "/git/ref/tags/"):
					version := strings.TrimPrefix(r.URL.Path, upstream+"/git/ref/tags/")
					object := strings.Repeat("a", 40)
					if version != "v1.999.0" {
						object = strings.Repeat("1", 40)
					}
					fmt.Fprintf(w, `{"ref":%q,"object":{"type":"tag","sha":%q}}`, "refs/tags/"+version, object)
					return true
				case strings.Contains(r.URL.Path, "/git/tags/"):
					tagReads++
					sha := strings.TrimPrefix(r.URL.Path, upstream+"/git/tags/")
					object, ok := objects[sha]
					if !ok {
						t.Errorf("unexpected tag object: %s", sha)
					}
					fmt.Fprintf(w, `{"sha":%q,"object":{"type":%q,"sha":%q}}`, sha, object.Type, object.SHA)
					return true
				case strings.Contains(r.URL.Path, "/commits/"):
					t.Error("must not resolve mutable commitish for evidence")
				case strings.Contains(r.URL.Path, "/license"), strings.Contains(r.URL.Path, "/contents/"):
					evidenceReads++
					ref := r.URL.Query().Get("ref")
					if strings.Contains(r.URL.Path, "/license") {
						licenseRefs[ref]++
					} else if ref != strings.Repeat("c", 40) {
						t.Errorf("wrong candidate evidence ref: %s", r.URL)
					}
				}
				return false
			}
			plan, err := f.remote.Plan(t.Context(), f.inputs)
			if err != nil || plan.Status != "planned" || tagReads != 4 || evidenceReads != 3 || licenseRefs[strings.Repeat("c", 40)] != 1 || licenseRefs[strings.Repeat("f", 40)] != 1 || f.posts.Load() != 0 {
				t.Fatalf("plan=%s err=%v peeled=%d evidence=%d posts=%d licenses=%v", plan.Status, err, tagReads, evidenceReads, f.posts.Load(), licenseRefs)
			}
		})
	}
}
