package fsutil

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// SHA256 返回文件的十六进制 SHA256。
func SHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
