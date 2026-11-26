package executors

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"bksmartpacker/pkg/container"
	"bksmartpacker/pkg/env"
	"bksmartpacker/pkg/log"
	"bksmartpacker/pkg/plan"
	"bksmartpacker/pkg/source"
	"bksmartpacker/pkg/utils"
)

// ContainerExecutor 使用 docker 或 podman 执行构建计划。
type ContainerExecutor struct {
	plan            plan.BuildPlan
	runtime         container.Config
	cnbBuilderImage string
	runImageArchive string
	workspace       *Workspace
	sidecarName     string
	sidecarStarted  bool
}

// NewContainerExecutor 使用已解析的运行时配置创建执行器。
func NewContainerExecutor(buildPlan plan.BuildPlan, runtimeCfg container.Config) (*ContainerExecutor, error) {
	builderImage := DefaultCNBBuilderImage
	if buildPlan.BuilderImage != "" {
		builderImage = buildPlan.BuilderImage
	}
	log.Info("container runtime", "driver", runtimeCfg.Driver, "environment", runtimeCfg.Environment, "outer", runtimeCfg.Outer.Backend, "outer_binary", runtimeCfg.Outer.Binary, "inner", runtimeCfg.Inner.Backend, "inner_binary", runtimeCfg.Inner.Binary, "inner_socket", runtimeCfg.Inner.Socket, "export", runtimeCfg.ExportMode)
	return &ContainerExecutor{
		plan:            buildPlan,
		runtime:         runtimeCfg,
		cnbBuilderImage: builderImage,
		runImageArchive: runtimeCfg.RunImageArchive,
	}, nil
}

// Execute 使用选中的容器运行时执行构建计划。
func (e *ContainerExecutor) Execute(ctx context.Context) error {
	return e.execute(ctx)
}

func (e *ContainerExecutor) execute(ctx context.Context) error {
	workspace, err := NewWorkspace(e.runtime.Workspace)
	if err != nil {
		return err
	}
	e.workspace = workspace
	defer func() { _ = workspace.Close() }()

	if err := e.ensureRuntime(ctx); err != nil {
		return err
	}
	defer e.stopSidecar()
	if e.runtime.BuilderUser != "" {
		if err := os.Chmod(e.runtime.Inner.Socket, 0o666); err != nil {
			return errors.Wrap(err, "failed to make runtime socket accessible to builder user")
		}
	}
	if e.runtime.Driver == container.DriverManaged {
		cacheBefore := e.snapshotRuntimeCache()
		defer e.reportRuntimeCache(cacheBefore)
	}
	if err := e.prepareRunImage(ctx); err != nil {
		return errors.Wrap(err, "failed to prepare run image")
	}

	if e.plan.PackagingVersion == plan.PackagingVersionV2 {
		for _, group := range e.plan.Groups {
			if err := e.buildGroup(ctx, group); err != nil {
				return errors.Wrapf(err, "failed to build group<%s>", group.ModuleName)
			}
			if err := e.exportGroupImage(ctx, group); err != nil {
				return errors.Wrapf(err, "failed to export image of group<%s>", group.ModuleName)
			}
		}
		return e.buildSMartPackageV2(ctx)
	}
	for _, step := range e.plan.Steps {
		if err := e.buildStep(ctx, step); err != nil {
			return errors.Wrapf(err, "failed to build module<%s>", step.ModuleName)
		}
		if err := e.exportImageLayer(ctx, step); err != nil {
			return errors.Wrapf(err, "failed to export image of module<%s>", step.ModuleName)
		}
	}
	return e.buildSMartPackage(ctx)
}

