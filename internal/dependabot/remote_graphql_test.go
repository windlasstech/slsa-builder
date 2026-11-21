package dependabot

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRemoteGraphQLBranchContract(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	api := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request struct {
			Query     string `json:"query"`
			Variables struct {
				Input struct {
					Branch map[string]string `json:"branch"`
					Head   string            `json:"expectedHeadOid"`
				} `json:"input"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		branch := request.Variables.Input.Branch
		if r.Method != "POST" || r.URL.Path != "/graphql" || !strings.HasPrefix(request.Query, "mutation(") || len(branch) != 2 || branch["branchName"] != "dependabot/go_modules/sigstore" || branch["repositoryNameWithOwner"] != companionRepository || request.Variables.Input.Head != strings.Repeat("b", 40) {
			t.Errorf("wrong request: %+v", request)
		}
		if _, exists := branch["refName"]; exists {
			t.Error("refName is not a GraphQL branch field")
		}
		w.WriteHeader(422)
	})
	var input commitInput
	input.Branch.Repository = companionRepository
	input.Branch.RefName = "dependabot/go_modules/sigstore"
	input.ExpectedHeadOID = strings.Repeat("b", 40)
	err := api.commitAndReconcile(t.Context(), input)
	var status *githubStatusError
	if !errors.As(err, &status) || status.Code != 422 || calls.Load() != 1 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
}
