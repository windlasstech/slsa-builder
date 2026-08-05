package provenance

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/digest"
)

const (
	// StatementType is the only in-toto Statement version accepted by the common contract.
	StatementType = "https://in-toto.io/Statement/v1"
	// PredicateType is the SLSA provenance v1 predicate identifier.
	PredicateType = "https://slsa.dev/provenance/v1"
)

// Statement is the closed common in-toto Statement shape emitted by producer profiles.
type Statement struct {
	Type          string    `json:"_type"`
	Subject       []Subject `json:"subject"`
	PredicateType string    `json:"predicateType"`
	Predicate     Predicate `json:"predicate"`
}

// Subject binds one profile-defined artifact name to profile-defined digest algorithms.
type Subject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// SHA256 returns the typed SHA-256 value when the profile supplied one.
func (subject Subject) SHA256() (digest.SHA256, bool, error) {
	encoded, present := subject.Digest["sha256"]
	if !present {
		return digest.SHA256{}, false, nil
	}
	parsed, err := digest.ParseSHA256(encoded)
	if err != nil {
		return digest.SHA256{}, true, fmt.Errorf("subject sha256: %w", err)
	}
	return parsed, true, nil
}

// SHA512 returns the typed SHA-512 value when the profile supplied one.
func (subject Subject) SHA512() (digest.SHA512, bool, error) {
	encoded, present := subject.Digest["sha512"]
	if !present {
		return digest.SHA512{}, false, nil
	}
	parsed, err := digest.ParseSHA512(encoded)
	if err != nil {
		return digest.SHA512{}, true, fmt.Errorf("subject sha512: %w", err)
	}
	return parsed, true, nil
}

// Predicate is the closed common SLSA provenance v1 predicate shape.
type Predicate struct {
	BuildDefinition BuildDefinition `json:"buildDefinition"`
	RunDetails      RunDetails      `json:"runDetails"`
}

// BuildDefinition contains the common build definition and profile-owned extension values.
type BuildDefinition struct {
	BuildType            string               `json:"buildType"`
	ExternalParameters   json.RawMessage      `json:"externalParameters"`
	InternalParameters   json.RawMessage      `json:"internalParameters"`
	ResolvedDependencies []ResourceDescriptor `json:"resolvedDependencies"`
}

// ResourceDescriptor is the closed descriptor envelope whose semantics are defined by a profile.
type ResourceDescriptor struct {
	Name        string                     `json:"name"`
	URI         string                     `json:"uri,omitempty"`
	Digest      map[string]string          `json:"digest,omitempty"`
	Annotations map[string]json.RawMessage `json:"annotations,omitempty"`
}

// RunDetails contains the builder and invocation metadata.
type RunDetails struct {
	Builder  Builder  `json:"builder"`
	Metadata Metadata `json:"metadata"`
}

// Builder records the immutable builder identity and closed platform component fields.
type Builder struct {
	ID                  string              `json:"id"`
	Version             map[string]string   `json:"version"`
	BuilderDependencies []BuilderDependency `json:"builderDependencies"`
}

// BuilderDependency records one orchestrator-side dependency.
type BuilderDependency struct {
	URI         string            `json:"uri"`
	Digest      map[string]string `json:"digest"`
	Annotations map[string]string `json:"annotations"`
}

// Metadata records the canonical run identity and whole-second event times.
type Metadata struct {
	InvocationID string `json:"invocationId"`
	StartedOn    string `json:"startedOn"`
	FinishedOn   string `json:"finishedOn"`
}

// DecodeStatement strictly decodes one duplicate-free Statement and rejects unknown common fields.
func DecodeStatement(data []byte) (Statement, error) {
	if err := canonicaljson.Validate(data); err != nil {
		return Statement{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var statement Statement
	if err := decoder.Decode(&statement); err != nil {
		return Statement{}, fmt.Errorf("decode closed SLSA Statement: %w", err)
	}
	return statement, nil
}

// CanonicalJSON serializes the Statement as RFC 8785 JCS bytes.
func (statement Statement) CanonicalJSON() ([]byte, error) {
	return canonicalJSON(statement)
}

// CanonicalJSON serializes the predicate as RFC 8785 JCS bytes.
func (predicate Predicate) CanonicalJSON() ([]byte, error) {
	return canonicalJSON(predicate)
}

func canonicalJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode provenance JSON: %w", err)
	}
	canonical, err := canonicaljson.Canonicalize(encoded)
	if err != nil {
		return nil, fmt.Errorf("canonicalize provenance JSON: %w", err)
	}
	return canonical, nil
}
