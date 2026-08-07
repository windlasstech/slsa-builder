package npmprofile

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/handoff"
	"github.com/windlasstech/slsa-builder/internal/identity"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const (
	IDAlreadyPublishedVersion                  = "windlass.verify.error.already-published-version"
	IDBundleByteFormatMismatch                 = "windlass.verify.error.bundle-byte-format-mismatch"
	IDMutationPermissionDenied                 = "windlass.verify.error.mutation-permission-denied"
	IDPrepublishRegistryMetadataRequired       = "windlass.verify.error.prepublish-registry-metadata-required"
	IDRegistryLinkageMismatch                  = "windlass.verify.error.registry-linkage-mismatch"
	IDUnsupportedInitialPublication            = "windlass.verify.error.unsupported-initial-publication"
	npmSLSAPredicateType                       = "https://slsa.dev/provenance/v1"
	publishPollInterval                        = 15 * time.Second
	publishPollBudget                          = 15 * time.Minute
	maxPublishBundleSize                 int64 = 32 << 20
	maxNPMOutputSize                           = 64 << 10
)

// PublishState is the closed ADR 0067 npm mutation-state vocabulary.
type PublishState string

const (
	PublishCommittedAsExpected PublishState = "committed-as-expected"
	PublishAbsent              PublishState = "absent"
	PublishForeignConflict     PublishState = "foreign-conflict"
	PublishIndeterminate       PublishState = "indeterminate"
)

// VerifiedPublishBundle is evidence returned only after full Sigstore and identity-policy verification.
type VerifiedPublishBundle struct {
	Statement        provenance.Statement
	RunInvocationURI string
}

// PublishBundleVerifier verifies both the local P02 handoff and registry-returned bundles.
type PublishBundleVerifier interface {
	Verify(context.Context, []byte) (VerifiedPublishBundle, error)
}

// PublishRequest contains the complete digest-bound input to one serialized npm mutation segment.
type PublishRequest struct {
	NPMExecutable       string
	TarballPath         string
	BundlePath          string
	TarballSHA256       digest.SHA256
	TarballSHA512       digest.SHA512
	BundleSHA256        digest.SHA256
	Registry            *RegistryClient
	RunID               string
	RunAttempt          string
	SourceRepositoryURI string
	OIDCExchange        OIDCExchangeResult
	Verifier            PublishBundleVerifier
	Now                 func() time.Time
	Sleep               func(context.Context, time.Duration) error
}

// PublishResult records the final state, whether npm ran, and the secret-safe report.
type PublishResult struct {
	State             PublishState
	MutationAttempted bool
	Report            diagnostic.Report
}

// PublishError preserves the classified result while exposing its registered diagnostic ID.
type PublishError struct {
	Result PublishResult
	Cause  error
}

func (publishError *PublishError) Error() string {
	if publishError.Cause == nil {
		return "npm publish failed"
	}
	return publishError.Cause.Error()
}

func (publishError *PublishError) Unwrap() error { return publishError.Cause }

func (publishError *PublishError) DiagnosticID() string {
	if publishError.Result.Report.PrimaryID == nil {
		return ""
	}
	return *publishError.Result.Report.PrimaryID
}

type preparedPublish struct {
	request       PublishRequest
	parameters    ExternalParameters
	statement     provenance.Statement
	runInvocation string
	expectedSRI   string
	argv          []string
	now           func() time.Time
	sleep         func(context.Context, time.Duration) error
}

type registryObservation struct {
	packageExists  bool
	version        *RegistryVersion
	attestations   RegistryAttestationState
	packumentErr   error
	attestationErr error
}

type observationDecision struct {
	state         PublishState
	terminal      bool
	packageAbsent bool
}

