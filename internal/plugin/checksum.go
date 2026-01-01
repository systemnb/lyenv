package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// VerifySHA256 computes SHA-256 of a file and compares with expected hex string (lowercase).
func VerifySHA256(filePath, expected string) error {
	if expected == "" {
		return nil // nothing to verify
	}
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file failed: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 64*1024)
	for {
		n, er := f.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if er != nil {
			break
		}
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if sum != expected {
		return fmt.Errorf("sha256 mismatch: got=%s expected=%s", sum, expected)
	}
	return nil
}
