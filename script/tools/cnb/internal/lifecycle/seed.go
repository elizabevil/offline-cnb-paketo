package lifecycle

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"learn-cnb-paketo/tools/cnb/internal/oci"
)

// runtimeCandidates 是容器运行时命令的探测顺序。
var runtimeCandidates = []string{"docker", "podman"}

// 常量：导出与落盘。
const (
	DefaultDockerSocket = "/var/run/docker.sock"
	DockerAPIVersion    = "v1.41"
	saveTimeout         = 5 * time.Minute
	runImageTempName    = "run-image.tar"
)

// seedRunImage 把 run image 导入 layout 目录。
//
// layout 模式的 exporter 只在该目录里查找 run image；本地构建的 fake runner
// 没有 registry 引用，需要先导出 docker-archive 再转 layout。
// 容器内可能没有 docker/podman CLI，因此直接走 daemon 的 HTTP API。
func (c Config) seedRunImage() error {
	if !c.ExportLayout || c.RunImage == "" {
		return nil
	}
	target, err := oci.LayoutImagePath(c.layoutDir(), c.RunImage)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}

	tmp := filepath.Join(os.TempDir(), runImageTempName)
	defer func() { _ = os.Remove(tmp) }()

	archive := c.RunImageArchive
	if archive == "" {
		archive = tmp
		if err := c.saveImage(archive); err != nil {
			return err
		}
	}
	if err := oci.ToLayout(archive, target); err != nil {
		return fmt.Errorf("转换 run image 为 layout: %w", err)
	}
	return nil
}

// saveImage 用 docker CLI 导出镜像为 docker-archive。
func (c Config) saveImage(dest string) error {
	bin := c.runtimeBin()
	if bin == "" {
		return fmt.Errorf("找不到容器运行时（docker/podman）")
	}
	cmd := exec.Command(bin, "image", "save", "-o", dest, c.RunImage)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("导出 run image: %w", err)
	}
	return nil
}

// runtimeBin 返回可用的容器运行时命令。
func (c Config) runtimeBin() string {
	if c.Runtime != "" {
		return c.Runtime
	}
	for _, name := range runtimeCandidates {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}
