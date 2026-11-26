package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultCacheContainerPath 是 buildpacks 约定的缓存挂载点。
	DefaultCacheContainerPath = "/layers/cache"
)

// Cache 是宿主机缓存目录到容器的映射。
type Cache struct {
	HostPath      string
	ContainerPath string
}

// Enabled 返回是否启用文件缓存。
func (c Cache) Enabled() bool { return c.HostPath != "" }

func resolveCache(value string) (Cache, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return Cache{}, nil
	}
	hostPath, err := filepath.Abs(raw)
	if err != nil {
		return Cache{}, fmt.Errorf("failed to resolve cache dir: %w", err)
	}
	if err := os.MkdirAll(hostPath, 0o775); err != nil {
		return Cache{}, fmt.Errorf("failed to create cache dir %s: %w", hostPath, err)
	}
	info, err := os.Stat(hostPath)
	if err != nil {
		return Cache{}, fmt.Errorf("failed to stat cache dir %s: %w", hostPath, err)
	}
	if !info.IsDir() {
		return Cache{}, fmt.Errorf("cache path %s is not a directory", hostPath)
	}
	return Cache{HostPath: hostPath, ContainerPath: DefaultCacheContainerPath}, nil
}
