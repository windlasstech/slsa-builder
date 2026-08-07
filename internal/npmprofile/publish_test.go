package npmprofile

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const (
	testPublishRunID      = "123456789"
	testPublishRunAttempt = "2"
)

func TestPublishAbsent(t *testing.T) {
	fixture := newPublishFixture(t, publishFixtureOptions{initialVersionAbsent: true})
	result, err := Publish(context.Background(), fixture.request)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.State != PublishCommittedAsExpected || !result.MutationAttempted {
		t.Fatalf("Publish() result = %#v, want committed mutation", result)
	}
	fixture.requireInvocations(t, [][]string{fixture.expectedArgv()})
	if fixture.packumentRequests != 2 || fixture.attestationRequests != 2 {
		t.Fatalf("registry requests = packument:%d attestation:%d, want 2 and 2", fixture.packumentRequests, fixture.attestationRequests)
	}
}

func TestPublishSameRunRetry(t *testing.T) {
	fixture := newPublishFixture(t, publishFixtureOptions{})
	result, err := Publish(context.Background(), fixture.request)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.State != PublishCommittedAsExpected || result.MutationAttempted {
		t.Fatalf("Publish() result = %#v, want read-only convergence", result)
	}
	fixture.requireInvocations(t, nil)
}

func TestPublishForeignConflict(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		options publishFixtureOptions
	}{
		{name: "integrity", options: publishFixtureOptions{foreignIntegrity: true}},
		{name: "run-identity", options: publishFixtureOptions{foreignRunID: true}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newPublishFixture(t, testCase.options)
			result, err := Publish(context.Background(), fixture.request)
			if err == nil {
				t.Fatal("Publish() error = nil, want foreign conflict")
			}
			if result.State != PublishForeignConflict || result.MutationAttempted {
				t.Fatalf("Publish() result = %#v, want pre-mutation foreign conflict", result)
			}
			fixture.requireInvocations(t, nil)
			if fixture.attestationRequests != 1 {
				t.Fatalf("attestation requests = %d, want paired entry read", fixture.attestationRequests)
			}
		})
	}
}

func TestPublishAmbiguousReadback(t *testing.T) {
	fixture := newPublishFixture(t, publishFixtureOptions{initialVersionAbsent: true, npmExitCode: 73, readbackIndeterminate: true})
	result, err := Publish(context.Background(), fixture.request)
	if err == nil {
		t.Fatal("Publish() error = nil, want indeterminate read-back")
	}
	if result.State != PublishIndeterminate || !result.MutationAttempted {
		t.Fatalf("Publish() result = %#v, want one ambiguous mutation", result)
	}
	fixture.requireInvocations(t, [][]string{fixture.expectedArgv()})
	if fixture.clock.sleeps != 60 || fixture.packumentRequests != 62 || fixture.attestationRequests != 62 {
		t.Fatalf("polling = sleeps:%d packument:%d attestations:%d, want exact 15-minute budget", fixture.clock.sleeps, fixture.packumentRequests, fixture.attestationRequests)
	}
}

func TestPublishUsesExactBundle(t *testing.T) {
	fixture := newPublishFixture(t, publishFixtureOptions{initialVersionAbsent: true})
	result, err := Publish(context.Background(), fixture.request)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.State != PublishCommittedAsExpected {
		t.Fatalf("Publish() state = %q", result.State)
	}
	fixture.requireInvocations(t, [][]string{fixture.expectedArgv()})
	if !reflect.DeepEqual(fixture.verifier.localBytes, fixture.bundleBytes) {
		t.Fatalf("local verifier received altered bundle bytes\n got: %q\nwant: %q", fixture.verifier.localBytes, fixture.bundleBytes)
	}
	bundleOnDisk, readErr := os.ReadFile(fixture.request.BundlePath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !reflect.DeepEqual(bundleOnDisk, fixture.bundleBytes) {
		t.Fatal("bundle handoff bytes changed during publish")
	}
}

func TestPublishErrorMapping(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		output string
		wantID string
	}{
		{name: "trusted-publisher", output: "npm error code E404", wantID: IDTrustedPublisherMismatch},
		{name: "permission", output: "npm error code E403", wantID: IDMutationPermissionDenied},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newPublishFixture(t, publishFixtureOptions{initialVersionAbsent: true, npmExitCode: 1, npmError: testCase.output})
			result, err := Publish(context.Background(), fixture.request)
			if err == nil || result.Report.PrimaryID == nil || *result.Report.PrimaryID != testCase.wantID {
				t.Fatalf("Publish() = result:%#v err:%v, want %s", result, err, testCase.wantID)
			}
			fixture.requireInvocations(t, [][]string{fixture.expectedArgv()})
			if fixture.packumentRequests != 1 || fixture.attestationRequests != 1 {
				t.Fatal("definitive npm rejection incorrectly triggered read-back")
			}
		})
	}
}

type publishFixtureOptions struct {
	initialVersionAbsent  bool
	foreignIntegrity      bool
	foreignRunID          bool
	readbackIndeterminate bool
	npmExitCode           int
	npmError              string
}

