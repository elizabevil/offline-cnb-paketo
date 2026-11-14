package fsutil

import (
	"os"
	"path/filepath"
)

// CopyFile 复制文件并将权限设为可执行。
func CopyFile(src, dst string) error {
	if _, err := Copy(src, dst); err != nil {
		return err
	}
	return os.Chmod(dst, ExecMode)
}

// Size 返回文件大小；不存在时返回 0。
func Size(path string) int64 {
	if st, err := os.Stat(path); err == nil {
		return st.Size()
	}
	return 0
}

// SizeOf 返回文件大小，不存在时返回错误。
func SizeOf(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

// Dir 是 filepath.Dir 的语义别名，避免各处重复包装。
func Dir(path string) string { return filepath.Dir(path) }

// ChownTree 递归修改属主；遇到不可访问的条目会跳过。
func ChownTree(root string, uid, gid int) error {
	return filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		_ = os.Chown(path, uid, gid) // 非本用户拥有的文件会失败，忽略
		return nil
	})
}
