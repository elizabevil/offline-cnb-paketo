package fsutil

import "os"

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
