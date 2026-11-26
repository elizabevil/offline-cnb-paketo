package executors

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const (
	// WorkspaceContainerRoot 是 builder 容器内固定的编排工作区。
	WorkspaceContainerRoot = "/cnb/work"
	// WorkspacePlatformMount 是 lifecycle 固定的 platform 目录。
	WorkspacePlatformMount = "/platform"

	sourceDirName   = "source"
	platformDirName = "platform"
	layersDirName   = "layers"
	runDirName      = "run"
	outDirName      = "out"
	tmpDirName      = "tmp"

	sourceArchiveName = "source.tgz"
	runImageName      = "run-image.tar"
)

// Workspace 是单次构建的宿主机工作区。
type Workspace struct {
	Host      string
	Container string
	temporary bool
}

// NewWorkspace 创建或复用工作区；hostPath 为空时创建临时目录。
func NewWorkspace(hostPath string) (*Workspace, error) {
	temporary := false
	if strings.TrimSpace(hostPath) == "" {
		dir, err := os.MkdirTemp("", "cnb-work")
		if err != nil {
			return nil, errors.Wrap(err, "failed to create workspace")
		}
		hostPath = dir
		temporary = true
	}
	abs, err := filepath.Abs(hostPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve workspace")
	}
	if err := os.MkdirAll(abs, 0o775); err != nil {
		return nil, errors.Wrap(err, "failed to create workspace dir")
	}
	w := &Workspace{Host: abs, Container: WorkspaceContainerRoot, temporary: temporary}
	if err := w.reset(); err != nil {
		_ = w.Close()
		return nil, err
	}
	return w, nil
}

// Close 清理临时工作区；用户指定工作区只清空子目录。
func (w *Workspace) Close() error {
	if w == nil {
		return nil
	}
	if w.temporary {
		return os.RemoveAll(w.Host)
	}
	return w.reset()
}

func (w *Workspace) reset() error {
	for _, dir := range []string{
		w.SourceDir(), w.PlatformDir(), w.LayersDir(), w.RunDir(), w.OutDir(), w.TmpDir(),
	} {
		if err := os.RemoveAll(dir); err != nil {
			return errors.Wrapf(err, "failed to reset %s", dir)
		}
		if err := os.MkdirAll(dir, 0o775); err != nil {
			return errors.Wrapf(err, "failed to create %s", dir)
		}
	}
	return nil
}

// SourceDir 是宿主机源码目录。
func (w *Workspace) SourceDir() string { return filepath.Join(w.Host, sourceDirName) }

// SourcePath 是宿主机源码 tar。
func (w *Workspace) SourcePath() string { return filepath.Join(w.SourceDir(), sourceArchiveName) }

// SourceContainerPath 是 builder 容器内源码 tar。
func (w *Workspace) SourceContainerPath() string {
	return pathJoin(w.Container, sourceDirName, sourceArchiveName)
}

// PlatformDir 是宿主机 platform 目录。
func (w *Workspace) PlatformDir() string { return filepath.Join(w.Host, platformDirName) }

// PlatformContainerPath 是 lifecycle 固定 platform 目录。
func (w *Workspace) PlatformContainerPath() string { return WorkspacePlatformMount }

// LayersDir 是宿主机 layer 输出目录。
func (w *Workspace) LayersDir() string { return filepath.Join(w.Host, layersDirName) }

// LayerContainerPath 是指定 layer archive 的容器路径。
func (w *Workspace) LayerContainerPath(name string) string {
	return pathJoin(w.Container, layersDirName, name)
}

// RunDir 是宿主机运行中间目录。
func (w *Workspace) RunDir() string { return filepath.Join(w.Host, runDirName) }

// RunImagePath 是宿主机 run image archive。
func (w *Workspace) RunImagePath() string { return filepath.Join(w.RunDir(), runImageName) }

// RunImageContainerPath 是容器内 run image archive。
func (w *Workspace) RunImageContainerPath() string {
	return pathJoin(w.Container, runDirName, runImageName)
}

// OutDir 是 s-mart 组装目录。
func (w *Workspace) OutDir() string { return filepath.Join(w.Host, outDirName) }

// TmpDir 是宿主机临时目录。
func (w *Workspace) TmpDir() string { return filepath.Join(w.Host, tmpDirName) }

// MountArgs 返回根挂载参数。
func (w *Workspace) MountArgs() []string {
	return []string{"--mount", fmt.Sprintf("type=bind,source=%s,target=%s", w.Host, w.Container)}
}

// PlatformMountArgs 返回 lifecycle platform 挂载参数。
func (w *Workspace) PlatformMountArgs() []string {
	return []string{"--mount", fmt.Sprintf("type=bind,source=%s,target=%s,readonly", w.PlatformDir(), w.PlatformContainerPath())}
}

func pathJoin(parts ...string) string {
	return strings.Join(parts, "/")
}
