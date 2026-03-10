package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileInfo holds metadata about a discovered file.
type FileInfo struct {
	// RelPath is the path relative to the root directory.
	RelPath    string
	AbsPath    string
	SHA256Hash string
	FileType   string // "sbom" or "vex"
}

// Scanner walks a directory to discover SPDX JSON files and compute their hashes.
type Scanner struct {
	rootDir string
}

// NewScanner creates a new Scanner for the given root directory.
func NewScanner(rootDir string) *Scanner {
	return &Scanner{rootDir: rootDir}
}

// Scan walks the root directory and returns all .spdx.json files with their hashes.
func (s *Scanner) Scan() ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking path %s: %w", path, err)
		}

		if info.IsDir() {
			return nil
		}

		// Classify file type: VEX or SBOM.
		name := strings.ToLower(info.Name())
		fileType := ""
		switch {
		case strings.HasSuffix(name, ".openvex.json"), strings.HasSuffix(name, ".vex.json"):
			fileType = "vex"
		case strings.HasSuffix(name, ".spdx.json"), strings.HasSuffix(name, ".json"):
			fileType = "sbom"
		default:
			return nil
		}

		hash, err := hashFile(path)
		if err != nil {
			return fmt.Errorf("failed to hash file %s: %w", path, err)
		}

		relPath, err := filepath.Rel(s.rootDir, path)
		if err != nil {
			return fmt.Errorf("failed to compute relative path: %w", err)
		}

		files = append(files, FileInfo{
			RelPath:    relPath,
			AbsPath:    path,
			SHA256Hash: hash,
			FileType:   fileType,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory %s: %w", s.rootDir, err)
	}

	return files, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