type publishFixture struct {
	request             PublishRequest
	bundleBytes         []byte
	logPath             string
	clock               *publishFakeClock
	verifier            *publishFakeVerifier
	packumentRequests   int
	attestationRequests int
	mu                  sync.Mutex
}

func newPublishFixture(t *testing.T, options publishFixtureOptions) *publishFixture {
	t.Helper()
	directory := t.TempDir()
	tarballPath := filepath.Join(directory, "windlass-slsa-builder-1.2.3.tgz")
	bundlePath := tarballPath + ".intoto.jsonl"
	tarballBytes := []byte("exact packed npm tarball bytes")
	bundleBytes := []byte("exact P02 Sigstore bundle bytes")
	if err := os.WriteFile(tarballPath, tarballBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bundlePath, bundleBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	sha256Value := digest.SumSHA256(tarballBytes)
	sha512Value := digest.SumSHA512(tarballBytes)
	expectedSRI := "sha512-" + base64.StdEncoding.EncodeToString(sha512Value[:])
	parameters := validExternalParameters(ManagerNPM)

	fixture := &publishFixture{bundleBytes: bundleBytes, logPath: filepath.Join(directory, "npm.log")}
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		switch {
		case request.URL.EscapedPath() == "/%40windlass%2Fslsa-builder":
			fixture.packumentRequests++
			if options.initialVersionAbsent && fixture.packumentRequests == 1 {
				writePackument(writer, nil)
				return
			}
			if options.readbackIndeterminate {
				fmt.Fprint(writer, `{`)
				return
			}
			integrity := expectedSRI
			if options.foreignIntegrity {
				integrity = "sha512-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="
			}
			writePackument(writer, &RegistryVersion{Name: parameters.Package.Name, Version: parameters.Package.Version, Integrity: integrity, Tarball: "https://registry.npmjs.org/windlass-slsa-builder-1.2.3.tgz"})
		case request.URL.EscapedPath() == "/-/npm/v1/attestations/%40windlass%2Fslsa-builder@1.2.3":
			fixture.attestationRequests++
			if options.initialVersionAbsent && fixture.packumentRequests == 1 {
				writer.WriteHeader(http.StatusNotFound)
				return
			}
			if options.readbackIndeterminate {
				fmt.Fprint(writer, `{`)
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			fmt.Fprint(writer, `{"attestations":[{"predicateType":"https://slsa.dev/provenance/v1","bundle":{"kind":"published"}}]}`)
		default:
			http.Error(writer, "unexpected path", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	encodedParameters, err := EncodeExternalParameters(parameters)
	if err != nil {
		t.Fatalf("EncodeExternalParameters() error = %v", err)
	}
	statement := provenance.Statement{
		Type: provenance.StatementType,
		Subject: []provenance.Subject{{
			Name:   npmPURL(parameters.Package.Name, parameters.Package.Version),
			Digest: map[string]string{"sha256": sha256Value.String(), "sha512": sha512Value.String()},
		}},
		PredicateType: provenance.PredicateType,
		Predicate: provenance.Predicate{
			BuildDefinition: provenance.BuildDefinition{BuildType: NPMBuildType, ExternalParameters: encodedParameters, InternalParameters: json.RawMessage(`{}`)},
			RunDetails:      provenance.RunDetails{Metadata: provenance.Metadata{InvocationID: "https://github.com/example/project/actions/runs/" + testPublishRunID + "/attempts/1"}},
		},
	}
	fixture.verifier = &publishFakeVerifier{localBundle: bundleBytes, statement: statement, foreignRunID: options.foreignRunID}
	fixture.clock = &publishFakeClock{now: time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)}

	npmPath := writeFakeNPM(t, directory)
	t.Setenv("NPM_LOG", fixture.logPath)
	t.Setenv("FAKE_NPM_EXIT", strconv.Itoa(options.npmExitCode))
	t.Setenv("FAKE_NPM_ERROR", options.npmError)
	t.Setenv("NPM_TOKEN", "must-not-reach-fake-npm")
	t.Setenv("NODE_AUTH_TOKEN", "must-not-reach-fake-npm")
	untrustedHome := filepath.Join(directory, "untrusted-home")
	if err := os.Mkdir(untrustedHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(untrustedHome, ".npmrc"), []byte("//registry.npmjs.org/:_authToken=must-not-reach-npm\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", untrustedHome)
	baseTransport, ok := server.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatal("httptest TLS client has an unexpected transport")
	}
	transport := baseTransport.Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	serverAddress := server.Listener.Addr().String()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		_ = address
		return (&net.Dialer{}).DialContext(ctx, network, serverAddress)
	}
	registryHTTPClient := &http.Client{Transport: transport}
	registry, err := NewRegistryClient(RegistryClientConfig{HTTPClient: registryHTTPClient, RegistryURL: "https://registry.npmjs.org/"})
	if err != nil {
		t.Fatal(err)
	}
	fixture.request = PublishRequest{
		NPMExecutable:       npmPath,
		TarballPath:         tarballPath,
		BundlePath:          bundlePath,
		TarballSHA256:       sha256Value,
		TarballSHA512:       sha512Value,
		BundleSHA256:        digest.SumSHA256(bundleBytes),
		Registry:            registry,
		RunID:               testPublishRunID,
		RunAttempt:          testPublishRunAttempt,
		SourceRepositoryURI: "https://github.com/example/project",
		OIDCExchange: OIDCExchangeResult{
			Token:            newSecretToken("short-lived-token-must-not-reach-npm-env"),
			CreatedAt:        fixture.clock.now.Add(-time.Minute),
			ExpiresAt:        fixture.clock.now.Add(time.Hour),
			WorkflowFilename: "release.yml",
			Report:           oidcPassReport(),
		},
		Verifier: fixture.verifier,
		Now:      fixture.clock.Now,
		Sleep:    fixture.clock.Sleep,
	}
	return fixture
}

func (fixture *publishFixture) expectedArgv() []string {
	return []string{
		"publish",
		fixture.request.TarballPath,
		"--provenance-file=" + fixture.request.BundlePath,
		"--registry=" + fixture.request.Registry.URL(),
		"--tag=latest",
	}
}

func (fixture *publishFixture) requireInvocations(t *testing.T, want [][]string) {
	t.Helper()
	encoded, err := os.ReadFile(fixture.logPath)
	if errors.Is(err, os.ErrNotExist) {
		if len(want) == 0 {
			return
		}
		t.Fatalf("fake npm log is absent, want %#v", want)
	}
	if err != nil {
		t.Fatal(err)
	}
	var got [][]string
	for _, line := range strings.Split(strings.TrimSpace(string(encoded)), "\n") {
		if line == "" {
			continue
		}
		var argv []string
		if err := json.Unmarshal([]byte(line), &argv); err != nil {
			t.Fatalf("decode fake npm log %q: %v", line, err)
		}
		got = append(got, argv)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fake npm invocations = %#v, want %#v", got, want)
	}
}

func writeFakeNPM(t *testing.T, directory string) string {
	t.Helper()
	path := filepath.Join(directory, "npm")
	script := `#!/bin/sh
if [ -n "${NPM_TOKEN-}" ] || [ -n "${NODE_AUTH_TOKEN-}" ]; then
  exit 99
fi
if [ -f "$HOME/.npmrc" ]; then
  exit 98
fi
python3 -c 'import json, os, sys; open(os.environ["NPM_LOG"], "a", encoding="utf-8").write(json.dumps(sys.argv[1:]) + "\n")' "$@"
if [ -n "${FAKE_NPM_ERROR-}" ]; then
  printf '%s\n' "$FAKE_NPM_ERROR" >&2
fi
exit "${FAKE_NPM_EXIT:-0}"
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func writePackument(writer http.ResponseWriter, version *RegistryVersion) {
	writer.Header().Set("Content-Type", "application/json")
	if version == nil {
		fmt.Fprint(writer, `{"name":"@windlass/slsa-builder","versions":{}}`)
		return
	}
	encoded, err := json.Marshal(map[string]any{
		"name": "@windlass/slsa-builder",
		"versions": map[string]any{
			version.Version: map[string]any{
				"name": version.Name, "version": version.Version,
				"dist": map[string]string{"integrity": version.Integrity, "tarball": version.Tarball},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	if _, err := writer.Write(encoded); err != nil {
		panic(err)
	}
}

type publishFakeVerifier struct {
	localBundle  []byte
	localBytes   []byte
	statement    provenance.Statement
	foreignRunID bool
}

func (verifier *publishFakeVerifier) Verify(ctx context.Context, bundle []byte) (VerifiedPublishBundle, error) {
	_ = ctx
	if reflect.DeepEqual(bundle, verifier.localBundle) {
		verifier.localBytes = append([]byte(nil), bundle...)
		return VerifiedPublishBundle{Statement: verifier.statement, RunInvocationURI: verifier.statement.Predicate.RunDetails.Metadata.InvocationID}, nil
	}
	if string(bundle) != `{"kind":"published"}` {
		return VerifiedPublishBundle{}, errors.New("unexpected published bundle")
	}
	statement := verifier.statement
	if verifier.foreignRunID {
		statement.Predicate.RunDetails.Metadata.InvocationID = "https://github.com/example/project/actions/runs/987654321/attempts/1"
	}
	return VerifiedPublishBundle{Statement: statement, RunInvocationURI: statement.Predicate.RunDetails.Metadata.InvocationID}, nil
}

type publishFakeClock struct {
	now    time.Time
	sleeps int
}

func (clock *publishFakeClock) Now() time.Time { return clock.now }

func (clock *publishFakeClock) Sleep(ctx context.Context, duration time.Duration) error {
	clock.sleeps++
	clock.now = clock.now.Add(duration)
	return ctx.Err()
}
