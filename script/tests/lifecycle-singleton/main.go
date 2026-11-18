// 复现 lifecycle singleton 连接冲突：
// 在一个流式请求（ImageLoad）尚未结束时发起第二个请求（ImageSave），
// 客户端直接返回 "singleton connection already in use"。
//
// 用途：验证 lifecycle 的未来版本是否修复了该问题。
// 必须以 root 运行：priv.shouldConnectSock 只对 uid=0 的 socket 生效。
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/buildpacks/lifecycle/priv"
)

func main() {
	sock := "/tmp/lc-singleton-repro.sock"
	_ = os.Remove(sock)
	l, err := net.Listen("unix", sock)
	if err != nil {
		panic(err)
	}
	defer l.Close()

	release := make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/images/load") {
			_, _ = io.CopyN(io.Discard, r.Body, 1)
			<-release // 保持该请求未完成
			_, _ = w.Write([]byte(`{"stream":"ok"}`))
			return
		}
		w.Header().Set("Content-Type", "application/x-tar")
		_, _ = w.Write([]byte("tar-bytes"))
	})}
	go func() { _ = srv.Serve(l) }()

	if err := os.Setenv("DOCKER_HOST", "unix://"+sock); err != nil {
		panic(err)
	}
	cli, err := priv.DockerClient()
	if err != nil {
		panic(err)
	}

	pr, pw := io.Pipe()
	go func() {
		res, err := cli.ImageLoad(context.Background(), pr)
		if res != nil {
			_ = res.Close()
		}
		_ = err
	}()
	_, _ = pw.Write([]byte("x"))
	time.Sleep(100 * time.Millisecond)

	res, err := cli.ImageSave(context.Background(), []string{"sha256:" + strings.Repeat("0", 64)})
	if res != nil {
		_ = res.Close()
	}
	fmt.Printf("ImageSave error: %v\n", err)
	close(release)
}
