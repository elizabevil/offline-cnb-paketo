package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"learn-cnb-paketo/tools/cnb/internal/apparchive"
	"learn-cnb-paketo/tools/cnb/internal/render"
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
	if c.Export.RunImage != "" && !c.appLayerMode() {
		phases = append(phases, Phase{
			Name: PhaseAnalyze, Bin: c.Bins.Analyzer, Args: c.analyzeArgs(),
		})
	}
	// 只输出应用层：不调用 exporter，直接由 /app 生成 docker-archive
	if c.appLayerMode() {
		return phases
	}
	phases = append(phases, Phase{
		Name: PhaseExport, Bin: c.Bins.Exporter, Args: c.exportArgs(),
	})
	return phases
}

// Run 依次执行四阶段，并在 layout 模式下转换为 docker-archive。
func (c Config) Run() error {
	if c.Export.OutputImage == "" {
		return fmt.Errorf("缺少输出镜像名")
	}
	if err := c.prepareWorkDirs(); err != nil {
		return err
	}
	for _, p := range c.Phases() {
		if err := c.runPhase(p); err != nil {
			return err
		}
	}
	if c.appLayerMode() {
		return c.exportAppLayer()
	}
	return nil
}

// prepareWorkDirs 在降权前把工作目录交给目标用户。
func (c Config) prepareWorkDirs() error {
	if c.Runtime.User == "" || os.Geteuid() != 0 {
		return nil
	}
	uid, gid, ok := lookupUser(c.Runtime.User)
	if !ok {
		return nil
	}
	for _, dir := range []string{c.Paths.Layers, c.Paths.App, c.Paths.Platform} {
		if err := chownTree(dir, int(uid), int(gid)); err != nil {
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
		"CNB_PLATFORM_API=" + c.effectivePlatformAPI(),
		"CNB_APP_DIR=" + c.Paths.App,
		"CNB_LAYERS_DIR=" + c.Paths.Layers,
		"CNB_PLATFORM_DIR=" + c.Paths.Platform,
	}
	// OCI layout 在 lifecycle 里仍标记为实验特性
	if c.Export.Layout {
		env = append(env, "CNB_EXPERIMENTAL_MODE=warn")
	}
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if p.User != "" && os.Geteuid() == 0 {
		if uid, gid, ok := lookupUser(p.User); ok {
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uid, Gid: gid}}
		}
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s 失败: %w", p.Name, err)
	}
	return nil
}

// minLayoutPlatformAPI 是 layout 导出所需的最低 Platform API。
// lifecycle 在 0.11 及以下不暴露 -layout 参数，直接使用会报 flag not defined。
const minLayoutPlatformAPI = "0.12"

// effectivePlatformAPI 在 layout 模式下抬高 Platform API 下限。
func (c Config) effectivePlatformAPI() string {
	if !c.Export.Layout {
		return c.Runtime.PlatformAPI
	}
	if apiLess(c.Runtime.PlatformAPI, minLayoutPlatformAPI) {
		return minLayoutPlatformAPI
	}
	return c.Runtime.PlatformAPI
}

// apiLess 比较 x.y 形式的版本号。
func apiLess(a, b string) bool {
	var amaj, amin, bmaj, bmin int
	// 解析失败时按 0 处理，等价于"低于下限"
	_, _ = fmt.Sscanf(a, "%d.%d", &amaj, &amin)
	_, _ = fmt.Sscanf(b, "%d.%d", &bmaj, &bmin)
	if amaj != bmaj {
		return amaj < bmaj
	}
	return amin < bmin
}

// lookupUser 解析用户名到 uid/gid。
func lookupUser(name string) (uint32, uint32, bool) {
	data, err := os.ReadFile(UserDatabase)
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
	return filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略不可访问项
		}
		_ = os.Chown(path, uid, gid) // 非本用户拥有的文件可能失败，忽略
		return nil
	})
}

// appLayerMode 判断是否走"只打包应用层"的导出方式。
// 平台给出层包路径（BKS_LAYER_ARCHIVE）即视为该模式。
func (c Config) appLayerMode() bool { return c.Export.LayerArchive != "" }

// exportAppLayer 把 /app 打成 docker-archive。
func (c Config) exportAppLayer() error {
	c.log().Info("export app layer", "image", c.Export.OutputImage, "archive", c.Export.LayerArchive)
	arch := ""
	if strings.HasPrefix(c.Paths.Platform, "linux-") {
		arch = strings.TrimPrefix(c.Paths.Platform, "linux-")
	}
	if err := apparchive.Build(c.Paths.App, c.Paths.Layers, c.Export.OutputImage, arch, c.Export.LayerArchive, c.Export.IncludeSource); err != nil {
		return fmt.Errorf("生成应用层归档: %w", err)
	}
	return nil
}

// log 返回日志器（未注入时用默认 stderr 日志）。
func (c Config) log() *slog.Logger {
	if c.Log != nil {
		return c.Log
	}
	return slog.New(render.NewCompactHandler(os.Stderr, slog.LevelInfo))
}
