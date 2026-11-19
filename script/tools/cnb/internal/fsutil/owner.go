package fsutil

import (
	"os"
	"path/filepath"
)

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
