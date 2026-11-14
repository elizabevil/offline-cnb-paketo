package lifecycle

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Phase 描述一个 lifecycle 阶段。
type Phase struct {
	Name string
	Bin  string
	Args []string
	User string // 非空则以该用户执行（detect/build 不允许 root）
}

// Phases 生成四阶段命令（顺序固定）。
func (c Config) Phases() []Phase {
	orderPath := c.Order
	if c.Required != "" {
		orderPath = c.Layers + "/order.toml"
	}
	common := []string{
		"-layers=" + c.Layers,
		"-app=" + c.App,
		"-platform=" + c.Platform,
	}
	phases := []Phase{
		{Name: "detect", Bin: "/cnb/lifecycle/detector", Args: append(append([]string{}, common...), "-order="+orderPath), User: c.User},
		{Name: "build", Bin: "/cnb/lifecycle/builder", Args: common, User: c.User},
	}
	if c.RunImage != "" {
		phases = append(phases, Phase{
			Name: "analyze", Bin: "/cnb/lifecycle/analyzer",
			Args: append([]string{"-layers=" + c.Layers, "-run-image=" + c.RunImage, "-daemon"}, c.OutputImage),
		})
	}
	exportArgs := []string{"-layers=" + c.Layers, "-app=" + c.App, "-daemon"}
	if c.RunImage != "" {
		exportArgs = append(exportArgs, "-run-image="+c.RunImage)
	}
	if c.CacheImage != "" {
		exportArgs = append(exportArgs, "-cache-image="+c.CacheImage)
	}
	phases = append(phases, Phase{
		Name: "export", Bin: "/cnb/lifecycle/exporter",
		Args: append(exportArgs, c.OutputImage),
	})
	return phases
}

// Run 依次执行四阶段。
func (c Config) Run(logf func(format string, args ...any)) error {
	if c.OutputImage == "" {
		return fmt.Errorf("缺少 OUTPUT_IMAGE")
	}
	// 以非 root 执行 detect/build 前，把工作目录交给该用户
	if c.User != "" && os.Geteuid() == 0 {
		if uid, gid, ok := lookupUser(c.User); ok {
			for _, dir := range []string{c.Layers, c.App, c.Platform} {
				if err := chownTree(dir, int(uid), int(gid)); err != nil {
					return fmt.Errorf("chown %s: %w", dir, err)
				}
			}
		}
	}

	for _, p := range c.Phases() {
		if c.DryRun {
			logf("• %s: %s %s", p.Name, p.Bin, strings.Join(p.Args, " "))
			continue
		}
		logf("• %s", p.Name)
		cmd := exec.Command(p.Bin, p.Args...)
		cmd.Env = append(os.Environ(), "CNB_PLATFORM_API="+c.PlatformAPI,
			"CNB_APP_DIR="+c.App, "CNB_LAYERS_DIR="+c.Layers, "CNB_PLATFORM_DIR="+c.Platform)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if p.User != "" && os.Geteuid() == 0 {
			if uid, gid, ok := lookupUser(p.User); ok {
				cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uid, Gid: gid}}
			}
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s 失败: %w", p.Name, err)
		}
	}
	return nil
}

// lookupUser 解析用户名到 uid/gid。
func lookupUser(name string) (uint32, uint32, bool) {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return 0, 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Split(line, ":")
		if len(f) < 4 || f[0] != name {
			continue
		}
		uid, err1 := strconv.Atoi(f[2])
		gid, err2 := strconv.Atoi(f[3])
		if err1 == nil && err2 == nil {
			return uint32(uid), uint32(gid), true
		}
	}
	return 0, 0, false
}

// chownTree 递归把 root 改为指定 uid/gid（仅 root 可用）。
func chownTree(root string, uid, gid int) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略不可访问项
		}
		if chownErr := os.Chown(path, uid, gid); chownErr != nil {
			return nil // 非本用户拥有的文件可能失败，忽略
		}
		return nil
	})
}
