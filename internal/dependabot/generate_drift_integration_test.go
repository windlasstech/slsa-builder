package dependabot

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

func TestRegenerateNextStableMinorExercisesCandidateDrift(t *testing.T) {
	project, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	scratch := CloneCommittedTestRoot(t, project)
	base, err := readModules(scratch)
	if err != nil {
		t.Fatal(err)
	}
	old, err := toolchainpins.ParseSigningAdapterModule(base.Mod, base.Sum)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(old.Version, ".")
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	candidate := parts[0] + "." + strconv.Itoa(minor+1) + ".0"
	head := Modules{
		Mod: bytes.ReplaceAll(base.Mod, []byte(signerModule+" "+old.Version), []byte(signerModule+" "+candidate)),
		Sum: bytes.ReplaceAll(base.Sum, []byte(signerModule+" "+old.Version), []byte(signerModule+" "+candidate)),
	}
	head.Sum = bytes.ReplaceAll(head.Sum, []byte(old.Sum), []byte("h1:DGdFdZLhYSc42yYN0+lL6yWh7v0z3S0i21v50lvSYKw="))
	transition, reason := ParseSigstoreModuleTransition(base, head)
	if reason != "" {
		t.Fatal(reason)
	}
	prepared := prepared{transition: transition, release: Release{ReleaseDate: "2026-11-10", GoVersion: "1.27.0"}}
	for _, path := range sourceSites {
		source := filepath.Join(scratch, path)
		raw, err := readBounded(source)
		if err != nil {
			t.Fatal(err)
		}
		edited, err := editSite(path, raw, prepared)
		if err != nil {
			t.Fatalf("edit %s: %v", path, err)
		}
		if err := os.WriteFile(filepath.Join(scratch, path), edited, 0600); err != nil {
			t.Fatal(err)
		}
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	if err := writeModules(tools, base); err != nil {
		t.Fatal(err)
	}
	audit := filepath.Join(tools, "compilations")
	script := fmt.Sprintf(`#!/bin/sh
set -eu
test "$1" = test
test "$2" = -c
cmp go.mod %q
cmp go.sum %q
printf 'base-only\n' >> %q
exec %q "$@"
`, filepath.Join(tools, "go.mod"), filepath.Join(tools, "go.sum"), audit, goTool)
	if err := os.WriteFile(filepath.Join(tools, "go"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Logf("real generation transition: %s -> %s", old.Version, candidate)
	if err := regenerate(t.Context(), scratch, transition); err != nil {
		t.Fatal(err)
	}
	compiled, err := os.ReadFile(audit)
	if err != nil {
		t.Fatal(err)
	}
	if string(compiled) != "base-only\nbase-only\n" {
		t.Fatalf("unexpected compilations: %q", compiled)
	}
	installed, err := readModules(scratch)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(installed.Mod, head.Mod) || !bytes.Equal(installed.Sum, head.Sum) {
		t.Fatal("candidate module data missing after generation")
	}
	t.Log("two real compilations matched base go.mod/go.sum; candidate files remained data; regeneration executed the candidate drift test")
}
