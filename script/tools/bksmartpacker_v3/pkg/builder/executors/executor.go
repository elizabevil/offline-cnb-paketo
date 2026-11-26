package executors

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"

	"bksmartpacker/pkg/builder"
	"bksmartpacker/pkg/container"
	"bksmartpacker/pkg/log"
	"bksmartpacker/pkg/utils"
)

// ContainerExecutor 使用 docker 或 podman 执行构建计划。
type ContainerExecutor struct {
	plan            builder.BuildPlan
	runtime         container.Config
	cnbBuilderImage string
	runImageArchive string
}

// NewContainerExecutor 使用已解析的运行时配置创建执行器。
func NewContainerExecutor(plan builder.BuildPlan, runtimeCfg container.Config) (*ContainerExecutor, error) {
	builderImage := DefaultCNBBuilderImage
	if plan.BuilderImage != "" {
		builderImage = plan.BuilderImage
	}
	log.Info("container runtime", "backend", runtimeCfg.Backend, "environment", runtimeCfg.Environment, "binary", runtimeCfg.Binary, "socket", runtimeCfg.Socket, "export", runtimeCfg.ExportMode)
	return &ContainerExecutor{
		plan:            plan,
		runtime:         runtimeCfg,
		cnbBuilderImage: builderImage,
		runImageArchive: runtimeCfg.RunImageArchive,
	}, nil
}

// Execute 使用选中的容器运行时执行构建计划。
func (e *ContainerExecutor) Execute() error {
	// 生成临时目录, 用于存放输出的镜像层
	workspace, err := os.MkdirTemp("", "workspace")
	if err != nil {
		return err
	}

	// TODO: add timeout?
	ctx := context.Background()
	if err := e.prepareFakeRunner(ctx); err != nil {
		return errors.Wrap(err, "failed to build fake runner")
	}
	if err := e.exportFakeRunnerArchive(ctx, workspace); err != nil {
		return errors.Wrap(err, "failed to export fake runner")
	}

	layerDir := filepath.Join(workspace, "layers")
	if err := os.MkdirAll(layerDir, 0o755); err != nil {
		return err
	}

	if e.plan.PackagingVersion == builder.PackagingVersionV2 {
		for _, group := range e.plan.Groups {
			if err := e.buildGroup(ctx, group, layerDir); err != nil {
				return errors.Wrapf(err, "failed to build group<%s>", group.ModuleName)
			}
			if err := e.exportGroupImage(ctx, workspace, group); err != nil {
				return errors.Wrapf(err, "failed to export image of group<%s>", group.ModuleName)
			}
		}
		return e.buildSMartPackageV2(ctx, workspace)
	}
	for _, step := range e.plan.Steps {
		if err := e.buildStep(ctx, step, layerDir); err != nil {
			return errors.Wrapf(err, "failed to build module<%s>", step.ModuleName)
		}
		if err := e.exportImageLayer(ctx, workspace, step); err != nil {
			return errors.Wrapf(err, "failed to export image of module<%s>", step.ModuleName)
		}
	}
	return e.buildSMartPackage(ctx, workspace)
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

	args := []string{"import", "-c", "ENV HOME=/app", "-c", "WORKDIR /app"}
	if pf := e.runtime.Platform; pf != "" {
		osName, arch := splitPlatform(pf)
		args = append(args, "--os", osName, "--arch", arch)
	}
	args = append(args, "-", fakeRunnerImage(e.runtime.Platform))
	cmd := exec.CommandContext(ctx, e.runtime.Binary, args...)
	cmd.Stdin = f
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type buildInput struct {
	Name               string
	Src                string
	Procfile           string
	RequiredBuildpacks string
	OutputImage        string
	ArchiveName        string
	CacheRegistry      string
	RegistryAuth       string
	Envs               map[string]string
}

func (e *ContainerExecutor) buildStep(ctx context.Context, step *builder.ModuleBuildStep, layerDir string) error {
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
	}, layerDir)
}

