package command

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestPublishCommandRejectsInvalidHandoffInputs(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"--tarball-sha256", "--tarball-sha512", "--bundle-sha256", "--bundle-artifact-dir"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			tarball, bundle := []byte("tarball"), []byte("bundle")
			args := publishArgs(t, oneFileDirectory(t, "package.tgz", tarball), oneFileDirectory(t, "package.tgz.intoto.jsonl", bundle), filepath.Join(t.TempDir(), "report"), filepath.Join(t.TempDir(), "out"), tarball, bundle)
			for i := range args {
				if args[i] == flag {
					args[i+1] = "invalid"
				}
			}
			command := npmProfilePublishCommand{publish: func(context.Context, npmProfilePublishInput) (npmprofile.PublishResult, error) {
				t.Fatal("publish called for invalid input")
				return npmprofile.PublishResult{}, nil
			}}
			var output bytes.Buffer
			err := command.Execute(t.Context(), args, &output)
			if err == nil || output.Len() != 0 || errors.Is(err, ErrVerificationFailure) {
				t.Fatalf("invalid input = %v, output %q", err, output.String())
			}
		})
	}
}

func TestPublishCommandOutputFailures(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"invalid report", "report file", "github output", "writer"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			tarball, bundle := []byte("tarball"), []byte("bundle")
			reportPath, githubOutput := filepath.Join(t.TempDir(), "report"), filepath.Join(t.TempDir(), "out")
			switch stage {
			case "report file":
				reportPath = t.TempDir()
			case "github output":
				githubOutput = t.TempDir()
			}
			called := false
			command := npmProfilePublishCommand{publish: func(context.Context, npmProfilePublishInput) (npmprofile.PublishResult, error) {
				called = true
				report, err := diagnostic.Build(nil, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				if stage == "invalid report" {
					report = diagnostic.Report{}
				}
				return npmprofile.PublishResult{State: npmprofile.PublishCommittedAsExpected, Report: report}, nil
			}}
			args := publishArgs(t, oneFileDirectory(t, "package.tgz", tarball), oneFileDirectory(t, "package.tgz.intoto.jsonl", bundle), reportPath, githubOutput, tarball, bundle)
			var output bytes.Buffer
			cause := errors.New("publish report sink failed")
			var err error
			if stage == "writer" {
				err = command.Execute(t.Context(), args, &stagedWriter{err: cause})
			} else {
				err = command.Execute(t.Context(), args, &output)
			}
			if !called || err == nil || errors.Is(err, ErrVerificationFailure) || output.Len() != 0 {
				t.Fatalf("output failure: called=%t err=%v output=%q", called, err, output.String())
			}
			if stage == "writer" && !errors.Is(err, cause) {
				t.Fatalf("writer error = %v, want original cause", err)
			}
		})
	}
}
