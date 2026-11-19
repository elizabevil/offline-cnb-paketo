// Package fsutil 提供文件复制、哈希、大小与属主等通用操作。
package fsutil

// 文件与目录的默认权限。
const (
	// FileMode 是普通文件的默认权限（无类型常量，便于各处隐式转换）。
	FileMode = 0o644
	// DirMode 是目录的默认权限。
	DirMode = 0o755
	// ExecMode 是可执行文件的权限。
	ExecMode = 0o755
)