// Publish revalidates entry state, performs at most one argv-only npm mutation, and resolves read-back.
func Publish(ctx context.Context, request PublishRequest) (PublishResult, error) {
	prepared, err := preparePublish(ctx, request)
	if err != nil {
		return publishFailure(request, PublishIndeterminate, false, diagnosticIDOr(err, IDBundleByteFormatMismatch), "npm.publish.entry", err)
	}

	entryDecision, err := prepared.classify(ctx, false)
	if err != nil {
		return prepared.failure(PublishIndeterminate, false, IDPrepublishRegistryMetadataRequired, "npm.publish.entry-revalidation", err)
	}
	if entryDecision.packageAbsent {
		return prepared.failure(PublishAbsent, false, IDUnsupportedInitialPublication, "npm.publish.package-identity", errors.New("npm package identity does not already exist"))
	}
	switch entryDecision.state {
	case PublishCommittedAsExpected:
		return prepared.success(false)
	case PublishForeignConflict:
		return prepared.failure(PublishForeignConflict, false, IDAlreadyPublishedVersion, "npm.publish.convergence", errors.New("registry version is foreign to this run"))
	case PublishIndeterminate:
		return prepared.failure(PublishIndeterminate, false, IDPrepublishRegistryMetadataRequired, "npm.publish.entry-revalidation", errors.New("registry entry state is indeterminate"))
	case PublishAbsent:
	default:
		return prepared.failure(PublishIndeterminate, false, IDPrepublishRegistryMetadataRequired, "npm.publish.entry-revalidation", errors.New("registry entry state is invalid"))
	}
	if err := prepared.validateNPMVersion(ctx); err != nil {
		return prepared.failure(PublishAbsent, false, diagnosticIDOr(err, "windlass.verify.error.builder-version-mismatch"), "npm.publish.toolchain", err)
	}

	output, mutationErr := prepared.runNPM(ctx)
	if mutationErr != nil {
		if id := definitivePublishError(output); id != "" {
			return prepared.failure(PublishAbsent, true, id, "npm.publish.mutation", errors.New("npm rejected publication before commit"))
		}
	}

	readbackDecision, readbackErr := prepared.classify(ctx, true)
	if readbackErr != nil || readbackDecision.state == PublishIndeterminate {
		return prepared.failure(PublishIndeterminate, true, IDRegistryLinkageMismatch, "npm.publish.readback", errors.New("publication outcome is indeterminate"))
	}
	if readbackDecision.state == PublishForeignConflict {
		return prepared.failure(PublishForeignConflict, true, IDRegistryLinkageMismatch, "npm.publish.readback", errors.New("published registry evidence conflicts with the verified handoff"))
	}
	if readbackDecision.state != PublishCommittedAsExpected {
		return prepared.failure(PublishIndeterminate, true, IDRegistryLinkageMismatch, "npm.publish.readback", errors.New("publication did not converge"))
	}
	return prepared.success(true)
}

func preparePublish(ctx context.Context, request PublishRequest) (preparedPublish, error) {
	if ctx == nil || ctx.Err() != nil || request.Registry == nil || request.Verifier == nil || request.Registry.URL() == "" {
		return preparedPublish{}, errors.New("publish context, registry, and verifier are required")
	}
	if !positiveDecimal(request.RunID) || !positiveDecimal(request.RunAttempt) {
		return preparedPublish{}, errors.New("run identity must use positive canonical decimal values")
	}
	if request.NPMExecutable == "" {
		return preparedPublish{}, errors.New("npm executable is required")
	}
	if !filepath.IsAbs(request.NPMExecutable) || filepath.Base(request.NPMExecutable) != "npm" {
		return preparedPublish{}, fmt.Errorf("npm executable must be an absolute path to an npm binary: %q", request.NPMExecutable)
	}
	tarballBytes, err := verifiedPublishFile(request.TarballPath, ".tgz", maxTarballSize)
	if err != nil {
		return preparedPublish{}, fmt.Errorf("verify tarball path: %w", err)
	}
	if !digest.SumSHA256(tarballBytes).Equal(request.TarballSHA256) || !digest.SumSHA512(tarballBytes).Equal(request.TarballSHA512) {
		return preparedPublish{}, errors.New("verified tarball handoff digest mismatch")
	}
	bundleBytes, err := verifiedPublishFile(request.BundlePath, ".intoto.jsonl", maxPublishBundleSize)
	if err != nil {
		return preparedPublish{}, fmt.Errorf("verify provenance bundle path: %w", err)
	}
	if !digest.SumSHA256(bundleBytes).Equal(request.BundleSHA256) {
		return preparedPublish{}, errors.New("verified provenance bundle handoff digest mismatch")
	}
	verified, err := request.Verifier.Verify(ctx, bundleBytes)
	if err != nil {
		return preparedPublish{}, fmt.Errorf("verify P02 provenance bundle: %w", err)
	}
	parameters, err := DecodeExternalParameters(verified.Statement.Predicate.BuildDefinition.ExternalParameters)
	if err != nil {
		return preparedPublish{}, err
	}
	runInvocation, err := validatePreparedBindings(request, verified, parameters)
	if err != nil {
		return preparedPublish{}, err
	}
	expectedSRI := sha512SRI(request.TarballSHA512)
	argv := []string{
		"publish",
		request.TarballPath,
		"--provenance-file=" + request.BundlePath,
		"--registry=" + parameters.Publish.ResolvedRegistryURL,
		"--tag=" + parameters.Publish.ResolvedDistTag,
	}
	if parameters.Publish.PublishAccessOption != nil {
		argv = append(argv, "--access="+*parameters.Publish.PublishAccessOption)
	}
	now := request.Now
	if now == nil {
		now = time.Now
	}
	sleep := request.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	return preparedPublish{
		request: request, parameters: parameters, statement: verified.Statement,
		runInvocation: runInvocation, expectedSRI: expectedSRI, argv: argv, now: now, sleep: sleep,
	}, nil
}

