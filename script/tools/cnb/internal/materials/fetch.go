package materials

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// FetchConfig 控制物料下载行为；零值使用 DefaultFetchConfig。
type FetchConfig struct {
	// Timeout 是单次请求的超时（含传输）。
	Timeout time.Duration
	// Retries 是可重试错误的最大尝试次数。
	Retries int
	// Backoff 是指数退避的基准时长。
	Backoff time.Duration
	// Proxy 为显式代理地址；为空时沿用环境变量（HTTPS_PROXY 等）。
	Proxy string
	// Headers 是附加请求头（如私有仓库令牌）。
	Headers map[string]string
	// UserAgent 是请求标识。
	UserAgent string
}

// DefaultFetchConfig 返回推荐默认值。
func DefaultFetchConfig() FetchConfig {
	return FetchConfig{
		Timeout:   30 * time.Minute,
		Retries:   3,
		Backoff:   2 * time.Second,
		UserAgent: "cnb-materials/1",
	}
}

// normalize 填充零值。
func (c FetchConfig) normalize() FetchConfig {
	d := DefaultFetchConfig()
	if c.Timeout <= 0 {
		c.Timeout = d.Timeout
	}
	if c.Retries <= 0 {
		c.Retries = d.Retries
	}
	if c.Backoff <= 0 {
		c.Backoff = d.Backoff
	}
	if c.UserAgent == "" {
		c.UserAgent = d.UserAgent
	}
	return c
}

// client 构造 http.Client；显式 Proxy 优先于环境变量。
func (c FetchConfig) client() (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if c.Proxy != "" {
		u, err := url.Parse(c.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy %q: %w", c.Proxy, err)
		}
		transport.Proxy = http.ProxyURL(u)
	}
	return &http.Client{Timeout: c.Timeout, Transport: transport}, nil
}

// download 下载 url 到 dest，失败时保留 .part 供诊断。
// 仅对网络错误与 5xx 重试；4xx 直接失败。
func (c FetchConfig) download(ctx context.Context, src, dest string) error {
	c = c.normalize()
	client, err := c.client()
	if err != nil {
		return err
	}
	tmp := dest + ".part"
	var lastErr error
	for attempt := 0; attempt < c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.Backoff << (attempt - 1)):
			}
		}
		retryable, err := c.fetchOnce(ctx, client, src, tmp)
		if err == nil {
			return os.Rename(tmp, dest)
		}
		lastErr = err
		if !retryable {
			break
		}
	}
	_ = os.Remove(tmp)
	return lastErr
}

// fetchOnce 单次下载；返回是否可重试。
func (c FetchConfig) fetchOnce(ctx context.Context, client *http.Client, src, dest string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return false, err
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return true, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode >= 500:
		return true, fmt.Errorf("http %s", resp.Status)
	default:
		return false, fmt.Errorf("http %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return false, err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		return true, err
	}
	return false, out.Close()
}
