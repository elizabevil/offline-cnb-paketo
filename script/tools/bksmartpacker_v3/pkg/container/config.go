package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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

// Driver 是运行时驱动方式。
type Driver string

const (
	// DriverHost 使用宿主机已有运行时。
	DriverHost Driver = "host"
	// DriverManaged 使用外部/managed 运行时端点。
	DriverManaged Driver = "managed"
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
	dockerSocketName    = "docker.sock"
	workspaceRunDir     = "run"
	// DefaultSidecarTimeout 是 managed sidecar socket 就绪的默认超时。
	DefaultSidecarTimeout = 60 * time.Second
)

// Config 是执行器需要的全部运行时配置。
// 环境变量只在本包解析，executors 不再读取 BKS_*。
type Config struct {
	Driver           Driver
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
	Cache            Cache
	Workspace        string
	RuntimeImage     string
	SidecarTimeout   time.Duration
}

// envConfig 是环境变量的唯一读取结果。
// 解析函数只接收该结构，不再各自调用 os.Getenv。
type envConfig struct {
	Runtime          string
	Driver           string
	ContainerCLI     string
	DockerSock       string
	ExportMode       string
	Platform         string
	Network          string
	ExtraMounts      string
	RunImageArchive  string
	CacheDir         string
	Workspace        string
	RuntimeImage     string
	SidecarTimeout   string
	BuildpackAliases string
	XDGRuntimeDir    string
}

func readEnv(getenv func(string) string) envConfig {
	return envConfig{
		Runtime:          firstNonEmpty(getenv(EnvBksRuntime), getenv(EnvRuntime)),
		Driver:           firstNonEmpty(getenv(EnvRuntimeDriver), getenv(EnvBksRuntimeDriver)),
		ContainerCLI:     getenv(EnvContainerCLI),
		DockerSock:       getenv(EnvDockerSock),
		ExportMode:       getenv(EnvExportMode),
		Platform:         getenv(EnvPlatform),
		Network:          getenv(EnvNetwork),
		ExtraMounts:      getenv(EnvExtraMounts),
		RunImageArchive:  getenv(EnvBksRunImageArchiveFile),
		CacheDir:         firstNonEmpty(getenv(EnvCacheDir), getenv(EnvBksCacheDir)),
		Workspace:        firstNonEmpty(getenv(EnvWorkspace), getenv(EnvBksWorkspace)),
		RuntimeImage:     firstNonEmpty(getenv(EnvRuntimeImage), getenv(EnvBksRuntimeImage)),
		SidecarTimeout:   firstNonEmpty(getenv(EnvSidecarTimeout), getenv(EnvBksSidecarTimeout)),
		BuildpackAliases: getenv(EnvBuildpackAliases),
		XDGRuntimeDir:    getenv(EnvXDGRuntimeDir),
	}
}

// DockerSocketContainerPath 是 builder 容器内固定的 Docker 兼容 socket 路径。
const DockerSocketContainerPath = dockerSocket

// DockerHostURL 是 builder 容器内访问运行时 socket 的地址。
const DockerHostURL = "unix://" + DockerSocketContainerPath

// IsPodman 返回是否使用 Podman 后端。
func (c Config) IsPodman() bool { return c.Backend == BackendPodman }

// ResolveFromEnv 从进程环境解析运行时配置。
func ResolveFromEnv() (Config, error) {
	return Parse(os.Getenv)
}

// Parse 从注入的 getenv 解析运行时配置，便于测试且保持唯一环境入口。
func Parse(getenv func(string) string) (Config, error) {
	return resolve(readEnv(getenv))
}