func (e *ContainerExecutor) buildGroup(ctx context.Context, group *builder.ModuleBuildGroup, layerDir string) error {
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
	}, layerDir)
}

// runBuild 执行一次 builder 构建；v1 每模块调用一次，v2 每构建组调用一次。
func (e *ContainerExecutor) runBuild(ctx context.Context, in buildInput, layerDir string) error {
	info, err := os.Stat(in.Src)
	if err != nil {
		return errors.Wrap(err, "src is not found")
	}

	var tarballPath string
	if !info.IsDir() {
		tarballPath = in.Src
	} else {
		fh, err := os.CreateTemp("", "*.tgz")
		if err != nil {
			return errors.Wrap(err, "failed to archive src")
		}
		_ = os.Remove(fh.Name())
		if err := builder.BuildSourceTarball(in.Src, in.Procfile, fh.Name()); err != nil {
			return errors.Wrap(err, "failed to archive src")
		}
		tarballPath = fh.Name()
	}

	platformDir, err := os.MkdirTemp("", "platform")
	if err != nil {
		return errors.Wrap(err, "failed to create platform dir")
	}
	defer func() { _ = os.RemoveAll(platformDir) }()
	if err := os.Chmod(platformDir, 0o755); err != nil {
		return errors.Wrap(err, "failed to chmod platform dir")
	}
	if err := writePlatformEnv(platformDir, in.Envs); err != nil {
		return errors.Wrap(err, "failed to write platform env")
	}

	args := e.buildArgs(in, layerDir, platformDir, tarballPath)
	args = append(args, e.cnbBuilderImage)
	log.Info("执行云原生构建", "args", args)
	cmd := exec.CommandContext(ctx, e.runtime.Binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (e *ContainerExecutor) buildArgs(in buildInput, layerDir, platformDir, tarballPath string) []string {
	args := []string{
		"run", "--rm",
		"-e", fmt.Sprintf("REQUIRED_BUILDPACKS=%s", in.RequiredBuildpacks),
		"-e", fmt.Sprintf("OUTPUT_IMAGE=%s", in.OutputImage),
		"-e", fmt.Sprintf("CNB_RUN_IMAGE=%s", fakeRunnerImage(e.runtime.Platform)),
		"-e", "SOURCE_GET_URL=file:///tmp/source.tgz",
		"-e", "USE_DOCKER_DAEMON=1",
		"-e", "CNB_PLATFORM_API=0.11",
		"-e", "CNB_BUILDPACK_ALIASES=" + e.runtime.BuildpackAliases,
	}
	// 缓存镜像：只在显式配置 cache registry 时启用，避免默认依赖外部仓库。
	if in.CacheRegistry != "" {
		cacheImage := fmt.Sprintf("%s/%s/%s:cnb-build-cache", in.CacheRegistry, e.plan.AppCode, in.Name)
		args = append(args, "-e", fmt.Sprintf("CACHE_IMAGE=%s", cacheImage))
		if in.RegistryAuth != "" {
			args = append(args, "-e", fmt.Sprintf("CNB_REGISTRY_AUTH=%s", in.RegistryAuth))
		}
	}
	// 交叉构建时显式传平台，避免 qemu 下镜像架构不一致。
	if pf := e.runtime.Platform; pf != "" {
		args = append(args, "--platform", pf)
	}
	// 容器运行时 socket：podman/pind 使用解析后的 rootful/rootless socket。
	args = append(args, "--mount", fmt.Sprintf("type=bind,source=%s,target=/var/run/docker.sock", e.runtime.Socket))
	args = append(args, "-e", "DOCKER_HOST=unix:///var/run/docker.sock")
	// 业务 env 走 /platform/env；lifecycle 只放行白名单环境变量，不能依赖 -e。
	args = append(args, "--mount", fmt.Sprintf("type=bind,source=%s,target=/platform,readonly", platformDir))
	// 源码以只读 tar 挂入容器，避免修改宿主源码目录。
	args = append(args, "--mount", fmt.Sprintf("type=bind,source=%s,target=/tmp/source.tgz", tarballPath))
	// layout 是 podman 下的默认导出方式；daemon 仅适用于真 dockerd。
	if e.runtime.ExportMode == container.ExportDaemon {
		args = append(args, "-e", "CNB_EXPORT_MODE=daemon")
	} else {
		args = append(args, "-e", "CNB_EXPORT_MODE=layout")
		args = append(args, "--mount", fmt.Sprintf("type=bind,source=%s,target=/cnb-layer", layerDir))
		args = append(args, "-e", fmt.Sprintf("CNB_LAYER_ARCHIVE=/cnb-layer/%s", in.ArchiveName))
	}
	// run image 以 docker-archive 形式挂入，避免容器内再访问 daemon。
	if archive := e.runImageArchive; archive != "" {
		args = append(args, "--mount", fmt.Sprintf("type=bind,source=%s,target=/cnb-run/run-image.tar,readonly", archive))
		args = append(args, "-e", "CNB_RUN_IMAGE_ARCHIVE=/cnb-run/run-image.tar")
	}
	// 网络模式：host 可访问宿主机内网镜像源。
	if net := e.runtime.Network; net != "" {
		args = append(args, "--network", net)
	}
	// 额外挂载已由容器配置解析，原样传给运行时。
	args = append(args, e.runtime.ExtraMounts...)
	return args
}

func (e *ContainerExecutor) exportArchive(ctx context.Context, workspace, archiveName, image string) error {
	layerDir := filepath.Join(workspace, "layers")
	if err := os.MkdirAll(layerDir, 0o755); err != nil {
		return err
	}
	produced := filepath.Join(layerDir, archiveName)
	if fi, err := os.Stat(produced); err == nil && fi.Size() > 0 {
		fmt.Printf("using layer archive from cnb: %s\n", produced)
		if err := utils.Copy(produced, filepath.Join(workspace, archiveName)); err != nil {
			return err
		}
		return os.Remove(produced)
	}
	args := e.runtime.SaveArgs(filepath.Join(workspace, archiveName), image)
	cmd := exec.CommandContext(ctx, e.runtime.Binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (e *ContainerExecutor) exportImageLayer(ctx context.Context, workspace string, step *builder.ModuleBuildStep) error {
	return e.exportArchive(ctx, workspace, step.ModuleName+".tgz", step.OutputImage())
}

func (e *ContainerExecutor) exportGroupImage(ctx context.Context, workspace string, group *builder.ModuleBuildGroup) error {
	return e.exportArchive(ctx, workspace, group.OutputImageTarName, group.OutputImage())
}

func (e *ContainerExecutor) buildSMartPackageV2(_ context.Context, workspace string) error {
	if e.plan.Architecture == "" {
		if pf := e.runtime.Platform; pf != "" {
			_, arch := splitPlatform(pf)
			e.plan.Architecture = arch
		} else {
			e.plan.Architecture = runtime.GOARCH
		}
	}
	_ = os.Remove(filepath.Join(workspace, "run-image.tar"))
	_ = os.RemoveAll(filepath.Join(workspace, "layers"))
	if err := os.WriteFile(filepath.Join(workspace, "app_desc.yaml"), []byte(e.plan.AppDescContent), 0o644); err != nil {
		return errors.Wrap(err, "failed to write app_desc.yaml")
	}
	if err := os.WriteFile(filepath.Join(workspace, ".Version"), []byte("cnb-image-layers"), 0o644); err != nil {
		return errors.Wrap(err, "failed to write .Version")
	}
	if e.plan.LogoFileName != "" {
		if err := utils.Copy(filepath.Join(e.plan.Src, e.plan.LogoFileName), filepath.Join(workspace, e.plan.LogoFileName)); err != nil {
			return errors.Wrapf(err, "failed to copy logo %s", e.plan.LogoFileName)
		}
	}
	artifact, err := builder.BuildArtifactJSON(&e.plan)
	if err != nil {
		return errors.Wrap(err, "failed to build artifact.json")
	}
	if err := os.WriteFile(filepath.Join(workspace, "artifact.json"), artifact, 0o644); err != nil {
		return errors.Wrap(err, "failed to write artifact.json")
	}
	for _, group := range e.plan.Groups {
		p := filepath.Join(workspace, group.OutputImageTarName)
		if fi, err := os.Stat(p); err != nil || fi.Size() == 0 {
			return errors.Errorf("image tar %s is missing or empty", group.OutputImageTarName)
		}
	}
	if err := os.RemoveAll(e.plan.Dest); err != nil {
		return errors.Wrap(err, "failed to reset dest")
	}
	if err := utils.PackDirTarGz(workspace, e.plan.Dest); err != nil {
		return errors.Wrap(err, "failed to archive package")
	}
	return nil
}

func (e *ContainerExecutor) buildSMartPackage(_ context.Context, workspace string) error {
	// 构建期中间产物，不进 s-mart
	_ = os.Remove(filepath.Join(workspace, "run-image.tar"))
	_ = os.RemoveAll(filepath.Join(workspace, "layers"))
	appDescPath := filepath.Join(workspace, "app_desc.yaml")
	if err := os.WriteFile(appDescPath, []byte(e.plan.AppDescContent), 0o644); err != nil {
		return errors.Wrap(err, "failed to write app_desc.yaml")
	}

	builderFlagPath := filepath.Join(workspace, ".Version")
	if err := os.WriteFile(builderFlagPath, []byte("cnb-image-layers"), 0o644); err != nil {
		return errors.Wrap(err, "failed to write builder flag .Version")
	}

	if e.plan.LogoFileName != "" {
		logoDestPath := filepath.Join(workspace, e.plan.LogoFileName)
		err := utils.Copy(filepath.Join(e.plan.Src, e.plan.LogoFileName), logoDestPath)
		if err != nil {
			return errors.Wrapf(err, "failed to copy src to %s", logoDestPath)
		}
	}

	artifactPath := filepath.Join(e.plan.Src, "artifact.json")
	if _, err := os.Stat(artifactPath); err == nil {
		if err := utils.Copy(artifactPath, filepath.Join(workspace, "artifact.json")); err != nil {
			return errors.Wrap(err, "failed to copy artifact.json")
		}
	}

	// 打包 tarball：目标已存在时先删除，避免 "file already exists"
	if err := os.RemoveAll(e.plan.Dest); err != nil {
		return errors.Wrap(err, "failed to reset dest")
	}
	if err := utils.PackDirTarGz(workspace, e.plan.Dest); err != nil {
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
func (e *ContainerExecutor) exportFakeRunnerArchive(ctx context.Context, workspace string) error {
	path := filepath.Join(workspace, "run-image.tar")
	cmd := exec.CommandContext(ctx, e.runtime.Binary, "image", "save", "-o", path, fakeRunnerImage(e.runtime.Platform))
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

// writePlatformEnv 把环境变量写成 <platform>/env/<KEY> 文件。
func writePlatformEnv(platformDir string, envs map[string]string) error {
	envDir := filepath.Join(platformDir, "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(envDir, 0o755); err != nil {
		return err
	}
	for key, value := range envs {
		if !validEnvKey(key) {
			return fmt.Errorf("invalid env key %q", key)
		}
		if strings.ContainsAny(value, "\n\r") {
			return fmt.Errorf("env %s contains newline", key)
		}
		if err := os.WriteFile(filepath.Join(envDir, key), []byte(value), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func validEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		switch {
		case r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}
