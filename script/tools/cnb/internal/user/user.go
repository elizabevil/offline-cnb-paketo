// Package user 解析 /etc/passwd 中的 uid/gid，供降权执行使用。
package user

import (
	"os"
	"strconv"
	"strings"
)

// PasswdFile 是用户信息文件（测试可替换）。
var PasswdFile = "/etc/passwd"

// Lookup 返回用户的 uid/gid。
func Lookup(name string) (uid, gid uint32, ok bool) {
	data, err := os.ReadFile(PasswdFile)
	if err != nil {
		return 0, 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Split(line, ":")
		if len(f) < 4 || f[0] != name {
			continue
		}
		u, err1 := strconv.Atoi(f[2])
		g, err2 := strconv.Atoi(f[3])
		if err1 == nil && err2 == nil {
			return uint32(u), uint32(g), true
		}
	}
	return 0, 0, false
}