func validatePreparedBindings(request PublishRequest, verified VerifiedPublishBundle, parameters ExternalParameters) (string, error) {
	statement := verified.Statement
	if statement.Type != provenance.StatementType || statement.PredicateType != provenance.PredicateType ||
		statement.Predicate.BuildDefinition.BuildType != NPMBuildType || len(statement.Subject) != 1 {
		return "", errors.New("verified provenance Statement has an unexpected type, build type, or subject count")
	}
	if parameters.Publish.ResolvedRegistryURL != request.Registry.URL() {
		return "", errors.New("registry client differs from signed publish intent")
	}
	if filepath.Base(request.TarballPath) != parameters.Package.TarballName || filepath.Base(request.BundlePath) != parameters.Package.TarballName+".intoto.jsonl" {
		return "", errors.New("artifact paths differ from signed package metadata")
	}
	wantPURL, err := NPMPackagePURL(parameters.Package.Name, parameters.Package.Version)
	if err != nil {
		return "", err
	}
	subject := statement.Subject[0]
	if subject.Name != wantPURL || subject.Digest["sha256"] != request.TarballSHA256.String() || subject.Digest["sha512"] != request.TarballSHA512.String() || len(subject.Digest) != 2 {
		return "", errors.New("verified provenance subject differs from the tarball handoff")
	}
	if verified.RunInvocationURI == "" || verified.RunInvocationURI != statement.Predicate.RunDetails.Metadata.InvocationID {
		return "", errors.New("verified certificate and SLSA invocationId bindings differ")
	}
	invocation, err := identity.ParseRunInvocationURI(verified.RunInvocationURI, request.SourceRepositoryURI)
	invocationAttempt, invocationAttemptErr := strconv.ParseUint(invocation.Attempt, 10, 64)
	currentAttempt, currentAttemptErr := strconv.ParseUint(request.RunAttempt, 10, 64)
	if err != nil || invocation.RunID != request.RunID || invocationAttemptErr != nil || currentAttemptErr != nil || invocationAttempt > currentAttempt {
		return "", errors.New("local P02 bundle run identity differs from the current attempt")
	}
	if request.OIDCExchange.Report.PrimaryID != nil || !request.OIDCExchange.Token.valid() || request.OIDCExchange.WorkflowFilename != parameters.Caller.WorkflowFilename {
		return "", errors.New("successful trusted-publisher preflight for the signed caller workflow is required")
	}
	now := time.Now()
	if request.Now != nil {
		now = request.Now()
	}
	if request.OIDCExchange.CreatedAt.IsZero() || request.OIDCExchange.ExpiresAt.IsZero() ||
		request.OIDCExchange.CreatedAt.After(now) || !request.OIDCExchange.ExpiresAt.After(now) {
		return "", errors.New("trusted-publisher exchange token lifetime is unusable")
	}
	return verified.RunInvocationURI, nil
}

