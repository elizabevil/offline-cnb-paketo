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

// RuntimeConfig 是一个具体运行时端点的配置。
type RuntimeConfig struct {
	Backend Backend
	Binary  string
	Socket  string
}

// IsPodman 返回该运行时端点是否使用 Podman。
func (r RuntimeConfig) IsPodman() bool { return r.Backend == BackendPodman }

// SocketURL 返回该运行时端点的 unix URL。
func (r RuntimeConfig) SocketURL() string { return UnixSocketURL(r.Socket) }

// Config 是执行器需要的全部运行时配置。
// Outer 负责启动 builder/sidecar；Inner 负责 builder 内构建。
// 环境变量只在本包解析，executors 不再读取 BKS_*。
type Config struct {
	Driver           Driver
	Outer            RuntimeConfig
	Inner            RuntimeConfig
	Environment      Environment
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
	BuilderUser      string
	BuilderUserNS    string
}

// envConfig 是环境变量的唯一读取结果。
// 解析函数只接收该结构，不再各自调用 os.Getenv。
type envConfig struct {
	Runtime           string
	Driver            string
	ContainerCLI      string
	InnerContainerCLI string
	DockerSock        string
	ExportMode        string
	Platform          string
	Network           string
	ExtraMounts       string
	RunImageArchive   string
	CacheDir          string
	Workspace         string
	RuntimeImage      string
	SidecarTimeout    string
	BuilderUser       string
	BuilderUserNS     string
	BuildpackAliases  string
	XDGRuntimeDir     string
}

func readEnv(getenv func(string) string) envConfig {
	return envConfig{
		Runtime:           firstNonEmpty(getenv(EnvBksRuntime), getenv(EnvRuntime)),
		Driver:            firstNonEmpty(getenv(EnvRuntimeDriver), getenv(EnvBksRuntimeDriver)),
		ContainerCLI:      getenv(EnvContainerCLI),
		InnerContainerCLI: getenv(EnvInnerContainerCLI),
		DockerSock:        getenv(EnvDockerSock),
		ExportMode:        getenv(EnvExportMode),
		Platform:          getenv(EnvPlatform),
		Network:           getenv(EnvNetwork),
		ExtraMounts:       getenv(EnvExtraMounts),
		RunImageArchive:   getenv(EnvBksRunImageArchiveFile),
		CacheDir:          firstNonEmpty(getenv(EnvCacheDir), getenv(EnvBksCacheDir)),
		Workspace:         firstNonEmpty(getenv(EnvWorkspace), getenv(EnvBksWorkspace)),
		RuntimeImage:      firstNonEmpty(getenv(EnvRuntimeImage), getenv(EnvBksRuntimeImage)),
		SidecarTimeout:    firstNonEmpty(getenv(EnvSidecarTimeout), getenv(EnvBksSidecarTimeout)),
		BuilderUser:       getenv(EnvBuilderUser),
		BuilderUserNS:     getenv(EnvBuilderUserNS),
		BuildpackAliases:  getenv(EnvBuildpackAliases),
		XDGRuntimeDir:     getenv(EnvXDGRuntimeDir),
	}
}

// DockerSocketContainerPath 是 builder 容器内固定的 Docker 兼容 socket 路径。
const DockerSocketContainerPath = dockerSocket

// DockerHostURL 是 builder 容器内访问运行时 socket 的地址。
var DockerHostURL = UnixSocketURL(DockerSocketContainerPath)

// IsPodman 返回是否使用 Podman 后端。
func (c Config) IsPodman() bool { return c.Inner.IsPodman() }

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
	outerBackend, err := resolveOuterBackend(selection, strings.TrimSpace(env.ContainerCLI), driver)
	if err != nil {
		return Config{}, err
	}
	innerBackend := selection.Backend
	if innerBackend == "" {
		innerBackend = outerBackend
	}
	return newConfig(outerBackend, innerBackend, selection.Environment, driver, env)
}

// runtimeSelection 是 RUNTIME 解析出的环境与内层后端组合。
// Backend 为空表示 host 模式需要自动探测；pind/dind 表示内层后端。
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

func resolveOuterBackend(selection runtimeSelection, explicitBinary string, driver Driver) (Backend, error) {
	if explicitBinary != "" {
		return backendFromBinary(explicitBinary)
	}
	if driver != DriverManaged && selection.Backend != "" {
		return selection.Backend, nil
	}
	for _, candidate := range []Backend{BackendPodman, BackendDocker} {
		if _, err := exec.LookPath(string(candidate)); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("failed to find podman or docker; set BKS_CONTAINER_CLI")
}

func resolveBackend(selection runtimeSelection, explicitBinary string) (Backend, error) {
	// 兼容测试和旧调用：host 模式解析外层后端。
	return resolveOuterBackend(selection, explicitBinary, DriverHost)
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

func newConfig(outerBackend, innerBackend Backend, environment Environment, driver Driver, env envConfig) (Config, error) {
	if err := validatePair(innerBackend, environment); err != nil {
		return Config{}, err
	}
	outerBinary, err := resolveBinary(outerBackend, env.ContainerCLI)
	if err != nil {
		return Config{}, err
	}
	innerBinary := outerBinary
	if driver == DriverManaged || outerBackend != innerBackend {
		if explicit := strings.TrimSpace(env.InnerContainerCLI); explicit != "" {
			innerBinary, err = resolveBinary(innerBackend, explicit)
			if err != nil {
				return Config{}, err
			}
		} else if outerBackend != innerBackend {
			innerBinary, err = resolveBinary(innerBackend, "")
			if err != nil {
				return Config{}, err
			}
		}
	}
	exportMode, err := resolveExportMode(innerBackend, environment, env.ExportMode)
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
	if driver == DriverManaged && workspace == "" {
		return Config{}, fmt.Errorf("BKS_RUNTIME_DRIVER=managed requires CNB_RUNTIME_WORKSPACE")
	}
	innerSocket := resolveInnerSocket(innerBackend, environment, driver, env.DockerSock, env.XDGRuntimeDir, workspace)
	outerSocket := ""
	if driver == DriverHost {
		outerSocket = innerSocket
	}
	return Config{
		Driver:           driver,
		Outer:            RuntimeConfig{Backend: outerBackend, Binary: outerBinary, Socket: outerSocket},
		Inner:            RuntimeConfig{Backend: innerBackend, Binary: innerBinary, Socket: innerSocket},
		Environment:      environment,
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
		BuilderUser:      strings.TrimSpace(env.BuilderUser),
		BuilderUserNS:    strings.TrimSpace(env.BuilderUserNS),
	}, nil
}

func resolveBinary(backend Backend, explicit string) (string, error) {
	binary := explicit
	if binary == "" {
		binary = string(backend)
	}
	if got := Backend(filepath.Base(binary)); got != backend {
		return "", fmt.Errorf("runtime backend %s requires the %s binary, got %s", backend, backend, binary)
	}
	binaryPath, err := exec.LookPath(binary)
	if err != nil {
		return "", fmt.Errorf("container runtime %s not found: %w", binary, err)
	}
	return binaryPath, nil
}

func resolveInnerSocket(backend Backend, environment Environment, driver Driver, dockerSock, xdgRuntimeDir, workspace string) string {
	if socket := strings.TrimSpace(dockerSock); socket != "" {
		return socket
	}
	if driver == DriverManaged {
		if backend == BackendDocker {
			return filepath.Join(workspace, workspaceRunDir, dockerSocketName)
		}
		return filepath.Join(workspace, workspaceRunDir, podmanSocketName)
	}
	return resolveSocket(backend, environment, "", xdgRuntimeDir)
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
