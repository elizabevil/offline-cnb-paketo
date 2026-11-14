package repack

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
)

// marshalCompact 输出紧凑 JSON（无空格、不转义 HTML），与 Python json.dumps 一致。
func marshalCompact(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func writeJSONFile(path string, v any) error {
	data, err := marshalCompact(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, fsutil.FileMode)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// parseRepoTag 拆分 repo:tag，忽略 docker.io/ 前缀。
func parseRepoTag(repoTag string) (repo, tag string) {
	normalized := strings.TrimPrefix(repoTag, "docker.io/")
	slash := strings.LastIndex(normalized, "/")
	lastSegment := normalized[slash+1:]
	if idx := strings.LastIndex(lastSegment, ":"); idx >= 0 {
		repo = normalized[:slash+1+idx]
		tag = lastSegment[idx+1:]
		return repo, tag
	}
	return normalized, "latest"
}
