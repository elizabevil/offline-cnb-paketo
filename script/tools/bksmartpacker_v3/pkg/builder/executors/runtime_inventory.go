package executors

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"bksmartpacker/pkg/container"
)

// runtimeCLI 封装运行时二进制和 socket 参数，避免调用点直接拼命令。
// 运行时镜像只需要提供与 podman/docker 兼容的 images 子命令；
// system df 是可选能力，缺失时只影响 size 观测，不影响构建。
const (
	flagDockerHost = "-H"
	flagPodmanURL  = "--url"
)

type runtimeCLI struct {
	binary  string
	socket  string
	backend container.Backend
}

func newRuntimeCLI(cfg container.RuntimeConfig) runtimeCLI {
	return runtimeCLI{binary: cfg.Binary, socket: cfg.Socket, backend: cfg.Backend}
}

func (c runtimeCLI) connectionArgs() []string {
	if c.backend == container.BackendDocker {
		return []string{flagDockerHost, container.UnixSocketURL(c.socket)}
	}
	return []string{flagPodmanURL, container.UnixSocketURL(c.socket)}
}

func (c runtimeCLI) args(command ...string) []string {
	return append(c.connectionArgs(), command...)
}

func (c runtimeCLI) output(ctx context.Context, command ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.binary, c.args(command...)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (c runtimeCLI) images(ctx context.Context) ([]string, error) {
	out, err := c.output(ctx, "images", "--format", "{{.ID}}")
	if err != nil {
		return nil, err
	}
	var ids []string
	for line := range strings.SplitSeq(out, "\n") {
		if id := strings.TrimSpace(line); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// imageSize 返回镜像层占用；system df 不是所有兼容运行时都支持。
func (c runtimeCLI) imageSize(ctx context.Context) (size int64, known bool) {
	if c.backend != container.BackendPodman {
		return 0, false
	}
	out, err := c.output(ctx, "system", "df", "--format", "json")
	if err != nil {
		return 0, false
	}
	return parsePodmanImageSize(out)
}
