package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"learn-cnb-paketo/tools/cnb/internal/oci"
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
		{Name: PhaseDetect, Bin: c.Bins.Detector, Args: c.detectArgs(), User: c.User},
		{Name: PhaseBuild, Bin: c.Bins.Builder, Args: c.buildArgs(), User: c.User},
	}
	if c.RunImage != "" {
		phases = append(phases, Phase{
			Name: PhaseAnalyze, Bin: c.Bins.Analyzer, Args: c.analyzeArgs(),
		})
	}
	phases = append(phases, Phase{
		Name: PhaseExport, Bin: c.Bins.Exporter, Args: c.exportArgs(),
	})
	return phases
}

// Run 依次执行四阶段，并在 layout 模式下转换为 docker-archive。
func (c Config) Run(logf func(format string, args ...any)) error {
	if c.OutputImage == "" {
		return fmt.Errorf("缺少输出镜像名")
	}
	if err := c.prepareWorkDirs(); err != nil {
		return err
	}
	for _, p := range c.Phases() {
		if err := c.runPhase(p, logf); err != nil {
			return err
		}
		// exporter 在 layout 模式下要求 run image 引用形如 name@digest，
		// 而 daemon 模式的 analyzer 只能写出镜像 ID，需要就地补全。
		if p.Name == PhaseAnalyze && c.ExportLayout {
			if err := c.normalizeAnalyzed(); err != nil {
				return err
			}
		}
	}
	return c.convertLayout()
}

// prepareWorkDirs 在降权前把工作目录交给目标用户。
func (c Config) prepareWorkDirs() error {
	if c.User == "" || os.Geteuid() != 0 {
		return nil
	}
	uid, gid, ok := lookupUser(c.User)
	if !ok {
		return nil
	}
	for _, dir := range []string{c.Layers, c.App, c.Platform} {
		if err := chownTree(dir, int(uid), int(gid)); err != nil {
			return fmt.Errorf("chown %s: %w", dir, err)
		}
	}
	return nil
}

// runPhase 执行单个阶段。
func (c Config) runPhase(p Phase, logf func(format string, args ...any)) error {
	if c.DryRun {
		logf("• %s: %s %s", p.Name, p.Bin, strings.Join(p.Args, " "))
		return nil
	}
	logf("• %s", p.Name)
	cmd := exec.CommandContext(context.Background(), p.Bin, p.Args...)
	env := []string{
		"CNB_PLATFORM_API=" + c.effectivePlatformAPI(),
		"CNB_APP_DIR=" + c.App,
		"CNB_LAYERS_DIR=" + c.Layers,
		"CNB_PLATFORM_DIR=" + c.Platform,
	}
	// OCI layout 在 lifecycle 里仍标记为实验特性
	if c.ExportLayout {
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
	if !c.ExportLayout {
		return c.PlatformAPI
	}
	if apiLess(c.PlatformAPI, minLayoutPlatformAPI) {
		return minLayoutPlatformAPI
	}
	return c.PlatformAPI
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

// convertLayout 把 OCI layout 转成 docker-archive（如指定了输出路径）。
func (c Config) convertLayout() error {
	if !c.ExportLayout || c.LayerArchive == "" {
		return nil
	}
	dir := c.LayoutDir
	if dir == "" {
		dir = defaultLayoutDir
	}
	if err := oci.ToDockerArchive(dir, c.OutputImage, c.LayerArchive); err != nil {
		return fmt.Errorf("转换 docker-archive: %w", err)
	}
	return nil
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
