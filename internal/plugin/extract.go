package plugin

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractZip extracts zipPath into dstDir.
func ExtractZip(zipPath, dstDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}

	for _, f := range r.File {
		// Prevent ZipSlip
		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return fmt.Errorf("invalid zip entry: %s", f.Name)
		}

		outPath := filepath.Join(dstDir, cleanName)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(outPath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}

		in, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			in.Close()
			return err
		}

		_, copyErr := io.Copy(out, in)
		in.Close()
		out.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

// ExtractTarGz extracts .tgz/.tar.gz into dstDir.
func ExtractTarGz(tgzPath, dstDir string) error {
	f, err := os.Open(tgzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}

	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		cleanName := filepath.Clean(h.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return fmt.Errorf("invalid tar entry: %s", h.Name)
		}

		outPath := filepath.Join(dstDir, cleanName)

		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(outPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			out.Close()
			if copyErr != nil {
				return copyErr
			}
		default:
			// ignore other types in MVP
		}
	}
	return nil
}
