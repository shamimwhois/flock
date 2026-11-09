package databaseadmin

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyArtifactAcceptsPinnedRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "adminer.php")
	content := []byte("<?php echo 'adminer';")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	if err := VerifyArtifact(path, hex.EncodeToString(sum[:])); err != nil {
		t.Fatalf("VerifyArtifact() error = %v", err)
	}
}

func TestVerifyArtifactRejectsDigestMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "adminer.php")
	if err := os.WriteFile(path, []byte("not the pinned artifact"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := VerifyArtifact(path, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("VerifyArtifact() error = %v, want ErrInvalidArtifact", err)
	}
}

func TestVerifyArtifactRejectsInvalidDigestAndMissingFile(t *testing.T) {
	if err := VerifyArtifact("unused", "not-a-digest"); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("invalid digest error = %v", err)
	}
	if err := VerifyArtifact(filepath.Join(t.TempDir(), "missing.php"), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("missing file error = %v", err)
	}
}

func TestVerifyArtifactRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.php")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxAdminerArtifactBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := VerifyArtifact(path, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("oversized file error = %v", err)
	}
}

func TestVerifyArtifactRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.php")
	link := filepath.Join(dir, "adminer.php")
	if err := os.WriteFile(target, []byte("artifact"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	sum := sha256.Sum256([]byte("artifact"))
	if err := VerifyArtifact(link, hex.EncodeToString(sum[:])); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("symlink error = %v, want ErrInvalidArtifact", err)
	}
}

func TestVerifyArtifactDigestValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "adminer.php")
	content := []byte("pinned artifact")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	for _, tc := range []struct {
		name, digest string
		valid        bool
	}{
		{"uppercase", strings.ToUpper(digest), true},
		{"empty", "", false}, {"short", digest[:63], false},
		{"long", digest + "0", false}, {"nonhex", strings.Repeat("z", 64), false},
		{"leading whitespace", " " + digest, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyArtifact(path, tc.digest)
			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, ErrInvalidArtifact) {
				t.Fatalf("error = %v, want ErrInvalidArtifact", err)
			}
		})
	}
	if err := VerifyArtifact(t.TempDir(), digest); !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("directory error = %v", err)
	}
}

func TestVerifyArtifactAcceptsSizeBoundaries(t *testing.T) {
	for _, size := range []int64{0, maxAdminerArtifactBytes} {
		name := "empty"
		if size != 0 {
			name = "maximum"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "adminer.php")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := file.Truncate(size); err != nil {
				file.Close()
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			file, err = os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.New()
			_, err = io.Copy(hash, file)
			file.Close()
			if err != nil {
				t.Fatal(err)
			}
			if err := VerifyArtifact(path, hex.EncodeToString(hash.Sum(nil))); err != nil {
				t.Fatalf("size %d: %v", size, err)
			}
		})
	}
}