func (prepared preparedPublish) classify(ctx context.Context, afterMutation bool) (observationDecision, error) {
	deadline := prepared.now().Add(publishPollBudget)
	var last registryObservation
	for {
		last = prepared.observe(ctx)
		decision := prepared.decide(ctx, last, afterMutation, false)
		if decision.terminal {
			return decision, nil
		}
		if !prepared.now().Before(deadline) {
			final := prepared.decide(ctx, last, afterMutation, true)
			if final.state == PublishIndeterminate {
				return final, errors.New("registry polling budget exhausted without authoritative evidence")
			}
			return final, nil
		}
		if err := prepared.sleep(ctx, publishPollInterval); err != nil {
			return observationDecision{state: PublishIndeterminate, terminal: true}, err
		}
	}
}

func (prepared preparedPublish) observe(ctx context.Context) registryObservation {
	state, packumentErr := prepared.request.Registry.Preflight(ctx, prepared.parameters.Package.Name, prepared.parameters.Package.Version)
	attestations, attestationErr := prepared.request.Registry.Attestations(ctx, prepared.parameters.Package.Name, prepared.parameters.Package.Version)
	return registryObservation{
		packageExists:  state.PackageExists,
		version:        state.Version,
		attestations:   attestations,
		packumentErr:   packumentErr,
		attestationErr: attestationErr,
	}
}

func (prepared preparedPublish) decide(ctx context.Context, observation registryObservation, afterMutation, exhausted bool) observationDecision {
	if observation.packumentErr != nil || observation.attestationErr != nil {
		return observationDecision{state: PublishIndeterminate, terminal: exhausted}
	}
	if !observation.packageExists {
		if afterMutation {
			return observationDecision{state: PublishIndeterminate, terminal: exhausted}
		}
		return observationDecision{state: PublishAbsent, terminal: true, packageAbsent: true}
	}
	if observation.version == nil {
		if afterMutation {
			return observationDecision{state: PublishIndeterminate, terminal: exhausted}
		}
		return observationDecision{state: PublishAbsent, terminal: true}
	}
	version := observation.version
	actualSRI, validSRI := normalizeSHA512SRI(version.Integrity)
	if !validSRI {
		return observationDecision{state: PublishIndeterminate, terminal: exhausted}
	}
	if actualSRI != prepared.expectedSRI || !registryTarballMatches(version.Tarball, prepared.parameters.Package.TarballName) {
		return observationDecision{state: PublishForeignConflict, terminal: true}
	}
	foundRequired := false
	verifiedForeign := false
	verificationFailed := false
	for _, candidate := range observation.attestations.Attestations {
		if candidate.PredicateType != npmSLSAPredicateType {
			continue
		}
		foundRequired = true
		verified, err := prepared.request.Verifier.Verify(ctx, candidate.Bundle)
		if err != nil {
			verificationFailed = true
			continue
		}
		if prepared.matchesPublishedBundle(verified) {
			attempt, parseErr := strconv.ParseUint(prepared.request.RunAttempt, 10, 64)
			if parseErr == nil && (afterMutation || attempt > 1) {
				return observationDecision{state: PublishCommittedAsExpected, terminal: true}
			}
		}
		verifiedForeign = true
	}
	if verifiedForeign && !verificationFailed {
		return observationDecision{state: PublishForeignConflict, terminal: true}
	}
	if exhausted {
		if !foundRequired && observation.attestations.Found {
			return observationDecision{state: PublishForeignConflict, terminal: true}
		}
		if foundRequired && verificationFailed {
			return observationDecision{state: PublishIndeterminate, terminal: true}
		}
		return observationDecision{state: PublishForeignConflict, terminal: true}
	}
	return observationDecision{state: PublishIndeterminate}
}

