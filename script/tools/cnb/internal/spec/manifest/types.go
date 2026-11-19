// Package manifest 定义物料清单（assets.lock.toml）的结构、解析与校验。
package manifest

// 语义别名：让函数签名自解释，且与 string 互通、无需类型转换。
type (
	// ID 是物料标识（如 lifecycle）。
	ID = string
	// Version 是物料版本（如 0.21.29）。
	Version = string
	// Ref 是 id@version 形式的引用。
	Ref = string
)

// File 是 assets.lock.toml 的完整结构。
type File struct {
	Schema  int
	Meta    Meta    `toml:"meta"`
	Storage Storage `toml:"storage"`
	Assets  []Asset `toml:"asset"`
}

// Storage 声明物料落盘位置，避免调用方自己猜路径。
type Storage struct {
	// Root 是物料根目录；相对路径以 lock 文件所在目录为基准。
	Root string `toml:"root"`
	// Layout 是 bundle 内布局：arch-first（默认）或 kind-first。
	Layout string `toml:"layout"`
}

// Meta 记录清单的来源与平台范围。
type Meta struct {
	Name        string   `toml:"name"`
	Description string   `toml:"description"`
	Provenance  string   `toml:"provenance"`
	CapturedAt  string   `toml:"captured_at"`
	Platforms   []string `toml:"platforms"`
}

// Asset 是一条物料定义。
type Asset struct {
	ID      string `toml:"id"`
	Version string `toml:"version"`
	Default bool   `toml:"default"`
	// MultiArch 表示所有平台共用同一个归档（如官方多架构 release 包）。
	MultiArch bool `toml:"multi_arch,omitempty"`

	Info     Info              `toml:"info"`
	Unpack   Unpack            `toml:"unpack"`
	Platform map[string]Source `toml:"platform"`
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
// Source 描述某平台物料的来源。
type Source struct {
	// Kind 为 "http"（默认）或 "local"。
	Kind string `toml:"kind,omitempty"`
	// URL 是 http 来源地址，允许唯一占位符 {version}。
	URL string `toml:"url,omitempty"`
	// Mirrors 是可选的回退源，仅在传输/HTTP 错误时尝试。
	Mirrors []string `toml:"mirrors,omitempty"`
	// Source 是 local 来源的仓库相对路径。
	Source string `toml:"source,omitempty"`
	// Recipe 是 local 来源的构建命令（可选，便于复现）。
	Recipe string `toml:"recipe,omitempty"`
	// Size 是字节数；-1 表示未知。
	Size int64 `toml:"size,omitempty"`
	// SHA256 是归档哈希。
	SHA256 string `toml:"sha256,omitempty"`
}

// Ref 返回 "id@version"。
