// Package digest provides fixed-size, lowercase-hex cryptographic digest values.
package digest

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidEncoding identifies a digest that is not exact-length lowercase hexadecimal.
var ErrInvalidEncoding = errors.New("invalid digest encoding")

// SHA256 is a SHA-256 digest value.
type SHA256 [sha256.Size]byte

// ParseSHA256 parses an exact 64-character lowercase hexadecimal SHA-256 value.
func ParseSHA256(encoded string) (SHA256, error) {
	var value SHA256
	if err := decodeLowerHex(value[:], encoded); err != nil {
		return SHA256{}, fmt.Errorf("parse SHA-256: %w", err)
	}
	return value, nil
}

// SumSHA256 computes the SHA-256 digest of data.
func SumSHA256(data []byte) SHA256 {
	return sha256.Sum256(data)
}

// SumSHA256Reader computes the SHA-256 digest of all bytes read from reader.
func SumSHA256Reader(reader io.Reader) (SHA256, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, reader); err != nil {
		return SHA256{}, fmt.Errorf("compute SHA-256: %w", err)
	}

	var value SHA256
	copy(value[:], hasher.Sum(nil))
	return value, nil
}

// String returns the canonical lowercase hexadecimal representation.
func (value SHA256) String() string {
	return hex.EncodeToString(value[:])
}

// Equal compares two SHA-256 values in constant time.
func (value SHA256) Equal(other SHA256) bool {
	return subtle.ConstantTimeCompare(value[:], other[:]) == 1
}

// MarshalText encodes a SHA-256 value as lowercase hexadecimal text.
func (value SHA256) MarshalText() ([]byte, error) {
	return []byte(value.String()), nil
}

// UnmarshalText decodes exact-length lowercase hexadecimal text into a SHA-256 value.
func (value *SHA256) UnmarshalText(text []byte) error {
	parsed, err := ParseSHA256(string(text))
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}

// SHA512 is a SHA-512 digest value.
type SHA512 [sha512.Size]byte

// ParseSHA512 parses an exact 128-character lowercase hexadecimal SHA-512 value.
func ParseSHA512(encoded string) (SHA512, error) {
	var value SHA512
	if err := decodeLowerHex(value[:], encoded); err != nil {
		return SHA512{}, fmt.Errorf("parse SHA-512: %w", err)
	}
	return value, nil
}

// SumSHA512 computes the SHA-512 digest of data.
func SumSHA512(data []byte) SHA512 {
	return sha512.Sum512(data)
}

// SumSHA512Reader computes the SHA-512 digest of all bytes read from reader.
func SumSHA512Reader(reader io.Reader) (SHA512, error) {
	hasher := sha512.New()
	if _, err := io.Copy(hasher, reader); err != nil {
		return SHA512{}, fmt.Errorf("compute SHA-512: %w", err)
	}

	var value SHA512
	copy(value[:], hasher.Sum(nil))
	return value, nil
}

// String returns the canonical lowercase hexadecimal representation.
func (value SHA512) String() string {
	return hex.EncodeToString(value[:])
}

// Equal compares two SHA-512 values in constant time.
func (value SHA512) Equal(other SHA512) bool {
	return subtle.ConstantTimeCompare(value[:], other[:]) == 1
}

// MarshalText encodes a SHA-512 value as lowercase hexadecimal text.
func (value SHA512) MarshalText() ([]byte, error) {
	return []byte(value.String()), nil
}

// UnmarshalText decodes exact-length lowercase hexadecimal text into a SHA-512 value.
func (value *SHA512) UnmarshalText(text []byte) error {
	parsed, err := ParseSHA512(string(text))
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}

func decodeLowerHex(destination []byte, encoded string) error {
	wantLength := hex.EncodedLen(len(destination))
	if len(encoded) != wantLength {
		return fmt.Errorf("%w: got %d characters, want %d", ErrInvalidEncoding, len(encoded), wantLength)
	}
	for _, character := range []byte(encoded) {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return fmt.Errorf("%w: expected lowercase hexadecimal", ErrInvalidEncoding)
		}
	}
	if _, err := hex.Decode(destination, []byte(encoded)); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEncoding, err)
	}
	return nil
}
