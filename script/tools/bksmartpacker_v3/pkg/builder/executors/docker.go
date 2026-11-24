package executors

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/samber/lo"

	"bksmartpacker/pkg/builder"
	"bksmartpacker/pkg/log"
	"bksmartpacker/pkg/utils"
)

var dockerExecPath string

type DockerExecutor struct {
	plan            builder.BuildPlan
	dockerExecPath  string
	cnbBuilderImage string
}

// NewDockerExecutor ...
func NewDockerExecutor(plan builder.BuildPlan) (*DockerExecutor, error) {
	if dockerExecPath == "" {
		// 临时改动：允许用 podman 验证（本机无 docker 时）
		candidates := []string{os.Getenv("BKS_CONTAINER_CLI"), "docker", "podman"}
		var lastErr error
		for _, c := range candidates {
			if c == "" {
				continue
			}
			if p, err := exec.LookPath(c); err == nil {
				dockerExecPath = p
				break
			} else {
				lastErr = err
			}
		}
		if dockerExecPath == "" {
			return nil, errors.Wrap(lastErr, "failed to lookup docker/podman")
		}
	}
	builderImage := DefaultCNBBuilderImage
	if plan.BuilderImage != "" {
		builderImage = plan.BuilderImage
	}
	return &DockerExecutor{
		plan:            plan,
		dockerExecPath:  dockerExecPath,
		cnbBuilderImage: builderImage,
	}, nil
}

