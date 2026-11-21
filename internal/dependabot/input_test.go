package dependabot

import (
	"bytes"
	"os"
	"testing"
)

func TestAcceptedTransitionIdentity(t *testing.T) {
	t.Parallel()
	base, err := readModules("testdata/base")
	if err != nil {
		t.Fatal(err)
	}
	head, err := readModules("testdata/head")
	if err != nil {
		t.Fatal(err)
	}
	for _, retain := range []bool{false, true} {
		t.Run(map[bool]string{false: "old removal new addition", true: "preexisting checksums"}[retain], func(t *testing.T) {
			t.Parallel()
			before, after := base, head
			if retain {
				before.Sum = append(bytes.Clone(base.Sum), bytes.Split(head.Sum, []byte("example.org/other"))[0]...)
				after.Sum = before.Sum
			}
			transition, reason := ParseSigstoreModuleTransition(before, after)
			if reason != "" {
				t.Fatalf("accepted transition rejected: %s", reason)
			}
			if transition.old.Version != "v1.4.0" || transition.candidate.Version != "v1.5.0" || transition.old.DirHash1 != "0867457592e1612738db260dd3e94beb25a1eefd33dd2d22db5bf9d25bd260ac" || transition.candidate.DirHash1 != "0000000000000000000000000000000000000000000000000000000000000000" {
				t.Fatalf("transition identity: old=%+v candidate=%+v", transition.old, transition.candidate)
			}
			if !bytes.Equal(transition.base.Mod, before.Mod) || !bytes.Equal(transition.base.Sum, before.Sum) || !bytes.Equal(transition.head.Mod, after.Mod) || !bytes.Equal(transition.head.Sum, after.Sum) {
				t.Fatal("accepted transition lost module bytes")
			}
		})
	}
}

func TestReadBoundedAcceptsExactLimit(t *testing.T) {
	t.Parallel()
	path := t.TempDir() + "/input"
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), maxInput), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readBounded(path)
	if err != nil || len(got) != maxInput || bytes.Count(got, []byte("x")) != maxInput {
		t.Fatalf("read length=%d err=%v, want exactly %d bytes", len(got), err, maxInput)
	}
}