func (prepared preparedPublish) matchesPublishedBundle(verified VerifiedPublishBundle) bool {
	statement := verified.Statement
	if verified.RunInvocationURI == "" || verified.RunInvocationURI != statement.Predicate.RunDetails.Metadata.InvocationID ||
		statement.Type != provenance.StatementType || statement.PredicateType != provenance.PredicateType ||
		statement.Predicate.BuildDefinition.BuildType != NPMBuildType || len(statement.Subject) != 1 {
		return false
	}
	invocation, err := identity.ParseRunInvocationURI(verified.RunInvocationURI, prepared.request.SourceRepositoryURI)
	if err != nil || invocation.RunID != prepared.request.RunID {
		return false
	}
	subject := statement.Subject[0]
	return subject.Name == prepared.statement.Subject[0].Name && len(subject.Digest) == 2 &&
		subject.Digest["sha256"] == prepared.request.TarballSHA256.String() &&
		subject.Digest["sha512"] == prepared.request.TarballSHA512.String()
}

func (prepared preparedPublish) runNPM(ctx context.Context) ([]byte, error) {
	tarballBytes, err := verifiedPublishFile(prepared.request.TarballPath, ".tgz", maxTarballSize)
	if err != nil || !digest.SumSHA256(tarballBytes).Equal(prepared.request.TarballSHA256) ||
		!digest.SumSHA512(tarballBytes).Equal(prepared.request.TarballSHA512) {
		return nil, errors.New("tarball handoff changed before npm invocation")
	}
	if !prepared.request.OIDCExchange.ExpiresAt.After(prepared.now()) {
		return nil, errors.New("trusted-publisher exchange expired before npm invocation")
	}
	bundleBytes, err := verifiedPublishFile(prepared.request.BundlePath, ".intoto.jsonl", maxPublishBundleSize)
	if err != nil || !digest.SumSHA256(bundleBytes).Equal(prepared.request.BundleSHA256) {
		return nil, errors.New("bundle handoff changed before npm invocation")
	}
	return prepared.runNPMCommand(ctx, prepared.argv)
}

func (prepared preparedPublish) validateNPMVersion(ctx context.Context) error {
	version, err := prepared.runNPMCommand(ctx, []string{"--version"})
	if err != nil {
		return fmt.Errorf("observe npm version before publish: %w", err)
	}
	if string(version) != prepared.parameters.Runtime.NPMVersion {
		return fmt.Errorf("npm version mismatch: build used %q but publish resolved %q", prepared.parameters.Runtime.NPMVersion, version)
	}
	return nil
}

func (prepared preparedPublish) runNPMCommand(ctx context.Context, arguments []string) ([]byte, error) {
	isolationDirectory, err := os.MkdirTemp("", "windlass-npm-publish-")
	if err != nil {
		return nil, errors.New("create isolated npm configuration directory")
	}
	defer func() { _ = os.RemoveAll(isolationDirectory) }()
	userConfig := filepath.Join(isolationDirectory, "user.npmrc")
	globalConfig := filepath.Join(isolationDirectory, "global.npmrc")
	if err := os.WriteFile(userConfig, nil, 0o600); err != nil {
		return nil, errors.New("create isolated npm user configuration")
	}
	if err := os.WriteFile(globalConfig, nil, 0o600); err != nil {
		return nil, errors.New("create isolated npm global configuration")
	}
	output, err := runPublishCommand(
		ctx,
		isolationDirectory,
		prepared.request.NPMExecutable,
		publishEnvironment(os.Environ(), isolationDirectory, userConfig, globalConfig),
		arguments,
	)
	return []byte(output), err
}

func (prepared preparedPublish) success(mutationAttempted bool) (PublishResult, error) {
	report, err := diagnostic.Build(&prepared.runInvocation, nil, nil)
	if err != nil {
		return PublishResult{}, err
	}
	return PublishResult{State: PublishCommittedAsExpected, MutationAttempted: mutationAttempted, Report: report}, nil
}

func (prepared preparedPublish) failure(state PublishState, mutationAttempted bool, id, check string, cause error) (PublishResult, error) {
	return publishFailureWithInvocation(prepared.runInvocation, state, mutationAttempted, id, check, cause)
}

