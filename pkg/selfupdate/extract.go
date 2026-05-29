package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// extractBinary extracts the agent-fleet binary from a .tar.gz archive.
// Returns the path to the extracted binary.
func extractBinary(archivePath string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	tmpDir, err := os.MkdirTemp("", "agent-fleet-extract-*")
	if err != nil {
		return "", err
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading tar: %w", err)
		}

		// Look for the agent-fleet binary
		if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == "agent-fleet" {
			outPath := filepath.Join(tmpDir, "agent-fleet")
			outFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY, 0755)
			if err != nil {
				return "", err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return "", err
			}
			outFile.Close()
			return outPath, nil
		}
	}

	return "", fmt.Errorf("agent-fleet binary not found in archive")
}
