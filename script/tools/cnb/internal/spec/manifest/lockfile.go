package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// LockFile 是 assets.lock.toml 的结构：只保存解析结果与校验信息。
type LockFile struct {
	Schema   int           `toml:"schema"`
	Manifest LockManifest  `toml:"manifest"`
	Locked   []LockedAsset `toml:"locked"`
}

// LockManifest 记录生成 lock 时的 manifest 指纹。
type LockManifest struct {
	SHA256 string `toml:"sha256"`
}

// LockedAsset 是单个物料在 lock 中的条目。
type LockedAsset struct {
	ID        string                  `toml:"id"`
	Version   string                  `toml:"version"`
	MultiArch bool                    `toml:"multi_arch,omitempty"`
	Platform  map[string]LockedSource `toml:"platform"`
}

// LockedSource 是某平台的解析结果。
type LockedSource struct {
	// URL 是展开占位符后的实际地址（local 物料为空）。
	URL string `toml:"url,omitempty"`
	// Size 是字节数；-1 表示未知。
	Size int64 `toml:"size,omitempty"`
	// SHA256 是归档哈希；local 物料为产物哈希。
	SHA256 string `toml:"sha256,omitempty"`
}

// ParseLock 解析 assets.lock.toml。
func ParseLock(data []byte) (*LockFile, error) {
	var l LockFile
	if err := toml.Unmarshal(data, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

// MarshalLock 序列化 assets.lock.toml。
func MarshalLock(l *LockFile) ([]byte, error) {
	return toml.Marshal(l)
}

// Hash 返回清单内容的指纹，用于检测 lock 漂移。
func Hash(f *File) (string, error) {
	data, err := toml.Marshal(f)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// FindLocked 按 id/version 查找 lock 条目。
func (l *LockFile) FindLocked(id, version string) (*LockedAsset, bool) {
	for i := range l.Locked {
		if l.Locked[i].ID == id && l.Locked[i].Version == version {
			return &l.Locked[i], true
		}
	}
	return nil, false
}

// LoadLockFile 读取 assets.lock.toml；文件不存在时返回空锁（非错误）。
func LoadLockFile(path string) (*LockFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &LockFile{Schema: 1}, nil
		}
		return nil, err
	}
	return ParseLock(data)
}

// SaveLockFile 原子写入 assets.lock.toml。
func SaveLockFile(path string, l *LockFile) error {
	data, err := MarshalLock(l)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Set 写入/更新一个物料的 lock 条目。
func (l *LockFile) Set(a *Asset, platform string, src LockedSource) {
	if existing, ok := l.FindLocked(a.ID, a.Version); ok {
		existing.MultiArch = a.MultiArch
		if existing.Platform == nil {
			existing.Platform = map[string]LockedSource{}
		}
		existing.Platform[platform] = src
		return
	}
	l.Locked = append(l.Locked, LockedAsset{
		ID: a.ID, Version: a.Version, MultiArch: a.MultiArch,
		Platform: map[string]LockedSource{platform: src},
	})
}
