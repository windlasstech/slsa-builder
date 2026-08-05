package canonicaljson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"unicode/utf8"
)

// DuplicateJSONMemberID is the stable diagnostic ID for ambiguous JSON object members.
const DuplicateJSONMemberID = "windlass.verify.error.duplicate-json-member"

const maxNestingDepth = 10_000

// DuplicateMemberError reports a repeated decoded JSON object member name.
type DuplicateMemberError struct {
	Member string
	Offset int64
}

// Error describes the duplicate member and the decoder offset immediately after its name.
func (e *DuplicateMemberError) Error() string {
	return fmt.Sprintf("duplicate JSON object member %q at byte offset %d", e.Member, e.Offset)
}

// DiagnosticID maps the error to the stable verification diagnostic contract.
func (*DuplicateMemberError) DiagnosticID() string {
	return DuplicateJSONMemberID
}

// Validate strictly parses exactly one complete JSON value and rejects duplicate object members.
func Validate(data []byte) error {
	_, err := parseStrict(data)
	return err
}

func parseStrict(data []byte) (any, error) {
	if !utf8.Valid(data) {
		return nil, errors.New("JSON input is not valid UTF-8")
	}
	if err := validateUnicodeEscapes(data); err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	value, err := parseValue(decoder, 0)
	if err != nil {
		return nil, fmt.Errorf("parse JSON value: %w", err)
	}

	trailing, err := decoder.Token()
	if errors.Is(err, io.EOF) {
		return value, nil
	}
	if err != nil {
		return nil, fmt.Errorf("parse trailing JSON data: %w", err)
	}
	return nil, fmt.Errorf("parse JSON value: unexpected trailing token %v", trailing)
}

func parseValue(decoder *json.Decoder, depth int) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	switch token := token.(type) {
	case json.Delim:
		if depth >= maxNestingDepth {
			return nil, fmt.Errorf("JSON nesting depth exceeds %d", maxNestingDepth)
		}
		switch token {
		case '{':
			return parseObject(decoder, depth+1)
		case '[':
			return parseArray(decoder, depth+1)
		default:
			return nil, fmt.Errorf("unexpected delimiter %q", token)
		}
	case json.Number:
		number, err := token.Float64()
		if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
			return nil, fmt.Errorf("number %q is not an IEEE 754 double: %w", token, err)
		}
		return number, nil
	case nil, bool, string:
		return token, nil
	default:
		return nil, fmt.Errorf("unexpected JSON token of type %T", token)
	}
}

func parseObject(decoder *json.Decoder, depth int) (map[string]any, error) {
	object := make(map[string]any)
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		member, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("object member name has type %T", token)
		}
		if _, exists := seen[member]; exists {
			return nil, &DuplicateMemberError{Member: member, Offset: decoder.InputOffset()}
		}
		seen[member] = struct{}{}

		value, err := parseValue(decoder, depth)
		if err != nil {
			return nil, err
		}
		object[member] = value
	}
	if err := consumeClosingDelimiter(decoder, '}'); err != nil {
		return nil, err
	}
	return object, nil
}

func parseArray(decoder *json.Decoder, depth int) ([]any, error) {
	array := make([]any, 0)
	for decoder.More() {
		value, err := parseValue(decoder, depth)
		if err != nil {
			return nil, err
		}
		array = append(array, value)
	}
	if err := consumeClosingDelimiter(decoder, ']'); err != nil {
		return nil, err
	}
	return array, nil
}

func consumeClosingDelimiter(decoder *json.Decoder, want json.Delim) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != want {
		return fmt.Errorf("unexpected closing token %v, want %q", token, want)
	}
	return nil
}

func validateUnicodeEscapes(data []byte) error {
	inString := false
	for offset := 0; offset < len(data); offset++ {
		switch data[offset] {
		case '"':
			inString = !inString
		case '\\':
			if !inString {
				continue
			}
			if offset+1 >= len(data) {
				return fmt.Errorf("unterminated JSON escape at byte offset %d", offset)
			}
			if data[offset+1] != 'u' {
				offset++
				continue
			}

			first, err := decodeCodeUnit(data, offset+2)
			if err != nil {
				return err
			}
			offset += 5
			switch {
			case first >= 0xd800 && first <= 0xdbff:
				if offset+6 >= len(data) || data[offset+1] != '\\' || data[offset+2] != 'u' {
					return fmt.Errorf("unpaired high surrogate at byte offset %d", offset-5)
				}
				second, err := decodeCodeUnit(data, offset+3)
				if err != nil {
					return err
				}
				if second < 0xdc00 || second > 0xdfff {
					return fmt.Errorf("unpaired high surrogate at byte offset %d", offset-5)
				}
				offset += 6
			case first >= 0xdc00 && first <= 0xdfff:
				return fmt.Errorf("unpaired low surrogate at byte offset %d", offset-5)
			}
		}
	}
	return nil
}

func decodeCodeUnit(data []byte, start int) (uint16, error) {
	if start+4 > len(data) {
		return 0, fmt.Errorf("incomplete Unicode escape at byte offset %d", start-2)
	}

	var codeUnit uint16
	for offset := start; offset < start+4; offset++ {
		hex, ok := hexValue(data[offset])
		if !ok {
			return 0, fmt.Errorf("invalid Unicode escape at byte offset %d", start-2)
		}
		codeUnit = codeUnit*16 + uint16(hex)
	}
	return codeUnit, nil
}

func hexValue(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}
