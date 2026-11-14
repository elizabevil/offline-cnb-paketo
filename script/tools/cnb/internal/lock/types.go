// Package lock 定义物料清单（assets.lock.toml）的结构与读取逻辑。
package lock

// 语义别名：让函数签名自解释，且与 string 互通、无需类型转换。
type (
	// ID 是物料标识（如 lifecycle）。
	ID = string
	// Version 是物料版本（如 0.21.29）。
	Version = string
	// Platform 是平台名（如 linux-amd64）。
	Platform = string
	// Ref 是 id@version 形式的引用。
	Ref = string
)

// File 是 assets.lock.toml 的完整结构。
type File struct {
	Schema int
	Meta   Meta    `toml:"meta"`
	Assets []Asset `toml:"asset"`
}

// Meta 记录清单的来源与平台范围。
type Meta struct {
	Name        string   `toml:"name"`
	Description string   `toml:"description"`
	Provenance  string   `toml:"provenance"`
	CapturedAt  string   `toml:"captured_at"`
	Platforms   []string `toml:"platforms"`
}

// Local 描述由本地构建产生的物料，不走下载。
type Local struct {
	Source string `toml:"source"` // 相对仓库根的产物路径
	SHA256 string `toml:"sha256"` // 发布后回填
}

// Asset 是一条物料定义。
type Asset struct {
	ID      string `toml:"id"`
	Version string `toml:"version"`
	Default bool   `toml:"default"`

	Info     Info              `toml:"info"`
	Unpack   Unpack            `toml:"unpack"`
	Platform map[string]Source `toml:"platform"`
	Local    *Local            `toml:"local"`
}

// Info 是元信息，不参与构建。
type Info struct {
	Name     string `toml:"name"`
	Homepage string `toml:"homepage"`
	License  string `toml:"license"`
}

// Kind 是解包方式，属于清单协议的一部分。
type Kind string

// 支持的解包方式。
const (
	// KindTGZ 是 gzip 压缩的 tar。
	KindTGZ Kind = "tgz"
	// KindTarGz 是 KindTGZ 的别名写法。
	KindTarGz Kind = "tar.gz"
	// KindZip 是 zip 压缩包。
	KindZip Kind = "zip"
	// KindBin 是单文件二进制。
	KindBin Kind = "bin"
)

// Unpack 描述如何解包；Dir 支持 {version}。
type Unpack struct {
	// Include 是解压白名单（通配符）；为空表示全部。
	Include []string `toml:"include,omitempty"`
	Kind    Kind     `toml:"kind"`
	Strip   int      `toml:"strip"`
	Dir     string   `toml:"dir"`
	Expect  []string `toml:"expect"`
}

// Source 是某平台的下载源。
type Source struct {
	URL    string `toml:"url"`
	SHA256 string `toml:"sha256"`
	Size   int64  `toml:"size,omitempty"`
}

// Ref 返回 "id@version"。