func (e *ContainerExecutor) prepareFakeRunner(ctx context.Context) error {
	// 用 image import 构造只有一层的空镜像：
	// FROM scratch + COPY 在部分运行时会得到 0 层镜像，
	// exporter 需要 run image 的 top layer，0 层会报 "image has no layers"。
	dir, err := os.MkdirTemp("", "fake-runner")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	keep := filepath.Join(dir, "keep")
	if err := os.WriteFile(keep, []byte{}, 0o644); err != nil {
		return err
	}
	tarPath := filepath.Join(dir, "rootfs.tar")
	if err := tarDir(dir, tarPath); err != nil {
		return err
	}

	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	args := imageImportArgs(e.runtime.Platform, fakeRunnerImage(e.runtime.Platform))
	cmd := exec.CommandContext(ctx, e.runtime.Outer.Binary, args...)
	cmd.Stdin = f
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type buildInput struct {
	Name               string
	RunName            string
	Src                string
	Procfile           string
	RequiredBuildpacks string
	OutputImage        string
	ArchiveName        string
	CacheRegistry      string
	RegistryAuth       string
	Envs               map[string]string
}

func (e *ContainerExecutor) buildStep(ctx context.Context, step *plan.ModuleBuildStep) error {
	return e.runBuild(ctx, buildInput{
		Name:               step.ModuleName,
		Src:                step.Src,
		Procfile:           step.Procfile,
		RequiredBuildpacks: step.RequiredBuildpacks(),
		OutputImage:        step.OutputImage(),
		ArchiveName:        step.ModuleName + ".tgz",
		CacheRegistry:      step.CacheRegistry,
		RegistryAuth:       step.RegistryAuth,
		Envs:               step.Envs,
	})
}

func (e *ContainerExecutor) buildGroup(ctx context.Context, group *plan.ModuleBuildGroup) error {
	return e.runBuild(ctx, buildInput{
		Name:               group.ModuleName,
		Src:                group.Src,
		Procfile:           group.Procfile,
		RequiredBuildpacks: group.RequiredBuildpacks(),
		OutputImage:        group.OutputImage(),
		ArchiveName:        group.OutputImageTarName,
		CacheRegistry:      group.CacheRegistry,
		RegistryAuth:       group.RegistryAuth,
		Envs:               group.Envs,
	})
}

// runBuild 执行一次 builder 构建；v1 每模块调用一次，v2 每构建组调用一次。
func (e *ContainerExecutor) runBuild(ctx context.Context, in buildInput) error {
	info, err := os.Stat(in.Src)
	if err != nil {
		return errors.Wrap(err, "src is not found")
	}

	sourcePath := e.workspace.SourcePath()
	if info.IsDir() {
		if err := source.Build(in.Src, in.Procfile, sourcePath); err != nil {
			return errors.Wrap(err, "failed to archive src")
		}
	} else if err := utils.Copy(in.Src, sourcePath); err != nil {
		return errors.Wrap(err, "failed to copy source archive")
	}

	platformDir := e.workspace.PlatformDir()
	if err := os.RemoveAll(platformDir); err != nil {
		return errors.Wrap(err, "failed to reset platform dir")
	}
	if err := os.MkdirAll(platformDir, 0o755); err != nil {
		return errors.Wrap(err, "failed to create platform dir")
	}
	if err := env.WritePlatform(platformDir, in.Envs); err != nil {
		return errors.Wrap(err, "failed to write platform env")
	}

	in.RunName = buildContainerName(e.plan.AppCode, in.Name)
	args := e.buildArgs(in)
	args = append(args, e.cnbBuilderImage)
	log.Info("执行云原生构建", "args", args)
	cmd := exec.CommandContext(ctx, e.runtime.Outer.Binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil || ctx.Err() != nil {
		e.removeContainer(in.RunName)
	}
	return err
}

// buildArgs 只从这里产生容器路径；业务代码不拼接宿主机路径。
func (e *ContainerExecutor) buildArgs(in buildInput) []string {
	a := newArgBuilder("run", "--rm")
	if in.RunName != "" {
		a.option("--name", in.RunName)
	}
	a.env(container.EnvRequiredBuildpacks, in.RequiredBuildpacks)
	a.env(container.EnvOutputImage, in.OutputImage)
	a.env(container.EnvRunImage, fakeRunnerImage(e.runtime.Platform))
	a.env(container.EnvSourceGetURL, fileURL(e.workspace.SourceContainerPath()))
	a.env(container.EnvUseDockerDaemon, "1")
	a.env(container.EnvPlatformAPI, "0.11")
	a.env(container.EnvCNBBuildpackAliases, e.runtime.BuildpackAliases)

	// 缓存镜像：只在显式配置 cache registry 时启用，避免默认依赖外部仓库。
	if in.CacheRegistry != "" {
		cacheImage := strings.Join([]string{in.CacheRegistry, e.plan.AppCode, in.Name}, "/") + cacheImageTag
		a.env(container.EnvCacheImage, cacheImage)
		if in.RegistryAuth != "" {
			a.env(container.EnvRegistryAuth, in.RegistryAuth)
		}
	}
	// 交叉构建时显式传平台，避免 qemu 下镜像架构不一致。
	if pf := e.runtime.Platform; pf != "" {
		a.option("--platform", pf)
		osName, arch := splitPlatform(pf)
		a.env(container.EnvTargetOS, osName)
		a.env(container.EnvTargetArch, arch)
	}
	// 单次构建的所有内部路径统一在 /cnb/work 下。
	a.bindMount(e.workspace.Host, e.workspace.Container, false)
	// 容器运行时 socket：podman/pind 使用解析后的 rootful/rootless socket。
	a.bindMount(e.runtime.Inner.Socket, container.DockerSocketContainerPath, false)
	a.env(container.EnvDockerHost, container.DockerHostURL)
	if e.runtime.BuilderUser != "" {
		a.option("--user", e.runtime.BuilderUser)
	}
	if e.runtime.BuilderUserNS != "" {
		a.option("--userns", e.runtime.BuilderUserNS)
	}
	// lifecycle 的 platform 目录固定为 /platform，app 目录固定为 /workspace。
	a.bindMount(e.workspace.PlatformDir(), e.workspace.PlatformContainerPath(), true)
	a.bindMount(e.workspace.AppDir(), WorkspaceAppMount, false)
	// 持久文件缓存独立于 workspace；用户显式挂载时不重复添加。
	if e.runtime.Cache.Enabled() && !hasMountTarget(e.runtime.ExtraMounts, e.runtime.Cache.ContainerPath) {
		a.bindMount(e.runtime.Cache.HostPath, e.runtime.Cache.ContainerPath, false)
		a.env(container.EnvCacheDir, e.runtime.Cache.ContainerPath)
	}
	// layout 是 podman 下的默认导出方式；daemon 仅适用于真 dockerd。
	if e.runtime.ExportMode == container.ExportDaemon {
		a.env(container.EnvCNBExportMode, string(container.ExportDaemon))
	} else {
		a.env(container.EnvCNBExportMode, string(container.ExportLayout))
		a.env(container.EnvLayerArchive, e.workspace.LayerContainerPath(in.ArchiveName))
	}
	// run image 位于 workspace/run，随根挂载进入容器。
	if e.runImageArchive != "" {
		a.env(container.EnvRunImageArchive, e.workspace.RunImageContainerPath())
	}
	// 网络模式：host 可访问宿主机内网镜像源。
	if net := e.runtime.Network; net != "" {
		a.option("--network", net)
	}
	// 额外挂载已由容器配置解析，原样传给运行时。
	a.add(e.runtime.ExtraMounts...)
	return a.args()
}

const (
	containerCleanupTimeout = 30 * time.Second
	cacheImageTag           = ":cnb-build-cache"
)

func buildContainerName(appCode, module string) string {
	return strings.Join([]string{
		"cnb-build", appCode, module,
		strconv.FormatInt(time.Now().UnixNano(), 36),
	}, "-")
}

func (e *ContainerExecutor) removeContainer(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), containerCleanupTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.runtime.Outer.Binary, "rm", "-f", name)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
}

