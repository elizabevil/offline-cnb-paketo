package lifecycle

import (
	"context"
	"fmt"
	"learn-cnb-paketo/tools/cnb/internal/errs"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/mod/semver"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/render"
	"learn-cnb-paketo/tools/cnb/internal/user"
)

// defaultLayoutDir 是未指定 layout 目录时的落点（调用方可覆盖）。
const defaultLayoutDir = "/layers/oci"

// Phase 描述一个 lifecycle 阶段。
type Phase struct {
	Name string
	Bin  string
	Args []string
	User string // 非空则以该用户执行（detect/build 不允许 root）
}

// Phases 按固定顺序生成四阶段命令。
func (c Config) Phases() []Phase {
	phases := []Phase{
		{Name: PhaseDetect, Bin: c.Bins.Detector, Args: c.detectArgs(), User: c.Runtime.User},
		{Name: PhaseBuild, Bin: c.Bins.Builder, Args: c.buildArgs(), User: c.Runtime.User},
	}
	if c.Export.Mode == ExportNone {
		return phases
	}
	if c.Export.RunImage != "" {
		phases = append(phases, Phase{
			Name: PhaseAnalyze, Bin: c.Bins.Analyzer, Args: c.analyzeArgs(),
		})
	}
	phases = append(phases, Phase{
		Name: PhaseExport, Bin: c.Bins.Exporter, Args: c.exportArgs(),
	})
	return phases
}

// Run 依次执行四阶段。
//
// BK 模式（Export.LayerArchive 非空）下导出走 daemon：exporter 把完整镜像写进
// 容器运行时，随后由 bksmartpacker 执行 image save，产物格式与平台一致。
func (c Config) Run() error {
	if c.Export.OutputImage == "" {
		return &errs.Missing{What: "output image"}
	}
	if err := c.prepareWorkDirs(); err != nil {
		return err
	}
	if err := c.loadRunImage(); err != nil {
		return err
	}
	for _, p := range c.Phases() {
		if err := c.runPhase(p); err != nil {
			return err
		}
	}
	return nil
}

// loadRunImage 把 run 镜像载入容器运行时（BK 以 docker-archive 形式提供）。
func (c Config) loadRunImage() error {
	if c.Export.RunImageArchive == "" || c.DryRun {
		return nil
	}
	tool := containerTool()
	if tool == "" {
		return &errs.Missing{What: "podman / docker"}
	}
	c.log().Info("load run image", "archive", c.Export.RunImageArchive)
	cmd := exec.CommandContext(context.Background(), tool, "load", "-i", c.Export.RunImageArchive)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("load run image: %w", err)
	}
	return nil
}

// containerTool 返回可用的容器 CLI（可用 CONTAINER_TOOL 强制指定）。
func containerTool() string {
	if t := os.Getenv("CONTAINER_TOOL"); t != "" {
		return t
	}
	for _, t := range []string{"podman", "docker"} {
		if _, err := exec.LookPath(t); err == nil {
			return t
		}
	}
	return ""
}

// prepareWorkDirs 在降权前把工作目录交给目标用户。
func (c Config) prepareWorkDirs() error {
	if c.Runtime.User == "" || os.Geteuid() != 0 {
		return nil
	}
	uid, gid, ok := user.Lookup(c.Runtime.User)
	if !ok {
		return nil
	}
	for _, dir := range []string{c.Paths.Layers, c.Paths.App, c.Paths.Platform} {
		if err := fsutil.ChownTree(dir, int(uid), int(gid)); err != nil {
			return fmt.Errorf("chown %s: %w", dir, err)
		}
	}
	return nil
}

// runPhase 执行单个阶段。
func (c Config) runPhase(p Phase) error {
	if c.DryRun {
		c.log().Info("phase", "name", p.Name, "bin", p.Bin, "args", strings.Join(p.Args, " "))
		return nil
	}
	c.log().Info("phase", "name", p.Name)
	cmd := exec.CommandContext(context.Background(), p.Bin, p.Args...)
	env := []string{
		envPlatformAPI + "=" + c.effectivePlatformAPI(),
		envAppDir + "=" + c.Paths.App,
		envLayersDir + "=" + c.Paths.Layers,
		envPlatformDir + "=" + c.Paths.Platform,
	}
	// OCI layout 在 lifecycle 里仍标记为实验特性
	if c.Export.Mode == ExportLayout || c.Export.Mode == "" {
		env = append(env, envExperimental+"=warn")
	}
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if p.User != "" && os.Geteuid() == 0 {
		if uid, gid, ok := user.Lookup(p.User); ok {
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uid, Gid: gid}}
		}
	}
	if err := cmd.Run(); err != nil {
		return errs.Wrap(p.Name+" failed", err)
	}
	return nil
}

// minLayoutPlatformAPI 是 layout 导出所需的最低 Platform API。
// lifecycle 在 0.11 及以下不暴露 -layout 参数，直接使用会报 flag not defined。
const minLayoutPlatformAPI = "0.12"

// effectivePlatformAPI 在 layout 模式下抬高 Platform API 下限。
func (c Config) effectivePlatformAPI() string {
	if c.Export.Mode != ExportLayout && c.Export.Mode != "" {
		return c.Runtime.PlatformAPI
	}
	if semver.Compare("v"+c.Runtime.PlatformAPI, "v"+minLayoutPlatformAPI) < 0 {
		return minLayoutPlatformAPI
	}
	return c.Runtime.PlatformAPI
}

// log 返回日志器（未注入时用默认 stderr 日志）。
func (c Config) log() *slog.Logger {
	if c.Log != nil {
		return c.Log
	}
	return slog.New(render.NewCompactHandler(os.Stderr, slog.LevelInfo))
}
