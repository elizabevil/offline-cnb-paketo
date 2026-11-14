package materials

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// 下载参数。
const (
	httpTimeout  = 30 * time.Minute
	connectRetry = 3
	retryBackoff = 2 * time.Second
	fileModeRead = 0o644
	tempSuffix   = ".part"
)

// ensureArchive 确保归档存在且通过 sha256 + size 校验。
func ensureArchive(a *lock.Asset, dest string, opts Options) error {
	want := a.SHA256(opts.Platform)
	if want == "" {
		return fmt.Errorf("%s: sha256 not pinned", a.Ref())
	}
	if ok, err := verifyArchive(dest, want, a.Size(opts.Platform)); err == nil && ok {
		return nil
	}
	if err := os.MkdirAll(dirOf(dest), 0o755); err != nil {
		return err
	}
	for _, url := range sources(a, opts.Platform) {
		if err := download(url, dest); err != nil {
			continue
		}
		ok, err := verifyArchive(dest, want, a.Size(opts.Platform))
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return fmt.Errorf("%s: all sources failed", a.Ref())
}

// sources 返回候选下载地址（环境变量镜像优先）。
func sources(a *lock.Asset, platform string) []string {
	base, _ := a.URL(platform)
	if base == "" {
		return nil
	}
	out := []string{}
	if m := os.Getenv("MATERIALS_MIRROR"); m != "" {
		out = append(out, m+base)
	}
	out = append(out, base)
	return out
}

// download 下载到 dest，失败时最多重试 connectRetry 次。
func download(url, dest string) error {
	tmp := dest + tempSuffix
	client := &http.Client{Timeout: httpTimeout}
	var lastErr error
	for i := 0; i < connectRetry; i++ {
		err := fetchOnce(client, url, tmp)
		if err == nil {
			return os.Rename(tmp, dest)
		}
		lastErr = err
		time.Sleep(retryBackoff << i)
	}
	return lastErr
}

// fetchOnce 单次下载。
func fetchOnce(client *http.Client, url, dest string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %s", resp.Status)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// verifyArchive 校验哈希与大小。
func verifyArchive(path, want string, wantSize int64) (bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		return false, nil
	}
	if wantSize > 0 && st.Size() != wantSize {
		return false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return hex.EncodeToString(h.Sum(nil)) == want, nil
}

// dirOf 返回文件所在目录。
func dirOf(path string) string { return filepath.Dir(path) }