func publishFailure(request PublishRequest, state PublishState, mutationAttempted bool, id, check string, cause error) (PublishResult, error) {
	runInvocation := ""
	if request.SourceRepositoryURI != "" && request.RunID != "" && request.RunAttempt != "" {
		candidate, err := identity.NewRunInvocationURI(request.SourceRepositoryURI, request.RunID, request.RunAttempt)
		if err == nil {
			runInvocation = candidate
		}
	}
	return publishFailureWithInvocation(runInvocation, state, mutationAttempted, id, check, cause)
}

func publishFailureWithInvocation(runInvocation string, state PublishState, mutationAttempted bool, id, check string, cause error) (PublishResult, error) {
	entry, err := diagnostic.New(id, check, cause.Error())
	if err != nil {
		return PublishResult{}, err
	}
	var invocation *string
	if runInvocation != "" {
		invocation = &runInvocation
	}
	report, err := diagnostic.Build(invocation, []diagnostic.Diagnostic{entry}, nil)
	if err != nil {
		return PublishResult{}, err
	}
	result := PublishResult{State: state, MutationAttempted: mutationAttempted, Report: report}
	return result, &PublishError{Result: result, Cause: cause}
}

func verifiedPublishFile(path, suffix string, maximum int64) ([]byte, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path || !strings.HasSuffix(filepath.Base(path), suffix) {
		return nil, errors.New("artifact path must be absolute, canonical, and have the expected suffix")
	}
	if err := handoff.ValidateSafeBasename(filepath.Base(path)); err != nil {
		return nil, err
	}
	return readBoundedRegularFile(path, maximum)
}

func sha512SRI(value digest.SHA512) string {
	return "sha512-" + base64.StdEncoding.EncodeToString(value[:])
}

func normalizeSHA512SRI(value string) (string, bool) {
	encoded, found := strings.CutPrefix(value, "sha512-")
	if !found || encoded == "" || strings.TrimSpace(value) != value {
		return "", false
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(decoded) != 64 {
		return "", false
	}
	return "sha512-" + base64.StdEncoding.EncodeToString(decoded), true
}

func registryTarballMatches(rawURL, expectedBasename string) bool {
	parsed, err := url.Parse(rawURL)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil &&
		parsed.RawQuery == "" && parsed.Fragment == "" && filepath.Base(parsed.Path) == expectedBasename
}

func definitivePublishError(output []byte) string {
	switch npmErrorCode(output) {
	case "eneedauth", "e404":
		return IDTrustedPublisherMismatch
	case "e401", "e403":
		return IDMutationPermissionDenied
	default:
		return ""
	}
}

func npmErrorCode(output []byte) string {
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(strings.ToLower(line))
		if len(fields) == 4 && fields[0] == "npm" && fields[1] == "error" && fields[2] == "code" {
			return fields[3]
		}
		if len(fields) == 4 && fields[0] == "npm" && fields[1] == "err!" && fields[2] == "code" {
			return fields[3]
		}
	}
	return ""
}

func publishEnvironment(environment []string, isolationDirectory, userConfig, globalConfig string) []string {
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		upper := strings.ToUpper(name)
		if upper == "NPM_TOKEN" || upper == "NODE_AUTH_TOKEN" || upper == "NPM_AUTH_TOKEN" ||
			upper == "NODE_OPTIONS" || upper == "HOME" || upper == "USERPROFILE" || upper == "XDG_CONFIG_HOME" ||
			strings.HasPrefix(upper, "NPM_CONFIG_") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered,
		"HOME="+isolationDirectory,
		"USERPROFILE="+isolationDirectory,
		"XDG_CONFIG_HOME="+isolationDirectory,
		"NPM_CONFIG_USERCONFIG="+userConfig,
		"NPM_CONFIG_GLOBALCONFIG="+globalConfig,
	)
}

func positiveDecimal(value string) bool {
	if value == "" || value[0] == '0' {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func diagnosticIDOr(err error, fallback string) string {
	type identified interface{ DiagnosticID() string }
	var value identified
	if errors.As(err, &value) {
		if _, registered := diagnostic.Lookup(value.DiagnosticID()); registered {
			return value.DiagnosticID()
		}
	}
	return fallback
}