func fileURL(pathname string) string {
	u := &url.URL{Scheme: "file", Path: pathname}
	return u.String()
}

func hasMountTarget(mounts []string, target string) bool {
	for _, mount := range mounts {
		if strings.Contains(mount, "target="+target) {
			return true
		}
	}
	return false
}

func (e *ContainerExecutor) ensureRuntime(ctx context.Context) error {
	if e.runtime.Driver != container.DriverManaged {
		return nil
	}
	if e.runtime.RuntimeImage != "" {
		return e.startSidecar(ctx)
	}
	if _, err := os.Stat(e.runtime.Inner.Socket); err != nil {
		return errors.Wrapf(err, "managed runtime socket %s is not ready; set CNB_RUNTIME_IMAGE to start a sidecar", e.runtime.Inner.Socket)
	}
	if !e.runtimeHealthy(ctx) {
		return fmt.Errorf("managed runtime socket %s exists but is not healthy", e.runtime.Inner.Socket)
	}
	return nil
}

func loadedImageName(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "Loaded image:"); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (e *ContainerExecutor) prepareRunImage(ctx context.Context) error {
	if e.runImageArchive != "" {
		if e.runImageArchive != e.workspace.RunImagePath() {
			if err := utils.Copy(e.runImageArchive, e.workspace.RunImagePath()); err != nil {
				return errors.Wrap(err, "failed to copy run image archive")
			}
		}
		e.runImageArchive = e.workspace.RunImagePath()
		return e.loadRunImageIntoInner(ctx, "")
	}
	if err := e.prepareFakeRunner(ctx); err != nil {
		return errors.Wrap(err, "failed to build fake runner")
	}
	if err := e.exportFakeRunnerArchive(ctx); err != nil {
		return errors.Wrap(err, "failed to export fake runner")
	}
	return e.loadRunImageIntoInner(ctx, fakeRunnerImage(e.runtime.Platform))
}

