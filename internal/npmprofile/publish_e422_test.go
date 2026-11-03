package npmprofile

import (
	"context"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

const testE422Output = "npm error code E422\nnpm error 422 Unprocessable Entity"

type publishDiagnosticExpectation struct {
	id      string
	check   string
	message string
}

func TestPublishE422AbsentWithinReducedBudget(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		options publishFixtureOptions
	}{
		{
			name: "clean-absence",
			options: publishFixtureOptions{
				initialVersionAbsent: true, npmExitCode: 1, npmError: testE422Output, readbackAbsent: true,
			},
		},
		{
			name: "clean-absence-then-errors",
			options: publishFixtureOptions{
				initialVersionAbsent: true, npmExitCode: 1, npmError: testE422Output,
				readbackAbsent: true, readbackAbsentPolls: 2, readbackIndeterminate: true,
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newPublishFixture(t, testCase.options)
			result, err := Publish(context.Background(), fixture.request)
			if err == nil {
				t.Fatal("Publish() error = nil, want determinate E422 absence")
			}
			if result.State != PublishAbsent || !result.MutationAttempted {
				t.Fatalf("Publish() result = %#v, want terminal absent after one mutation", result)
			}
			if result.Report.PrimaryID == nil || *result.Report.PrimaryID != "windlass.verify.error.publish-rejected-unprocessable" {
				t.Fatalf("Publish() primary diagnostic = %v, want publish-rejected-unprocessable", result.Report.PrimaryID)
			}
			requirePublishDiagnostics(t, result.Report, []publishDiagnosticExpectation{{
				id:      "windlass.verify.error.publish-rejected-unprocessable",
				check:   "npm.publish.readback",
				message: "npm rejected the publication as unprocessable and no committed publication was observed",
			}})
			fixture.requireInvocations(t, fixture.expectedMutationInvocations())
			if fixture.packumentRequests != 6 || fixture.attestationRequests != 6 || fixture.clock.sleeps != 4 {
				t.Fatalf("polling = packument:%d attestations:%d sleeps:%d, want exact reduced budget 6/6/4", fixture.packumentRequests, fixture.attestationRequests, fixture.clock.sleeps)
			}
		})
	}
}

func TestPublishE422CommittedAfterRejectionWarns(t *testing.T) {
	fixture := newPublishFixture(t, publishFixtureOptions{initialVersionAbsent: true, npmExitCode: 1, npmError: testE422Output})
	result, err := Publish(context.Background(), fixture.request)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.State != PublishCommittedAsExpected || !result.MutationAttempted {
		t.Fatalf("Publish() result = %#v, want committed mutation", result)
	}
	if result.Report.Result != diagnostic.ResultPass || result.Report.ExitCode != 0 || result.Report.PrimaryID != nil {
		t.Fatalf("Publish() report = %#v, want passing warning-only report", result.Report)
	}
	requirePublishDiagnostics(t, result.Report, []publishDiagnosticExpectation{{
		id:      "windlass.verify.warning.publish-committed-after-rejection",
		check:   "npm.publish.readback",
		message: "npm reported E422 but registry read-back observed a committed publication",
	}})
	fixture.requireInvocations(t, fixture.expectedMutationInvocations())
	if fixture.packumentRequests != 2 || fixture.attestationRequests != 2 || fixture.clock.sleeps != 0 {
		t.Fatalf("polling = packument:%d attestations:%d sleeps:%d, want immediate convergence 2/2/0", fixture.packumentRequests, fixture.attestationRequests, fixture.clock.sleeps)
	}
}

func TestPublishE422ForeignConflictAfterRejectionWarns(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		options publishFixtureOptions
	}{
		{name: "foreign-integrity", options: publishFixtureOptions{foreignIntegrity: true}},
		{name: "foreign-run-id", options: publishFixtureOptions{foreignRunID: true}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			options := testCase.options
			options.initialVersionAbsent = true
			options.npmExitCode = 1
			options.npmError = testE422Output
			fixture := newPublishFixture(t, options)
			result, err := Publish(context.Background(), fixture.request)
			if err == nil {
				t.Fatal("Publish() error = nil, want foreign conflict")
			}
			if result.State != PublishForeignConflict || !result.MutationAttempted {
				t.Fatalf("Publish() result = %#v, want post-mutation foreign conflict", result)
			}
			if result.Report.PrimaryID == nil || *result.Report.PrimaryID != IDRegistryLinkageMismatch {
				t.Fatalf("Publish() primary diagnostic = %v, want registry-linkage-mismatch", result.Report.PrimaryID)
			}
			requirePublishDiagnostics(t, result.Report, []publishDiagnosticExpectation{
				{id: IDRegistryLinkageMismatch, check: "npm.publish.readback", message: "published registry evidence conflicts with the verified handoff"},
				{id: "windlass.verify.warning.publish-committed-after-rejection", check: "npm.publish.readback", message: "npm reported E422 but registry read-back observed a committed publication"},
			})
			fixture.requireInvocations(t, fixture.expectedMutationInvocations())
			if fixture.packumentRequests != 2 || fixture.attestationRequests != 2 || fixture.clock.sleeps != 0 {
				t.Fatalf("polling = packument:%d attestations:%d sleeps:%d, want immediate conflict 2/2/0", fixture.packumentRequests, fixture.attestationRequests, fixture.clock.sleeps)
			}
		})
	}
}

