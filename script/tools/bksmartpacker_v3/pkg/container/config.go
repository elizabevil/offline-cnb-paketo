package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Backend 是容器运行时后端。
type Backend string

const (
	// BackendDocker 使用 Docker 二进制。
	BackendDocker Backend = "docker"
	// BackendPodman 使用 Podman 二进制。
	BackendPodman Backend = "podman"
)

// Environment 是构建所处环境。
type Environment string

const (
	// EnvironmentHost 直接使用宿主机运行时。
	EnvironmentHost Environment = "host"
	// EnvironmentPind 是 podman-in-docker。
	EnvironmentPind Environment = "pind"
	// EnvironmentDind 是 docker-in-docker。
	EnvironmentDind Environment = "dind"
)

// ExportMode 是镜像导出方式。
type ExportMode string

const (
	// ExportLayout 通过 CNB layer archive 导出。
	ExportLayout ExportMode = "layout"
	// ExportDaemon 通过 Docker daemon 导出。
	ExportDaemon ExportMode = "daemon"
)

const (
	// runtimeNamePind 是官方 podman-in-docker 的 RUNTIME 值。
	runtimeNamePind = "pind"
	// runtimeNameDind 是官方 docker-in-docker 的 RUNTIME 值。
	runtimeNameDind = "dind"
	// runtimeNameHostPodman 是本地宿主机 podman 模式。
	runtimeNameHostPodman = "host-podman"
	// runtimeNameHostDocker 是本地宿主机 docker 模式。
	runtimeNameHostDocker = "host-docker"
	// runtimeNamePodman 是 host-podman 的兼容别名。
	runtimeNamePodman = "podman"
	// runtimeNameDocker 是 host-docker 的兼容别名。
	runtimeNameDocker = "docker"

	defaultBuildpackAliases = "bk-buildpack-go=paketo-buildpacks/go," +
		"bk-buildpack-nodejs=paketo-buildpacks/nodejs," +
		"bk-buildpack-python=paketo-buildpacks/python"

	dockerSocket        = "/var/run/docker.sock"
	podmanRootfulSocket = "/run/podman/podman.sock"
	podmanSocketDir     = "podman"
	podmanSocketName    = "podman.sock"
)

// Config 是执行器需要的全部运行时配置。
// 环境变量只在本包解析，executors 不再读取 BKS_*。
type Config struct {
	Backend          Backend
	Environment      Environment
	Binary           string
	Socket           string
	ExportMode       ExportMode
	Platform         string
	Network          string
	ExtraMounts      []string
	RunImageArchive  string
	BuildpackAliases string
}

// IsPodman 返回是否使用 Podman 后端。
func (c Config) IsPodman() bool { return c.Backend == BackendPodman }

// SaveArgs 生成镜像导出参数。
func (c Config) SaveArgs(dest, image string) []string {
	args := []string{"image", "save"}
	if c.IsPodman() {
		args = append(args, "--format", "oci-archive")
	}
	return append(args, "-o", dest, image)
}

// ResolveFromEnv 从环境变量解析运行时配置。
//
// 选择顺序：
//  1. BKS_RUNTIME / RUNTIME：
//     pind、dind、host-podman、host-docker、podman、docker
//  2. BKS_CONTAINER_CLI：覆盖二进制
//  3. 自动探测 docker、podman，环境为 host
func ResolveFromEnv() (Config, error) {
	runtimeName := strings.TrimSpace(os.Getenv("BKS_RUNTIME"))
	if runtimeName == "" {
		runtimeName = strings.TrimSpace(os.Getenv("RUNTIME"))
	}
	selection, err := parseRuntimeSelection(runtimeName)
	if err != nil {
		return Config{}, err
	}
	backend, err := resolveBackend(selection, strings.TrimSpace(os.Getenv("BKS_CONTAINER_CLI")))
	if err != nil {
		return Config{}, err
	}
	return newConfig(backend, selection.Environment, strings.TrimSpace(os.Getenv("BKS_CONTAINER_CLI")))
}

// runtimeSelection 是 RUNTIME 解析出的环境与后端组合。
// Backend 为空表示需要自动探测。
type runtimeSelection struct {
	Environment Environment
	Backend     Backend
}

func parseRuntimeSelection(runtimeName string) (runtimeSelection, error) {
	switch strings.ToLower(strings.TrimSpace(runtimeName)) {
	case "":
		return runtimeSelection{Environment: EnvironmentHost}, nil
	case runtimeNamePind:
		return runtimeSelection{Environment: EnvironmentPind, Backend: BackendPodman}, nil
	case runtimeNameDind:
		return runtimeSelection{Environment: EnvironmentDind, Backend: BackendDocker}, nil
	case runtimeNamePodman, runtimeNameHostPodman:
		return runtimeSelection{Environment: EnvironmentHost, Backend: BackendPodman}, nil
	case runtimeNameDocker, runtimeNameHostDocker:
		return runtimeSelection{Environment: EnvironmentHost, Backend: BackendDocker}, nil
	default:
		return runtimeSelection{}, fmt.Errorf("unsupported BKS_RUNTIME %q: must be pind, dind, host-podman or host-docker", runtimeName)
	}
}