// Execute build plan with docker
func (e *DockerExecutor) Execute() error {
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

func (e *DockerExecutor) prepareFakeRunner(ctx context.Context) error {
	// 用 image import 构造只有一层的空镜像：
	// FROM scratch + COPY 在部分运行时会得到 0 层镜像，
	// exporter 需要 run image 的 top layer，0 层会报 "image has no layers"。
	dir, err := os.MkdirTemp("", "fake-runner")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

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
	defer f.Close()

	args := []string{"import", "-c", "ENV HOME=/app", "-c", "WORKDIR /app"}
	if pf := targetPlatform(); pf != "" {
		osName, arch := splitPlatform(pf)
		args = append(args, "--os", osName, "--arch", arch)
	}
	args = append(args, "-", fakeRunnerImage())
	cmd := exec.CommandContext(ctx, e.dockerExecPath, args...)
	cmd.Stdin = f
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (e *DockerExecutor) buildStep(ctx context.Context, step *builder.ModuleBuildStep, layerDir string) error {
	var tarballPath string
	if info, err := os.Stat(step.Src); err != nil {
		return errors.Wrap(err, "step src is not found")
	} else if !info.IsDir() {
		tarballPath = step.Src
	} else {
		fh, err := os.CreateTemp("", "*.tgz")
		if err != nil {
			return errors.Wrap(err, "failed to archive src")
		}
		_ = os.Remove(fh.Name())
		if err = builder.BuildSourceTarball(step.Src, step.Procfile, fh.Name()); err != nil {
			return errors.Wrap(err, "failed to archive src!")
		}
		tarballPath = fh.Name()
	}

	args := []string{
		"run", "--rm",
		"-e", fmt.Sprintf("REQUIRED_BUILDPACKS=%s", step.RequiredBuildpacks()),
		"-e", fmt.Sprintf("OUTPUT_IMAGE=%s", step.OutputImage()),
		"-e", fmt.Sprintf("CNB_RUN_IMAGE=%s", fakeRunnerImage()),
		"-e", fmt.Sprintf("SOURCE_GET_URL=file:///tmp/source.tgz"),
		"-e", "USE_DOCKER_DAEMON=1",
		"-e", "CNB_PLATFORM_API=0.11",
		"-e", "CNB_BUILDPACK_ALIASES=" + buildpackAliases(),
	}

	if step.CacheRegistry != "" {
		cacheImage := fmt.Sprintf("%s/%s/%s:cnb-build-cache", step.CacheRegistry, step.AppCode, step.ModuleName)
		args = append(args, "-e", fmt.Sprintf("CACHE_IMAGE=%s", cacheImage))

		if step.RegistryAuth != "" {
			args = append(args, "-e", fmt.Sprintf("CNB_REGISTRY_AUTH=%s", step.RegistryAuth))
		}
	}

	// 描述文件声明的环境变量
	lo.ForEach(step.BuildEnvs(), func(env string, index int) {
		args = append(args, "-e", env)
	})

	// 目标平台（qemu 交叉构建时显式指定）
	if pf := targetPlatform(); pf != "" {
		args = append(args, "--platform", pf)
	}

	// 挂载容器运行时 socket（默认 /var/run/docker.sock，可用 BKS_DOCKER_SOCK 换成 podman）
	sock := os.Getenv("BKS_DOCKER_SOCK")
	if sock == "" {
		sock = "/var/run/docker.sock"
	}
	args = append(args, "--mount", fmt.Sprintf("type=bind,source=%s,target=/var/run/docker.sock", sock))
	args = append(args, "-e", "DOCKER_HOST=unix:///var/run/docker.sock")

	// 挂载源码压缩包
	args = append(args, "--mount", fmt.Sprintf("type=bind,source=%s,target=/tmp/source.tgz", tarballPath))
	// 导出方式：
	//   layout（默认）：容器内导出 OCI layout，再打包为层包写回宿主目录
	//   daemon：导出到容器运行时，构建后由 image save 取回。
	//           仅适用于真 dockerd。podman 的 docker 兼容层会让 lifecycle
	//           导出阶段报 "singleton connection already in use"
	//           （lifecycle 0.21.29，issue #1763，尚未修复）；
	//           该模式保留用于 docker 节点，podman 下必须用 layout。
	if exportMode() == "daemon" {
		args = append(args, "-e", "CNB_EXPORT_MODE=daemon")
	} else {
		args = append(args, "-e", "CNB_EXPORT_MODE=layout")
		args = append(args, "--mount", fmt.Sprintf("type=bind,source=%s,target=/cnb-layer", layerDir))
		args = append(args, "-e", fmt.Sprintf("CNB_LAYER_ARCHIVE=/cnb-layer/%s.tgz", step.ModuleName))
	}
	// run image 以 docker-archive 形式落盘并挂载，容器内不再需要 daemon
	if archive := os.Getenv("BKS_RUN_IMAGE_ARCHIVE_FILE"); archive != "" {
		args = append(args, "--mount", fmt.Sprintf("type=bind,source=%s,target=/cnb-run/run-image.tar,readonly", archive))
		args = append(args, "-e", "CNB_RUN_IMAGE_ARCHIVE=/cnb-run/run-image.tar")
	}

	// 网络模式：BKS_NETWORK=host 可让容器直接访问宿主机网络
	if net := os.Getenv("BKS_NETWORK"); net != "" {
		args = append(args, "--network", net)
	}

	// 额外挂载：BKS_EXTRA_MOUNTS="--mount type=bind,source=...,target=/cnb/runtimes"
	for _, extra := range strings.Fields(os.Getenv("BKS_EXTRA_MOUNTS")) {
		args = append(args, extra)
	}

	args = append(args, e.cnbBuilderImage)
	log.Info("执行云原生构建: ", args)
	cmd := exec.CommandContext(ctx, e.dockerExecPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (e *DockerExecutor) exportImageLayer(ctx context.Context, workspace string, step *builder.ModuleBuildStep) error {
	produced := filepath.Join(workspace, "layers", step.ModuleName+".tgz")
	if fi, err := os.Stat(produced); err == nil && fi.Size() > 0 {
		fmt.Printf("using layer archive from cnb: %s\n", produced)
		if err := utils.Copy(produced, filepath.Join(workspace, step.ModuleName+".tgz")); err != nil {
			return err
		}
		return os.RemoveAll(filepath.Join(workspace, "layers"))
	}
	args := saveArgs(e.dockerExecPath, fmt.Sprintf("%s/%s.tgz", workspace, step.ModuleName), step.OutputImage())
	cmd := exec.CommandContext(ctx, e.dockerExecPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (e *DockerExecutor) buildSMartPackage(ctx context.Context, workspace string) error {
	// run-image.tar 只是构建期中间产物，不进 s-mart
	_ = os.Remove(filepath.Join(workspace, "run-image.tar"))
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
	defer out.Close()

	tw := tar.NewWriter(out)
	defer tw.Close()

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
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}

// exportFakeRunnerArchive 把 fake runner 导出为 docker-archive，供容器内使用。
func (e *DockerExecutor) exportFakeRunnerArchive(ctx context.Context, workspace string) error {
	path := filepath.Join(workspace, "run-image.tar")
	cmd := exec.CommandContext(ctx, e.dockerExecPath, "image", "save", "-o", path, fakeRunnerImage())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return os.Setenv("BKS_RUN_IMAGE_ARCHIVE_FILE", path)
}

// defaultBuildpackAliases 是 BK 名称到 order id 的映射；
// 可用 BKS_BUILDPACK_ALIASES 覆盖（格式 a=b,c=d）。
const defaultBuildpackAliases = "bk-buildpack-go=paketo-buildpacks/go," +
	"bk-buildpack-nodejs=paketo-buildpacks/nodejs," +
	"bk-buildpack-python=paketo-buildpacks/python"

// buildpackAliases 返回注入给 cnb 的别名表。
func buildpackAliases() string {
	if v := os.Getenv("BKS_BUILDPACK_ALIASES"); v != "" {
		return v
	}
	return defaultBuildpackAliases
}

// saveArgs 生成镜像导出参数。
// podman 使用 --format oci-archive 直接产出 OCI layout（蓝鲸平台期望的格式）；
// docker 无该参数，保持默认（容器存储为 containerd 时本身即 OCI layout）。
func saveArgs(runtime, dest, image string) []string {
	args := []string{"image", "save"}
	if filepath.Base(runtime) == "podman" {
		args = append(args, "--format", "oci-archive")
	}
	return append(args, "-o", dest, image)
}

// exportMode 返回导出模式（BKS_EXPORT_MODE，默认 layout）。
func exportMode() string {
	if mode := os.Getenv("BKS_EXPORT_MODE"); mode == "daemon" {
		return "daemon"
	}
	return "layout"
}

// targetPlatform 返回 BKS_PLATFORM（如 linux/arm64），未设置时为空。
func targetPlatform() string { return os.Getenv("BKS_PLATFORM") }

// splitPlatform 拆分 os/arch。
func splitPlatform(p string) (string, string) {
	if osName, arch, ok := strings.Cut(p, "/"); ok {
		return osName, arch
	}
	return "linux", p
}

// fakeRunnerImage 按目标平台返回 fake runner 镜像名，避免架构间互相覆盖。
func fakeRunnerImage() string {
	if pf := targetPlatform(); pf != "" {
		return cnbFakeRunnerImageBase + ":" + strings.ReplaceAll(pf, "/", "-")
	}
	return cnbFakeRunnerImageBase + ":latest"
}