func resolve(env envConfig) (Config, error) {
	selection, err := parseRuntimeSelection(env.Runtime)
	if err != nil {
		return Config{}, err
	}
	driver, err := resolveDriver(env.Driver)
	if err != nil {
		return Config{}, err
	}
	backend, err := resolveBackend(selection, strings.TrimSpace(env.ContainerCLI))
	if err != nil {
		return Config{}, err
	}
	return newConfig(backend, selection.Environment, strings.TrimSpace(env.ContainerCLI), driver, env)
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

func newConfig(backend Backend, environment Environment, explicitBinary string, driver Driver, env envConfig) (Config, error) {
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
	binaryPath, err := exec.LookPath(binary)
	if err != nil {
		return Config{}, fmt.Errorf("container runtime %s not found: %w", binary, err)
	}
	exportMode, err := resolveExportMode(backend, environment, env.ExportMode)
	if err != nil {
		return Config{}, err
	}
	cache, err := resolveCache(env.CacheDir)
	if err != nil {
		return Config{}, err
	}
	sidecarTimeout, err := resolveSidecarTimeout(env.SidecarTimeout)
	if err != nil {
		return Config{}, err
	}
	workspace := strings.TrimSpace(env.Workspace)
	socket := resolveSocket(backend, environment, env.DockerSock, env.XDGRuntimeDir)
	if driver == DriverManaged {
		if workspace == "" {
			return Config{}, fmt.Errorf("BKS_RUNTIME_DRIVER=managed requires CNB_RUNTIME_WORKSPACE")
		}
		if strings.TrimSpace(env.DockerSock) == "" {
			socket = filepath.Join(workspace, workspaceRunDir, podmanSocketName)
			if backend == BackendDocker {
				socket = filepath.Join(workspace, workspaceRunDir, dockerSocketName)
			}
		}
	}
	return Config{
		Driver:           driver,
		Backend:          backend,
		Environment:      environment,
		Binary:           binaryPath,
		Socket:           socket,
		ExportMode:       exportMode,
		Platform:         strings.TrimSpace(env.Platform),
		Network:          strings.TrimSpace(env.Network),
		ExtraMounts:      strings.Fields(env.ExtraMounts),
		RunImageArchive:  strings.TrimSpace(env.RunImageArchive),
		BuildpackAliases: buildpackAliases(env.BuildpackAliases),
		Cache:            cache,
		Workspace:        workspace,
		RuntimeImage:     strings.TrimSpace(env.RuntimeImage),
		SidecarTimeout:   sidecarTimeout,
	}, nil
}

func resolveSidecarTimeout(value string) (time.Duration, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return DefaultSidecarTimeout, nil
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid sidecar timeout %q: %w", raw, err)
	}
	if timeout <= 0 {
		return 0, fmt.Errorf("sidecar timeout must be positive, got %s", timeout)
	}
	return timeout, nil
}

func resolveDriver(value string) (Driver, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return DriverHost, nil
	}
	switch d := Driver(strings.ToLower(raw)); d {
	case DriverHost, DriverManaged:
		return d, nil
	default:
		return "", fmt.Errorf("unsupported runtime driver %q: must be host or managed", raw)
	}
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

func resolveExportMode(backend Backend, environment Environment, value string) (ExportMode, error) {
	if raw := strings.TrimSpace(value); raw != "" {
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

func resolveSocket(backend Backend, environment Environment, dockerSock, xdgRuntimeDir string) string {
	if socket := strings.TrimSpace(dockerSock); socket != "" {
		return socket
	}
	if environment == EnvironmentPind {
		return podmanRootfulSocket
	}
	if backend == BackendPodman {
		return podmanSocket(xdgRuntimeDir)
	}
	return dockerSocket
}

// podmanSocket 优先返回存在的 socket；root 优先 rootful，非 root 优先 rootless。
func podmanSocket(xdgRuntimeDir string) string {
	rootful := podmanRootfulSocket
	rootless := filepath.Join("/run/user", fmt.Sprint(os.Getuid()), podmanSocketDir, podmanSocketName)
	if runtimeDir := xdgRuntimeDir; runtimeDir != "" {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func buildpackAliases(value string) string {
	if aliases := value; aliases != "" {
		return aliases
	}
	return defaultBuildpackAliases
}