func TestPublishE422IndeterminateAfterFullBudgetWarns(t *testing.T) {
	fixture := newPublishFixture(t, publishFixtureOptions{
		initialVersionAbsent: true, npmExitCode: 1, npmError: testE422Output, readbackAttestationIndeterminate: true,
	})
	result, err := Publish(context.Background(), fixture.request)
	if err == nil {
		t.Fatal("Publish() error = nil, want indeterminate read-back")
	}
	if result.State != PublishIndeterminate || !result.MutationAttempted {
		t.Fatalf("Publish() result = %#v, want one ambiguous mutation", result)
	}
	if result.Report.PrimaryID == nil || *result.Report.PrimaryID != IDRegistryLinkageMismatch {
		t.Fatalf("Publish() primary diagnostic = %v, want registry-linkage-mismatch", result.Report.PrimaryID)
	}
	requirePublishDiagnostics(t, result.Report, []publishDiagnosticExpectation{
		{id: IDRegistryLinkageMismatch, check: "npm.publish.readback", message: "publication outcome is indeterminate"},
		{id: "windlass.verify.warning.publish-committed-after-rejection", check: "npm.publish.readback", message: "npm reported E422 but registry read-back observed a committed publication"},
	})
	fixture.requireInvocations(t, fixture.expectedMutationInvocations())
	if fixture.packumentRequests != 22 || fixture.attestationRequests != 22 || fixture.clock.sleeps != 20 {
		t.Fatalf("polling = packument:%d attestations:%d sleeps:%d, want one continuous full budget 22/22/20", fixture.packumentRequests, fixture.attestationRequests, fixture.clock.sleeps)
	}
}

func TestPublishE422ReducedErrorsUseFullBudget(t *testing.T) {
	fixture := newPublishFixture(t, publishFixtureOptions{
		initialVersionAbsent: true, npmExitCode: 1, npmError: testE422Output, readbackIndeterminate: true,
	})
	result, err := Publish(context.Background(), fixture.request)
	if err == nil {
		t.Fatal("Publish() error = nil, want indeterminate read-back")
	}
	if result.State != PublishIndeterminate || !result.MutationAttempted {
		t.Fatalf("Publish() result = %#v, want one ambiguous mutation", result)
	}
	requirePublishDiagnostics(t, result.Report, []publishDiagnosticExpectation{{
		id: IDRegistryLinkageMismatch, check: "npm.publish.readback", message: "publication outcome is indeterminate",
	}})
	fixture.requireInvocations(t, fixture.expectedMutationInvocations())
	if fixture.packumentRequests != 22 || fixture.attestationRequests != 22 || fixture.clock.sleeps != 20 {
		t.Fatalf("polling = packument:%d attestations:%d sleeps:%d, want standing full budget 22/22/20", fixture.packumentRequests, fixture.attestationRequests, fixture.clock.sleeps)
	}
}

func TestPublishE422VersionAppearingAfterReducedWindowWarns(t *testing.T) {
	fixture := newPublishFixture(t, publishFixtureOptions{
		initialVersionAbsent: true, npmExitCode: 1, npmError: testE422Output, readbackPendingPolls: 5,
	})
	result, err := Publish(context.Background(), fixture.request)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.State != PublishCommittedAsExpected || !result.MutationAttempted {
		t.Fatalf("Publish() result = %#v, want late committed mutation", result)
	}
	requirePublishDiagnostics(t, result.Report, []publishDiagnosticExpectation{{
		id:      "windlass.verify.warning.publish-committed-after-rejection",
		check:   "npm.publish.readback",
		message: "npm reported E422 but registry read-back observed a committed publication",
	}})
	fixture.requireInvocations(t, fixture.expectedMutationInvocations())
	if fixture.packumentRequests != 7 || fixture.attestationRequests != 7 || fixture.clock.sleeps != 5 {
		t.Fatalf("polling = packument:%d attestations:%d sleeps:%d, want convergence at 75 seconds 7/7/5", fixture.packumentRequests, fixture.attestationRequests, fixture.clock.sleeps)
	}
}

func requirePublishDiagnostics(t *testing.T, report diagnostic.Report, want []publishDiagnosticExpectation) {
	t.Helper()
	if len(report.Diagnostics) != len(want) {
		t.Fatalf("diagnostics = %#v, want exactly %#v", report.Diagnostics, want)
	}
	for index, expected := range want {
		got := report.Diagnostics[index]
		if got.ID != expected.id || got.Check != expected.check || got.Message != expected.message {
			t.Errorf("diagnostics[%d] = %#v, want id:%q check:%q message:%q", index, got, expected.id, expected.check, expected.message)
		}
	}
}
