package executors

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/pkg/errors"

	"bksmartpacker/pkg/log"
)

const (
	sidecarReadyTimeout = 60 * time.Second
	sidecarPollInterval = 200 * time.Millisecond
)

// sidecarArgs 构造 managed runtime sidecar 的启动参数。
// 镜像约定：entrypoint 在 /cnb/work/run/<backend>.sock 提供运行时服务。
func (e *ContainerExecutor) sidecarArgs(name string) []string {
	a := newArgBuilder("run", "-d", "--rm")
	a.option("--name", name)
	a.flag("--privileged")
	a.bindMount(e.workspace.Host, e.workspace.Container, false)
	if e.runtime.Cache.Enabled() {
		a.bindMount(e.runtime.Cache.HostPath, e.runtime.Cache.ContainerPath, false)
	}
	if e.runtime.Network != "" {
		a.option("--network", e.runtime.Network)
	}
	a.add(e.runtime.RuntimeImage)
	return a.args()
}

func (e *ContainerExecutor) startSidecar(ctx context.Context) error {
	if e.runtime.RuntimeImage == "" {
		return fmt.Errorf("managed driver requires CNB_RUNTIME_IMAGE when no runtime socket is provided")
	}
	name := buildContainerName(e.plan.AppCode, "runtime")
	args := e.sidecarArgs(name)
	log.Info("start runtime sidecar", "args", args)
	cmd := exec.CommandContext(ctx, e.runtime.Binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to start runtime sidecar")
	}
	e.sidecarName = name
	e.sidecarStarted = true
	if err := e.waitSocket(ctx); err != nil {
		e.stopSidecar()
		return err
	}
	return nil
}

func (e *ContainerExecutor) waitSocket(ctx context.Context) error {
	timer := time.NewTimer(sidecarReadyTimeout)
	defer timer.Stop()
	ticker := time.NewTicker(sidecarPollInterval)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(e.runtime.Socket); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "waiting for managed runtime socket")
		case <-timer.C:
			return fmt.Errorf("managed runtime socket %s not ready after %s", e.runtime.Socket, sidecarReadyTimeout)
		case <-ticker.C:
		}
	}
}

func (e *ContainerExecutor) stopSidecar() {
	if !e.sidecarStarted || e.sidecarName == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), containerCleanupTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.runtime.Binary, "rm", "-f", e.sidecarName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	e.sidecarStarted = false
}