func resolveBackend(selection runtimeSelection, explicitBinary string) (Backend, error) {
	if explicitBinary != "" {
		backend, err := backendFromBinary(explicitBinary)
		if err != nil {
			return "", err
		}
		if selection.Backend != "" && selection.Backend != backend {
			return "", fmt.Errorf("runtime environment %s requires the %s backend, got %s", selection.Environment, selection.Backend, backend)
		}
		return backend, nil
	}
	if selection.Backend != "" {
		return selection.Backend, nil
	}
	for _, candidate := range []Backend{BackendDocker, BackendPodman} {
		if _, err := exec.LookPath(string(candidate)); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("failed to find docker or podman; set BKS_RUNTIME or BKS_CONTAINER_CLI")
}

func backendFromBinary(binary string) (Backend, error) {
	switch base := Backend(filepath.Base(binary)); base {
	case BackendDocker:
		return BackendDocker, nil
	case BackendPodman:
		return BackendPodman, nil
	default:
		return "", fmt.Errorf("unsupported container runtime %q: must be docker or podman", binary)
	}
}

func newConfig(backend Backend, environment Environment, explicitBinary string) (Config, error) {
	if err := validatePair(backend, environment); err != nil {
		return Config{}, err
	}
	binary := explicitBinary
	if binary == "" {
		binary = string(backend)
	}
	if got := Backend(filepath.Base(binary)); got != backend {
		return Config{}, fmt.Errorf("runtime backend %s requires the %s binary, got %s", backend, backend, binary)
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return Config{}, fmt.Errorf("container runtime %s not found: %w", binary, err)
	}
	exportMode, err := resolveExportMode(backend, environment)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Backend:          backend,
		Environment:      environment,
		Binary:           path,
		Socket:           resolveSocket(backend, environment),
		ExportMode:       exportMode,
		Platform:         strings.TrimSpace(os.Getenv("BKS_PLATFORM")),
		Network:          strings.TrimSpace(os.Getenv("BKS_NETWORK")),
		ExtraMounts:      strings.Fields(os.Getenv("BKS_EXTRA_MOUNTS")),
		RunImageArchive:  strings.TrimSpace(os.Getenv("BKS_RUN_IMAGE_ARCHIVE_FILE")),
		BuildpackAliases: buildpackAliases(),
	}, nil
}

func validatePair(backend Backend, environment Environment) error {
	switch environment {
	case EnvironmentHost:
		return nil
	case EnvironmentPind:
		if backend != BackendPodman {
			return fmt.Errorf("pind requires the podman backend, got %s", backend)
		}
	case EnvironmentDind:
		if backend != BackendDocker {
			return fmt.Errorf("dind requires the docker backend, got %s", backend)
		}
	default:
		return fmt.Errorf("unsupported environment %q", environment)
	}
	return nil
}

func resolveExportMode(backend Backend, environment Environment) (ExportMode, error) {
	if raw := strings.TrimSpace(os.Getenv("BKS_EXPORT_MODE")); raw != "" {
		switch ExportMode(raw) {
		case ExportLayout:
			if environment == EnvironmentDind {
				return "", fmt.Errorf("dind requires BKS_EXPORT_MODE=daemon")
			}
			return ExportLayout, nil
		case ExportDaemon:
			if backend != BackendDocker {
				return "", fmt.Errorf("BKS_EXPORT_MODE=daemon requires the docker backend, got %s", backend)
			}
			return ExportDaemon, nil
		default:
			return "", fmt.Errorf("invalid BKS_EXPORT_MODE %q: must be layout or daemon", raw)
		}
	}
	if environment == EnvironmentDind {
		return ExportDaemon, nil
	}
	return ExportLayout, nil
}

func resolveSocket(backend Backend, environment Environment) string {
	if socket := strings.TrimSpace(os.Getenv("BKS_DOCKER_SOCK")); socket != "" {
		return socket
	}
	if environment == EnvironmentPind {
		return podmanRootfulSocket
	}
	if backend == BackendPodman {
		return podmanSocket()
	}
	return dockerSocket
}

// podmanSocket 优先返回存在的 socket；root 优先 rootful，非 root 优先 rootless。
func podmanSocket() string {
	rootful := podmanRootfulSocket
	rootless := filepath.Join("/run/user", fmt.Sprint(os.Getuid()), podmanSocketDir, podmanSocketName)
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		rootless = filepath.Join(runtimeDir, podmanSocketDir, podmanSocketName)
	}
	candidates := []string{rootless, rootful}
	if os.Geteuid() == 0 {
		candidates = []string{rootful, rootless}
	}
	for _, socket := range candidates {
		if _, err := os.Stat(socket); err == nil {
			return socket
		}
	}
	return candidates[0]
}

func buildpackAliases() string {
	if aliases := os.Getenv("BKS_BUILDPACK_ALIASES"); aliases != "" {
		return aliases
	}
	return defaultBuildpackAliases
}
