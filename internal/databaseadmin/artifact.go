package databaseadmin

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const maxAdminerArtifactBytes int64 = 32 << 20

var ErrInvalidArtifact = errors.New("invalid database admin artifact")

// VerifyArtifact validates a locally supplied, pinned Adminer artifact before it is served.
// expectedSHA256 must come from Flock's reviewed release manifest; it is never inferred from
// the downloaded file. Symlinks, non-regular files, oversized files and digest mismatches fail.
func VerifyArtifact(path, expectedSHA256 string) error {
	if len(expectedSHA256) != sha256.Size*2 {
		return fmt.Errorf("%w: expected SHA-256 must be 64 hexadecimal characters", ErrInvalidArtifact)
	}
	expected, err := hex.DecodeString(expectedSHA256)
	if err != nil {
		return fmt.Errorf("%w: malformed expected SHA-256", ErrInvalidArtifact)
	}

	before, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("%w: inspect file: %v", ErrInvalidArtifact, err)
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() > maxAdminerArtifactBytes {
		return fmt.Errorf("%w: file must be a regular file no larger than %d bytes", ErrInvalidArtifact, maxAdminerArtifactBytes)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: open file: %v", ErrInvalidArtifact, err)
	}
	defer file.Close()

	after, err := file.Stat()
	if err != nil {
		return fmt.Errorf("%w: inspect opened file: %v", ErrInvalidArtifact, err)
	}
	if !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return fmt.Errorf("%w: artifact changed while opening", ErrInvalidArtifact)
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, io.LimitReader(file, maxAdminerArtifactBytes+1)); err != nil {
		return fmt.Errorf("%w: hash file: %v", ErrInvalidArtifact, err)
	}
	if after.Size() > maxAdminerArtifactBytes {
		return fmt.Errorf("%w: artifact exceeds size limit", ErrInvalidArtifact)
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), hex.EncodeToString(expected)) {
		return fmt.Errorf("%w: SHA-256 mismatch", ErrInvalidArtifact)
	}
	return nil
}