// loadRunImageIntoInner 在 daemon 导出模式下把 run image 放进 inner runtime。
// imageName 非空时确保加载后的镜像有可引用的 tag。
func (e *ContainerExecutor) loadRunImageIntoInner(ctx context.Context, imageName string) error {
	if e.runtime.ExportMode != container.ExportDaemon {
		return nil
	}
	cli := newRuntimeCLI(e.runtime.Inner)
	output, err := cli.output(ctx, "image", "load", "-i", e.runImageArchive)
	if err != nil {
		return errors.Wrap(err, "failed to load run image into inner runtime")
	}
	log.Info("load run image into inner runtime", "output", strings.TrimSpace(output))
	if imageName == "" {
		return nil
	}
	// docker load 可能把镜像保存为 localhost/name，需要显式补上无前缀 tag。
	source := loadedImageName(output)
	if source == "" {
		ids, listErr := cli.images(ctx)
		if listErr != nil || len(ids) == 0 {
			return errors.Wrapf(errors.New("loaded image name not found"), "run image %s not found in inner runtime after load", imageName)
		}
		source = ids[0]
	}
	if _, tagErr := cli.output(ctx, "image", "tag", source, imageName); tagErr != nil {
		return errors.Wrapf(tagErr, "failed to tag run image %s from %s", imageName, source)
	}
	log.Info("tag run image in inner runtime", "source", source, "image", imageName)
	return nil
}

