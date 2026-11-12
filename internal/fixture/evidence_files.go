package fixture

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CheckEvidence evaluates evidence expectations without changing the corpus or fetching missing inputs.
func CheckEvidence(index Index, indexPath string) (err error) {
	if len(index.Fixtures) == 0 {
		return nil
	}
	absolute, err := filepath.Abs(indexPath)
	if err != nil {
		return fmt.Errorf("resolve fixture index: %w", err)
	}
	directory := filepath.Dir(absolute)
	for current := directory; ; current = filepath.Dir(current) {
		if filepath.Base(current) == "testdata" {
			directory = filepath.Dir(current)
			break
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	root, err := os.OpenRoot(filepath.Join(directory, "testdata"))
	if err != nil {
		return fmt.Errorf("%w: open fixture root: %w", ErrInputUnavailable, err)
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	for _, manifest := range index.Fixtures {
		if err := checkFixtureEvidence(root, manifest); err != nil {
			return fmt.Errorf("fixture %q: %w", manifest.Name, err)
		}
	}
	return nil
}

func checkFixtureEvidence(root *os.Root, manifest Manifest) error {
	document, err := readEvidenceFile(root, manifest.Provenance)
	if err != nil {
		return fmt.Errorf("%w: provenance is unavailable: %w", ErrInputUnavailable, err)
	}
	statement, applicable, err := evidenceStatement(document)
	if err != nil {
		return err
	}
	var evidence []byte
	if manifest.ManifestEvidence != nil {
		evidence, err = readEvidenceFile(root, *manifest.ManifestEvidence)
		if err != nil {
			err = evidenceFailure("referenced manifest evidence is unavailable")
		}
	}
	if err == nil {
		err = manifest.validateEvidence(statement, applicable, evidence)
	}
	expectsRejection := manifest.Type == "rejected" && manifest.ExpectedPrimaryID != nil && *manifest.ExpectedPrimaryID == IDPackageManifestEvidence
	if expectsRejection && applicable {
		var rejected *evidenceError
		if errors.As(err, &rejected) {
			return nil
		}
		if err == nil {
			return fmt.Errorf("%w: expected manifest-evidence rejection was not observed", ErrInvalidIndex)
		}
	}
	return err
}

func readEvidenceFile(root *os.Root, filename string) (data []byte, err error) {
	if err := validateTestdataPath("evidence input", filename); err != nil {
		return nil, err
	}
	relative := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(filename)), "testdata/")
	file, err := root.Open(filepath.FromSlash(relative))
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	const maximum = 32 << 20
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maximum {
		return nil, errors.New("evidence input must be a bounded regular file")
	}
	data, err = io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maximum {
		return nil, errors.New("evidence input exceeds size bound")
	}
	return data, nil
}
