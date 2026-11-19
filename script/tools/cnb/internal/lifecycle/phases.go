package lifecycle

import (
	"context"
	"fmt"
	"learn-cnb-paketo/tools/cnb/internal/archive"
	"learn-cnb-paketo/tools/cnb/internal/errs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/mod/semver"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/render"
	"learn-cnb-paketo/tools/cnb/internal/spec/imagefmt"
	platformspec "learn-cnb-paketo/tools/cnb/internal/spec/platform"
	"learn-cnb-paketo/tools/cnb/internal/user"
)

// defaultLayoutDir is the fallback OCI layout directory.
const defaultLayoutDir = platformspec.DefaultLayoutDir

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

// Run runs the four phases in order.
//
// BK mode (Export.LayerArchive set) exports an OCI layout and then packs that
// image directory into a layer archive; bksmartpacker copies the archive into
// the s-mart package. The daemon path is only used when the caller asks for it.
func (c *Config) Run() error {
	if c.Export.OutputImage == "" {
		return &errs.Missing{What: "output image"}
	}
	if err := c.prepareWorkDirs(); err != nil {
		return err
	}
	if err := c.prepareRunImage(); err != nil {
		return err
	}
	for _, p := range c.Phases() {
		if err := c.runPhase(p); err != nil {
			return err
		}
	}
	return c.packLayout()
}

// packLayout 在 layout 导出后把目录打包为 OCI archive（如调用方要求）。
func (c Config) packLayout() error {
	if c.Export.Mode != ExportLayout || c.Export.OutputArchive == "" {
		return nil
	}
	repo, tag, digest, err := NormalizeRef(c.Export.OutputImage)
	if err != nil {
		return err
	}
	imageDir := filepath.Join(c.layoutDir(), repo, tag)
	if digest != "" {
		imageDir = filepath.Join(c.layoutDir(), repo, "sha256", strings.TrimPrefix(digest, "sha256:"))
	}
	if _, statErr := os.Stat(filepath.Join(imageDir, "index.json")); statErr != nil {
		return &errs.NotFound{Kind: "layout image", Ref: imageDir}
	}
	c.log().Info("pack layout", "dir", imageDir, "archive", c.Export.OutputArchive)
	// 重新写出一次：imagefmt 会补 docker 兼容文件（manifest.json / repositories），
	// 与参考包保持一致；同时避免把 layout 根下的其它镜像带进包。
	img, _, err := imagefmt.Open(imageDir)
	if err != nil {
		return err
	}
	// 补齐 PATH：lifecycle 从 run 镜像继承 PATH，fake runner 没有标准目录
	if err := ensureStandardPath(img); err != nil {
		return err
	}
	// lifecycle layout 不带标签，用导出镜像名补上（影响 manifest.json / repositories）
	if c.Export.OutputImage != "" {
		img.RepoTags = []string{c.Export.OutputImage}
	}
	tmp, err := os.MkdirTemp("", "cnb-pack-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := img.Write(imagefmt.FormatOCILayout, tmp); err != nil {
		return err
	}
	return archive.PackTar(tmp, c.Export.OutputArchive)
}

// chownLayout 把 layout 目录交给 detect/build 使用的非 root 用户。
func (c *Config) chownLayout() error {
	if c.Runtime.User == "" || os.Geteuid() != 0 {
		return nil
	}
	uid, gid, ok := user.Lookup(c.Runtime.User)
	if !ok {
		return nil
	}
	return fsutil.ChownTree(c.layoutDir(), int(uid), int(gid))
}

// prepareRunImage 在 layout 模式下把 run 镜像预置到 layout 目录，
// 并把 Export.RunImage 换成 layout 内引用。daemon 模式仍载入运行时。
func (c *Config) prepareRunImage() error {
	if c.Export.RunImageArchive == "" || c.DryRun {
		return nil
	}
	if c.Export.Mode == ExportLayout {
		ref, err := PrepareRunImage(c.layoutDir(), c.Export.RunImage, c.Export.RunImageArchive, c.log())
		if err != nil {
			return err
		}
		c.Export.RunImage = ref
		// 预置目录由 root 创建，需交给运行时用户，否则 exporter 无法写入
		return c.chownLayout()
	}
	// daemon 模式：run 镜像由调用方预先放入运行时，无需再次载入。
	// 注意：该路径仅在真 dockerd 下可用；podman 兼容层会触发 lifecycle 的
	// singleton 连接错误（issue #1763），修复前保持 layout 为默认。
	return nil
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