func (e *ContainerExecutor) exportArchive(ctx context.Context, archiveName, image string) error {
	layerDir := e.workspace.LayersDir()
	if err := os.MkdirAll(layerDir, 0o755); err != nil {
		return err
	}
	produced := filepath.Join(layerDir, archiveName)
	if fi, err := os.Stat(produced); err == nil && fi.Size() > 0 {
		fmt.Printf("using layer archive from cnb: %s\n", produced)
		if err := utils.Copy(produced, filepath.Join(e.workspace.OutDir(), archiveName)); err != nil {
			return err
		}
		return os.Remove(produced)
	}
	inner := newRuntimeCLI(e.runtime.Inner)
	args := append(inner.connectionArgs(), imageSaveArgs(e.runtime.Inner, filepath.Join(e.workspace.OutDir(), archiveName), image)...)
	cmd := exec.CommandContext(ctx, e.runtime.Inner.Binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (e *ContainerExecutor) exportImageLayer(ctx context.Context, step *plan.ModuleBuildStep) error {
	return e.exportArchive(ctx, step.ModuleName+".tgz", step.OutputImage())
}

func (e *ContainerExecutor) exportGroupImage(ctx context.Context, group *plan.ModuleBuildGroup) error {
	return e.exportArchive(ctx, group.OutputImageTarName, group.OutputImage())
}

func (e *ContainerExecutor) buildSMartPackageV2(_ context.Context) error {
	if e.plan.Architecture == "" {
		if pf := e.runtime.Platform; pf != "" {
			_, arch := splitPlatform(pf)
			e.plan.Architecture = arch
		} else {
			e.plan.Architecture = runtime.GOARCH
		}
	}
	out := e.workspace.OutDir()
	if err := os.WriteFile(filepath.Join(out, "app_desc.yaml"), []byte(e.plan.AppDescContent), 0o644); err != nil {
		return errors.Wrap(err, "failed to write app_desc.yaml")
	}
	if err := os.WriteFile(filepath.Join(out, ".Version"), []byte("cnb-image-layers"), 0o644); err != nil {
		return errors.Wrap(err, "failed to write .Version")
	}
	if e.plan.LogoFileName != "" {
		if err := utils.Copy(filepath.Join(e.plan.Src, e.plan.LogoFileName), filepath.Join(out, e.plan.LogoFileName)); err != nil {
			return errors.Wrapf(err, "failed to copy logo %s", e.plan.LogoFileName)
		}
	}
	artifact, err := plan.BuildArtifactJSON(&e.plan)
	if err != nil {
		return errors.Wrap(err, "failed to build artifact.json")
	}
	if err := os.WriteFile(filepath.Join(out, "artifact.json"), artifact, 0o644); err != nil {
		return errors.Wrap(err, "failed to write artifact.json")
	}
	for _, group := range e.plan.Groups {
		p := filepath.Join(out, group.OutputImageTarName)
		if fi, err := os.Stat(p); err != nil || fi.Size() == 0 {
			return errors.Errorf("image tar %s is missing or empty", group.OutputImageTarName)
		}
	}
	if err := os.RemoveAll(e.plan.Dest); err != nil {
		return errors.Wrap(err, "failed to reset dest")
	}
	if err := utils.PackDirTarGz(out, e.plan.Dest); err != nil {
		return errors.Wrap(err, "failed to archive package")
	}
	return nil
}

func (e *ContainerExecutor) buildSMartPackage(_ context.Context) error {
	out := e.workspace.OutDir()
	appDescPath := filepath.Join(out, "app_desc.yaml")
	if err := os.WriteFile(appDescPath, []byte(e.plan.AppDescContent), 0o644); err != nil {
		return errors.Wrap(err, "failed to write app_desc.yaml")
	}

	builderFlagPath := filepath.Join(out, ".Version")
	if err := os.WriteFile(builderFlagPath, []byte("cnb-image-layers"), 0o644); err != nil {
		return errors.Wrap(err, "failed to write builder flag .Version")
	}

	if e.plan.LogoFileName != "" {
		logoDestPath := filepath.Join(out, e.plan.LogoFileName)
		err := utils.Copy(filepath.Join(e.plan.Src, e.plan.LogoFileName), logoDestPath)
		if err != nil {
			return errors.Wrapf(err, "failed to copy src to %s", logoDestPath)
		}
	}

	artifactPath := filepath.Join(e.plan.Src, "artifact.json")
	if _, err := os.Stat(artifactPath); err == nil {
		if err := utils.Copy(artifactPath, filepath.Join(out, "artifact.json")); err != nil {
			return errors.Wrap(err, "failed to copy artifact.json")
		}
	}

	if err := os.RemoveAll(e.plan.Dest); err != nil {
		return errors.Wrap(err, "failed to reset dest")
	}
	if err := utils.PackDirTarGz(out, e.plan.Dest); err != nil {
		return errors.Wrap(err, "failed to archive src")
	}
	return nil
}

// tarDir 把目录打包成 tar（供 image import 使用）。
func tarDir(src, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	tw := tar.NewWriter(out)
	defer func() { _ = tw.Close() }()

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if path == dest {
			return nil // 不要把自己打进 tar
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		hdr := &tar.Header{Name: rel, Mode: 0o644, Size: info.Size()}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		_, err = io.Copy(tw, f)
		return err
	})
}

// exportFakeRunnerArchive 把 fake runner 导出为 docker-archive，供容器内使用。
func (e *ContainerExecutor) exportFakeRunnerArchive(ctx context.Context) error {
	path := e.workspace.RunImagePath()
	args := dockerArchiveSaveArgs(path, fakeRunnerImage(e.runtime.Platform))
	cmd := exec.CommandContext(ctx, e.runtime.Outer.Binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	e.runImageArchive = path
	return nil
}

// splitPlatform 拆分 os/arch。
func splitPlatform(p string) (string, string) {
	if osName, arch, ok := strings.Cut(p, "/"); ok {
		return osName, arch
	}
	return "linux", p
}

// fakeRunnerImage 按目标平台返回 fake runner 镜像名，避免架构间互相覆盖。
func fakeRunnerImage(platform string) string {
	if pf := platform; pf != "" {
		return cnbFakeRunnerImageBase + ":" + strings.ReplaceAll(pf, "/", "-")
	}
	return cnbFakeRunnerImageBase + ":latest"
}
