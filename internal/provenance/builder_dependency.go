package provenance

import (
	"bytes"
	"encoding/json"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

// BuilderDependency is the ADR 0113 descriptor envelope. Semantic defects are
// retained until the owning profile or common validator can classify them.
type BuilderDependency struct {
	Name          string            `json:"name,omitempty"`
	URI           string            `json:"uri,omitempty"`
	Digest        map[string]string `json:"digest,omitempty"`
	MediaType     string            `json:"mediaType,omitempty"`
	Annotations   json.RawMessage   `json:"annotations,omitempty"`
	malformed     map[string]json.RawMessage
	unexpected    map[string]json.RawMessage
	invalidObject bool
}

// HasUnexpectedFields reports effective members outside the descriptor envelope.
func (dependency BuilderDependency) HasUnexpectedFields() bool {
	return len(dependency.unexpected) != 0
}

// HasMalformedMembers reports semantic type defects, including non-object input.
func (dependency BuilderDependency) HasMalformedMembers() bool {
	return dependency.invalidObject || len(dependency.malformed) != 0
}

// UnmarshalJSON rejects duplicate keys but defers semantic type failures.
func (dependency *BuilderDependency) UnmarshalJSON(data []byte) error {
	if err := canonicaljson.Validate(data); err != nil {
		return err
	}
	var members map[string]json.RawMessage
	parsed := BuilderDependency{}
	if err := json.Unmarshal(data, &members); err != nil || members == nil {
		parsed.invalidObject = true
		*dependency = parsed
		return nil
	}
	parsed.malformed = make(map[string]json.RawMessage)
	parsed.unexpected = make(map[string]json.RawMessage)
	for name, raw := range members {
		if !effectiveBuilderMember(raw) {
			continue
		}
		var err error
		switch name {
		case "name":
			err = json.Unmarshal(raw, &parsed.Name)
		case "uri":
			err = json.Unmarshal(raw, &parsed.URI)
		case "digest":
			err = json.Unmarshal(raw, &parsed.Digest)
		case "mediaType":
			err = json.Unmarshal(raw, &parsed.MediaType)
		case "annotations":
			parsed.Annotations = bytes.Clone(raw)
		default:
			parsed.unexpected[name] = raw
		}
		if err != nil {
			parsed.malformed[name] = raw
		}
	}
	*dependency = parsed
	return nil
}

// MarshalJSON omits unset optional members while retaining invalid evidence.
// Producers must validate before signing; encoding must not launder a rejected input.
func (dependency BuilderDependency) MarshalJSON() ([]byte, error) {
	if dependency.invalidObject {
		return []byte("null"), nil
	}
	type wire BuilderDependency
	encoded, err := json.Marshal(wire(dependency))
	if err != nil {
		return nil, err
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &members); err != nil {
		return nil, err
	}
	for key, value := range dependency.malformed {
		members[key] = value
	}
	for key, value := range dependency.unexpected {
		members[key] = value
	}
	return json.Marshal(members)
}

func effectiveBuilderMember(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return true
	}
	switch compact.String() {
	case "null", `""`, "[]", "{}":
		return false
	default:
		return true
	}
}

func cloneBuilderDependencies(dependencies []BuilderDependency) []BuilderDependency {
	cloned := make([]BuilderDependency, len(dependencies))
	for index, dependency := range dependencies {
		cloned[index] = dependency
		cloned[index].Digest = cloneStringMap(dependency.Digest)
		cloned[index].Annotations = bytes.Clone(dependency.Annotations)
		cloned[index].malformed = cloneBuilderRawMembers(dependency.malformed)
		cloned[index].unexpected = cloneBuilderRawMembers(dependency.unexpected)
	}
	return cloned
}

func cloneBuilderRawMembers(members map[string]json.RawMessage) map[string]json.RawMessage {
	if members == nil {
		return nil
	}
	cloned := make(map[string]json.RawMessage, len(members))
	for key, value := range members {
		cloned[key] = bytes.Clone(value)
	}
	return cloned
}
