package executors

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/pkg/errors"

	"bksmartpacker/pkg/container"
	"bksmartpacker/pkg/log"
)

const (
	sidecarPollInterval = 200 * time.Millisecond
	runtimeVolumePrefix = "cnb-runtime-"
	sidecarRuntimeMount = "/cnb/runtime"
)

// sidecarSpec 是 sidecar 的结构化启动定义。
// 命令参数只在 args() 边界生成，业务代码不直接拼 []string。
type sidecarSpec struct {
	Name       string
	Image      string
	Network    string
	Mounts     []sidecarMount
	Env        []sidecarEnv
	Privileged bool
}

type sidecarMount struct {
	Type     string
	Source   string
	Target   string
	ReadOnly bool
}

type sidecarEnv struct {
	Name  container.EnvKey
	Value string
}

func (m sidecarMount) spec() string {
	if m.Type == "volume" {
		return volumeMountSpec(m.Source, m.Target)
	}
	return bindMountSpec(m.Source, m.Target, m.ReadOnly)
}

func (s sidecarSpec) args() []string {
	a := newArgBuilder("run", "-d", "--rm")
	a.option("--name", s.Name)
	if s.Privileged {
		a.flag("--privileged")
	}
	for _, mount := range s.Mounts {
		a.add("--mount", mount.spec())
	}
	for _, env := range s.Env {
		a.env(env.Name, env.Value)
	}
	if s.Network != "" {
		a.option("--network", s.Network)
	}
	a.add(s.Image)
	return a.args()
}

// sidecarSpec 只描述 sidecar 需要什么，不关心命令如何拼。
func (e *ContainerExecutor) sidecarSpec(name string) sidecarSpec {
	mounts := []sidecarMount{
		{Type: "bind", Source: e.workspace.Host, Target: e.workspace.Container},
	}
	if e.runtime.Cache.Enabled() {
		mounts = append(mounts, sidecarMount{
			Type:   "bind",
			Source: e.runtime.Cache.HostPath,
			Target: e.runtime.Cache.ContainerPath,
		})
	}
	mounts = append(mounts, sidecarMount{
		Type:   "volume",
		Source: runtimeVolumeName(e.runtime.Inner.Backend),
		Target: sidecarRuntimeMount,
	})
	return sidecarSpec{
		Name:       name,
		Image:      e.runtime.RuntimeImage,
		Network:    e.runtime.Network,
		Mounts:     mounts,
		Env:        []sidecarEnv{{Name: container.EnvRuntimeCache, Value: sidecarRuntimeMount}},
		Privileged: true,
	}
}

func (e *ContainerExecutor) startSidecar(ctx context.Context) error {
	if e.runtime.RuntimeImage == "" {
		return fmt.Errorf("managed driver requires CNB_RUNTIME_IMAGE when no runtime socket is provided")
	}
	name := buildContainerName(e.plan.AppCode, "runtime")
	if err := e.ensureRuntimeVolume(ctx); err != nil {
		return err
	}
	args := e.sidecarSpec(name).args()
	log.Info("start runtime sidecar", "args", args)
	cmd := exec.CommandContext(ctx, e.runtime.Outer.Binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to start runtime sidecar")
	}
	e.sidecarName = name
	e.sidecarStarted = true
	if err := e.waitSocket(ctx); err != nil {
		e.sidecarLogs(ctx, name)
		e.stopSidecar()
		return err
	}
	return nil
}

func runtimeVolumeName(backend container.Backend) string {
	return runtimeVolumePrefix + string(backend)
}

func (e *ContainerExecutor) ensureRuntimeVolume(ctx context.Context) error {
	name := runtimeVolumeName(e.runtime.Inner.Backend)
	inspect := exec.CommandContext(ctx, e.runtime.Outer.Binary, "volume", "inspect", name)
	inspect.Stdout = io.Discard
	inspect.Stderr = io.Discard
	if inspect.Run() == nil {
		return nil
	}
	create := exec.CommandContext(ctx, e.runtime.Outer.Binary, "volume", "create", name)
	create.Stdout = io.Discard
	create.Stderr = os.Stderr
	if err := create.Run(); err != nil {
		return errors.Wrapf(err, "failed to create runtime volume %s", name)
	}
	return nil
}

func (e *ContainerExecutor) waitSocket(ctx context.Context) error {
	timeout := e.runtime.SidecarTimeout
	if timeout <= 0 {
		timeout = container.DefaultSidecarTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(sidecarPollInterval)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(e.runtime.Inner.Socket); err == nil && e.runtimeHealthy(ctx) {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "waiting for managed runtime socket")
		case <-timer.C:
			return fmt.Errorf("managed runtime socket %s not ready after %s", e.runtime.Inner.Socket, timeout)
		case <-ticker.C:
		}
	}
}

func (e *ContainerExecutor) runtimeHealthy(parent context.Context) bool {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	cli := newRuntimeCLI(e.runtime.Inner)
	args := append(cli.connectionArgs(), "info")
	cmd := exec.CommandContext(ctx, cli.binary, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func (e *ContainerExecutor) sidecarLogs(_ context.Context, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.runtime.Outer.Binary, "logs", "--tail", "50", name)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func (e *ContainerExecutor) stopSidecar() {
	if !e.sidecarStarted || e.sidecarName == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), containerCleanupTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.runtime.Outer.Binary, "rm", "-f", e.sidecarName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	e.sidecarStarted = false
}
