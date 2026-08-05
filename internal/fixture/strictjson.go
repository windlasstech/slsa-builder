package fixture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// rejectDuplicateMembers is intentionally local to the fixture harness. C01 will provide the
// shared internal/canonicaljson strict parser; this function is the migration seam until then.
func rejectDuplicateMembers(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanJSONValue(decoder); err != nil {
		return err
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return fmt.Errorf("read trailing JSON: %w", err)
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("read JSON token: %w", err)
	}

	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delimiter {
	case '{':
		members := make(map[string]struct{})
		for decoder.More() {
			nameToken, tokenErr := decoder.Token()
			if tokenErr != nil {
				return fmt.Errorf("read JSON member: %w", tokenErr)
			}
			name, nameOK := nameToken.(string)
			if !nameOK {
				return fmt.Errorf("JSON object member name is not a string")
			}
			if _, exists := members[name]; exists {
				return fmt.Errorf("duplicate JSON member %q", name)
			}
			members[name] = struct{}{}
			if valueErr := scanJSONValue(decoder); valueErr != nil {
				return valueErr
			}
		}
		if _, endErr := decoder.Token(); endErr != nil {
			return fmt.Errorf("close JSON object: %w", endErr)
		}
	case '[':
		for decoder.More() {
			if valueErr := scanJSONValue(decoder); valueErr != nil {
				return valueErr
			}
		}
		if _, endErr := decoder.Token(); endErr != nil {
			return fmt.Errorf("close JSON array: %w", endErr)
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}

	return nil
}
