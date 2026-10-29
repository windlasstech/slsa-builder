package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
	"github.com/windlasstech/slsa-builder/internal/provenance"
	"github.com/windlasstech/slsa-builder/internal/signing"
)

func TestNPMProfileSignBuildTimestamps(t *testing.T) {
	for _, test := range []struct {
		name, started, finished, diagnostic string
	}{
		{"build window", "2026-08-14T11:59:58Z", "2026-08-14T12:00:03Z", ""},
		{"same second", "2026-08-14T11:59:58Z", "2026-08-14T11:59:58Z", ""},
		{"malformed start", "yesterday", "2026-08-14T12:00:03Z", "windlass.verify.error.timestamp-format-invalid"},
		{"reversed window", "2026-08-14T12:00:10Z", "2026-08-14T12:00:04Z", "windlass.verify.error.timestamp-ordering-invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given a digest-bound metadata handoff with observed build times.
			metadata, tarball := signSourceRefMetadata(t)
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(mustMarshalSignJSON(t, metadata), &fields); err != nil {
				t.Fatal(err)
			}
			fields["started_on"] = mustMarshalSignJSON(t, test.started)
			fields["finished_on"] = mustMarshalSignJSON(t, test.finished)
			encoded := mustMarshalSignJSON(t, fields)
			githubOutput := filepath.Join(t.TempDir(), "output")
			setSignGitHubEnvironment(t, signInvocationRef, signInvocationRevision, githubOutput)
			called := false
			command := npmProfileSignCommand{
				preflight: func(context.Context, string, string) npmprofile.OIDCExchangeResult {
					return npmprofile.OIDCExchangeResult{WorkflowFilename: "release.yml"}
				},
				sign: func(ctx context.Context, request signing.Request) (signing.Result, error) {
					if err := ctx.Err(); err != nil {
						return signing.Result{}, err
					}
					called = true
					statement, err := provenance.DecodeStatement(request.Statement)
					if err != nil {
						t.Fatal(err)
					}
					got := statement.Predicate.RunDetails.Metadata
					if got.StartedOn != test.started || got.FinishedOn != test.finished {
						t.Fatalf("signed build times = %#v, want %s / %s", got, test.started, test.finished)
					}
					return signing.Result{Statement: request.Statement, Bundle: []byte(`{"bundle":"fixture"}`)}, nil
				},
			}
			args := signSourceRefArguments(writeSignHandoff(t, "build-metadata.json", encoded), writeSignHandoff(t, signTarballName, tarball), t.TempDir(), githubOutput, digest.SumSHA256(encoded).String(), digest.SumSHA256(tarball).String(), signBuiltRef, signBuiltRevision)
			// When the signing command consumes the handoff.
			var output bytes.Buffer
			err := command.Execute(t.Context(), args, &output)
			// Then timestamps are preserved or rejected by the existing policy.
			if test.diagnostic != "" {
				if !errors.Is(err, ErrVerificationFailure) || called {
					t.Fatalf("Execute() = %v, signer called = %v, output = %s", err, called, output.String())
				}
				assertPrimaryDiagnostic(t, output.Bytes(), ExitCodeVerificationFailure, test.diagnostic)
				return
			}
			if err != nil || !called {
				t.Fatalf("Execute() = %v, signer called = %v, output = %s", err, called, output.String())
			}
		})
	}
}

func TestBuildMetadataDecodeRequiresFinishedOn(t *testing.T) {
	t.Parallel()
	// Given otherwise complete metadata with the completion member omitted.
	metadata, _ := signSourceRefMetadata(t)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(mustMarshalSignJSON(t, metadata), &fields); err != nil {
		t.Fatal(err)
	}
	fields["started_on"] = mustMarshalSignJSON(t, "2026-08-14T11:59:58Z")
	delete(fields, "finished_on")
	// When decoding the closed handoff schema.
	_, err := decodeTypedJSON[npmprofile.BuildMetadata](mustMarshalSignJSON(t, fields), nil)
	// Then the missing required member is identified, not an unknown start field.
	if err == nil || !strings.Contains(err.Error(), "finished_on") {
		t.Fatalf("decode error = %v, want missing finished_on", err)
	}
}

func TestBuildMetadataDecodeRejectsInvalidTimestampMembers(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, member string
		value        json.RawMessage
	}{
		{"missing start", "started_on", nil},
		{"null start", "started_on", json.RawMessage(`null`)},
		{"null finish", "finished_on", json.RawMessage(`null`)},
		{"numeric start", "started_on", json.RawMessage(`123`)},
		{"unknown member", "unexpected", json.RawMessage(`true`)},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			metadata, _ := signSourceRefMetadata(t)
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(mustMarshalSignJSON(t, metadata), &fields); err != nil {
				t.Fatal(err)
			}
			if test.value == nil {
				delete(fields, test.member)
			} else {
				fields[test.member] = test.value
			}
			_, err := decodeTypedJSON[npmprofile.BuildMetadata](mustMarshalSignJSON(t, fields), nil)
			if err == nil {
				t.Fatal("invalid metadata decoded successfully")
			}
		})
	}
}
