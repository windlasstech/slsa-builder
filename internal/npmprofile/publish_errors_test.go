package npmprofile

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

func TestPublishErrorMethods(t *testing.T) {
	t.Parallel()
	for _, withCause := range []bool{false, true} {
		for _, withReport := range []bool{false, true} {
			t.Run(fmt.Sprintf("cause=%t/report=%t", withCause, withReport), func(t *testing.T) {
				t.Parallel()
				// Given an independently optional cause and primary diagnostic.
				value := &PublishError{}
				wantMessage, wantID := "npm publish failed", ""
				if withCause {
					value.Cause = context.Canceled
					wantMessage = context.Canceled.Error()
				}
				if withReport {
					wantID = IDAlreadyPublishedVersion
					value.Result.Report.PrimaryID = &wantID
				}
				// When the error is wrapped by a caller.
				wrapped := fmt.Errorf("publish: %w", value)
				// Then its message, classification, and cause remain accessible.
				var got *PublishError
				if !errors.As(wrapped, &got) || got != value || got.Error() != wantMessage || got.DiagnosticID() != wantID {
					t.Fatalf("error methods = %#v, want message %q and ID %q", got, wantMessage, wantID)
				}
				if errors.Is(wrapped, context.Canceled) != withCause || !errors.Is(value.Unwrap(), value.Cause) {
					t.Fatal("error chain did not preserve optional cause")
				}
			})
		}
	}
}

func TestPublishFailurePreservesResult(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, repository, run, attempt, invocation string
	}{
		{"valid", observedRepository, "123", "2", observedRepository + "/actions/runs/123/attempts/2"},
		{"missing repository", "", "123", "2", ""},
		{"missing run", observedRepository, "", "2", ""},
		{"missing attempt", observedRepository, "123", "", ""},
		{"invalid repository", "not-a-repository", "123", "2", ""},
		{"invalid run", observedRepository, "x", "2", ""},
		{"invalid attempt", observedRepository, "123", "0", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for _, state := range []PublishState{PublishAbsent, PublishIndeterminate, PublishForeignConflict} {
				for _, mutated := range []bool{false, true} {
					// Given source/run fields and a classified failure.
					request := PublishRequest{SourceRepositoryURI: test.repository, RunID: test.run, RunAttempt: test.attempt}
					// When failure constructs the report.
					result, err := publishFailure(request, state, mutated, IDAlreadyPublishedVersion, "npm.publish.test", context.Canceled)
					// Then state, cause, and only a valid invocation survive.
					requireNPMDiagnostic(t, err, IDAlreadyPublishedVersion)
					var failure *PublishError
					if !errors.As(err, &failure) || !errors.Is(err, context.Canceled) || !reflect.DeepEqual(failure.Result, result) || result.State != state || result.MutationAttempted != mutated {
						t.Fatalf("failure = %#v, %v; want state %s, mutation %t", result, err, state, mutated)
					}
					if test.invocation == "" {
						if result.Report.RunInvocation != nil {
							t.Fatalf("invalid invocation retained: %q", *result.Report.RunInvocation)
						}
					} else if result.Report.RunInvocation == nil || *result.Report.RunInvocation != test.invocation {
						t.Fatalf("invocation = %v, want %q", result.Report.RunInvocation, test.invocation)
					}
				}
			}
		})
	}
}

func TestSleepContext(t *testing.T) {
	t.Parallel()
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprintf("canceled=%t", canceled), func(t *testing.T) {
			t.Parallel()
			// Given a ready timer or a pre-canceled context, never both.
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			duration := time.Duration(0)
			var want error
			if canceled {
				cancel()
				duration, want = time.Hour, context.Canceled
			}
			// When waiting, then return immediately with the correct result.
			if err := sleepContext(ctx, duration); !errors.Is(err, want) {
				t.Fatalf("sleepContext() = %v, want %v", err, want)
			}
		})
	}
}

func TestPositiveDecimal(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "0", "01", "-1", "+1", "1x", "1 ", "١", "1", "123456789"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			want := value == "1" || value == "123456789"
			if got := positiveDecimal(value); got != want {
				t.Fatalf("positiveDecimal(%q) = %t, want %t", value, got, want)
			}
		})
	}
}

func TestDiagnosticIDOr(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, IDNPMSubjectMismatch},
		{"ordinary", context.Canceled, IDNPMSubjectMismatch},
		{"empty ID", &PublishError{}, IDNPMSubjectMismatch},
		{"unregistered", &PublishError{Result: PublishResult{Report: diagnostic.Report{PrimaryID: testStringPointer("unregistered")}}}, IDNPMSubjectMismatch},
		{"wrapped registered", fmt.Errorf("wrapped: %w", digestMismatch(ManagerPNPM, "sha512", "mismatch")), IDPackageManagerDigestMismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := diagnosticIDOr(test.err, IDNPMSubjectMismatch); got != test.want {
				t.Fatalf("diagnosticIDOr() = %q, want %q", got, test.want)
			}
		})
	}
}
