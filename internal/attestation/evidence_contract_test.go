package attestation

import (
	"bytes"
	"crypto/x509"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/sigstore/sigstore-go/pkg/verify"
)

func TestVerificationErrorPreservesCause(t *testing.T) {
	t.Parallel()
	cause := &url.Error{Op: "read", URL: "fixture", Err: errors.New("unavailable")}
	for _, test := range []struct {
		name  string
		cause error
		want  string
	}{
		{"without cause", nil, idInputUnavailable + ": missing bytes"},
		{"with typed cause", cause, idInputUnavailable + ": missing bytes: read \"fixture\": unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := newError(idInputUnavailable, "bundle", "missing bytes", test.cause)
			if got := err.Error(); got != test.want {
				t.Fatalf("Error() = %q, want %q", got, test.want)
			}
			requireDiagnosticID(t, err, idInputUnavailable)
			if got := errors.Unwrap(err); got != test.cause {
				t.Fatalf("Unwrap() = %v, want %v", got, test.cause)
			}
			if test.cause != nil {
				if !errors.Is(err, cause) {
					t.Fatalf("errors.Is(%v, cause) = false, want true", err)
				}
				var got *url.Error
				if !errors.As(err, &got) || got != cause {
					t.Fatalf("errors.As cause = %v, want %v", got, cause)
				}
			}
		})
	}
}

func TestEvidenceAccessorsReturnDefensiveCopies(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"result bundle", "result statement", "parsed bundle", "parsed statement"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want := []byte("preserved evidence")
			result := Result{bundle: bytes.Clone(want), statement: bytes.Clone(want)}
			parsed := ParsedBundle{raw: bytes.Clone(want), statement: bytes.Clone(want)}
			accessors := map[string]func() []byte{
				"result bundle": result.BundleBytes, "result statement": result.StatementBytes,
				"parsed bundle": parsed.BundleBytes, "parsed statement": parsed.StatementBytes,
			}
			accessor := accessors[name]
			got := accessor()
			if !bytes.Equal(got, want) {
				t.Fatalf("initial evidence = %q, want %q", got, want)
			}
			got[0] ^= 0xff
			if reread := accessor(); !bytes.Equal(reread, want) {
				t.Fatalf("evidence after mutating returned slice = %q, want %q", reread, want)
			}
		})
	}
}

func TestClassifyVerificationError(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ message, id string }{
		{"SIGNED CERTIFICATE TIMESTAMP invalid", idMissingSCT},
		{"SCT missing", idMissingSCT},
		{"integrated time outside certificate validity", idSignatureTimeViolation},
		{"signature time outside validity", idSignatureTimeViolation},
		{"log inclusion invalid", idMissingRekorEntry},
		{"transparency log missing", idMissingRekorEntry},
		{"identity mismatch", idSignerIdentityUntrusted},
		{"signature invalid", idSignatureMismatch},
	} {
		t.Run(test.message, func(t *testing.T) {
			t.Parallel()
			cause := errors.New(test.message)
			err := classifyVerificationError(cause)
			requireDiagnosticID(t, err, test.id)
			if !errors.Is(err, cause) {
				t.Fatalf("classified error %v lost original cause %v", err, cause)
			}
		})
	}
}

func TestVerifiedSigningTimeRequiresValidTlog(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	certificate := &x509.Certificate{NotBefore: start, NotAfter: end}
	for _, test := range []struct {
		name       string
		timestamps []verify.TimestampVerificationResult
		want       time.Time
	}{
		{"no timestamps", nil, time.Time{}},
		{"TSA alone cannot replace Tlog", []verify.TimestampVerificationResult{{Type: "TimestampAuthority", Timestamp: start}}, time.Time{}},
		{"ignores invalid TSA before valid Tlog", []verify.TimestampVerificationResult{{Type: "TimestampAuthority", Timestamp: end.Add(time.Hour)}, {Type: "Tlog", Timestamp: start.In(time.FixedZone("offset", 3600))}}, start},
		{"inclusive not after", []verify.TimestampVerificationResult{{Type: "Tlog", Timestamp: end}}, end},
		{"before validity", []verify.TimestampVerificationResult{{Type: "Tlog", Timestamp: start.Add(-time.Nanosecond)}}, time.Time{}},
		{"after validity", []verify.TimestampVerificationResult{{Type: "Tlog", Timestamp: end.Add(time.Nanosecond)}}, time.Time{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := &verify.VerificationResult{VerifiedTimestamps: test.timestamps}
			got, err := verifiedSigningTime(result, certificate)
			if test.want.IsZero() {
				requireDiagnosticID(t, err, idSignatureTimeViolation)
				if !got.IsZero() {
					t.Fatalf("rejected signing time = %s, want zero", got)
				}
				return
			}
			if err != nil || !got.Equal(test.want) || got.Location() != time.UTC {
				t.Fatalf("signing time = %s (%v), error = %v, want %s in UTC", got, got.Location(), err, test.want)
			}
		})
	}
}
