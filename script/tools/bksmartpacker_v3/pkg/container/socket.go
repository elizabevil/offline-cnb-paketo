package container

import "net/url"

// UnixSocketURL 把 socket 路径转换为运行时 CLI 使用的 unix URL。
func UnixSocketURL(socket string) string {
	u := &url.URL{Scheme: "unix", Path: socket}
	return u.String()
}
