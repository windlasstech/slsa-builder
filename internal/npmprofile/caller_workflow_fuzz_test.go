package npmprofile

import "testing"

func FuzzParseCallerWorkflowPath(f *testing.F) {
	f.Add("example/project/.github/workflows/release.yml@refs/heads/main")
	f.Add("example/project/.github/workflows/js-ts-npm-package-slsa3.yml@refs/heads/main")
	f.Add("example/project/.github/workflows/../outside.yml@refs/heads/main")
	f.Add("not-a-workflow-ref")
	f.Fuzz(func(t *testing.T, reference string) {
		callerPath, err := ParseCallerWorkflowPath(reference)
		if err != nil {
			return
		}
		if !validCallerWorkflowPath(callerPath) || callerPath == NPMWorkflowPath {
			t.Fatalf("accepted invalid caller path %q", callerPath)
		}
		roundTrip, err := ParseCallerWorkflowPath("example/project/" + callerPath + "@refs/heads/main")
		if err != nil {
			t.Fatalf("reparse accepted caller path %q: %v", callerPath, err)
		}
		if roundTrip != callerPath {
			t.Fatalf("round trip changed caller path: first %q, second %q", callerPath, roundTrip)
		}
	})
}
