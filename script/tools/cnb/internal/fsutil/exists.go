package fsutil

import "os"

// Exists 判断路径是否存在（不区分文件与目录）。
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DirExists 判断路径是否存在且为目录。
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
