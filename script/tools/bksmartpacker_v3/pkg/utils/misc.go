package utils

import (
	"os"
	"path/filepath"
)

// DetectLogoFile 探测当前应用源码是否有 logo 文件, 如果有, 则返回 logo 文件名; 否则返回空字符串
func DetectLogoFile(sourceDir string) string {
	for _, filename := range []string{"logo.png", "logo.jpg", "logo.jpeg"} {
		_, err := os.Stat(filepath.Join(sourceDir, filename))
		if err == nil {
			return filename
		}
	}
	// logo not found!
	return ""
}
