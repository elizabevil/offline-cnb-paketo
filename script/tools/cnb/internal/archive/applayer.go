package archive

// 本文件实现应用层 docker-archive 的构造（只含应用层）：
// 不依赖 lifecycle exporter，也不依赖容器 daemon。
import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 默认镜像配置。
const (
	DefaultOS   = "linux"
	DefaultArch = "amd64"

	layerTarName = "layer.tar"
)

type manifestEntry struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

type imageConfig struct {
	Architecture string    `json:"architecture"`
	OS           string    `json:"os"`
	Config       runConfig `json:"config"`
	RootFS       rootFS    `json:"rootfs"`
	History      []history `json:"history"`
}

type runConfig struct {
	Env        []string          `json:"Env,omitempty"`
	Labels     map[string]string `json:"Labels,omitempty"`
	WorkingDir string            `json:"WorkingDir,omitempty"`
}

type rootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

type history struct {
	Created    time.Time `json:"created"`
	CreatedBy  string    `json:"created_by"`
	EmptyLayer bool      `json:"empty_layer,omitempty"`
}

// 层内路径约定：应用与 launch 层内容都落在 workspace/ 下，
// 与 BK 的 procCommand（如 ./workspace/app）保持一致。
const workPrefix = "workspace"

// Build 把 launch 层（可选叠加应用源码）打成单层 docker-archive。
//
// includeSource=false 时只打包构建产物（launch 层），应用源码不进包。
func Build(appDir, layersDir, repoTag, arch, dest string, includeSource bool) error {
	if arch == "" {
		arch = DefaultArch
	}
	layerTar := dest + ".layer.tar"
	if err := writeLayerTar(appDir, layersDir, layerTar, includeSource); err != nil {
		return err
	}
	defer func() { _ = os.Remove(layerTar) }()

	diffID, _, err := fileDigest(layerTar)
	if err != nil {
		return err
	}
	cfg := imageConfig{
		Architecture: arch,
		OS:           DefaultOS,
		Config:       runConfig{WorkingDir: "/workspace"},
		RootFS:       rootFS{Type: "layers", DiffIDs: []string{"sha256:" + diffID}},
		History:      []history{{Created: time.Now().UTC(), CreatedBy: "cnb apparchive"}},
	}
	cfgData, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	cfgName := diffID + ".json"

	man := []manifestEntry{{
		Config:   cfgName,
		RepoTags: []string{repoTag},
		Layers:   []string{diffID + "/" + layerTarName},
	}}
	manData, err := json.Marshal(man)
	if err != nil {
		return err
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	tw := tar.NewWriter(out)
	defer func() { _ = tw.Close() }()

	if err := addFile(tw, cfgName, cfgData); err != nil {
		return err
	}
	if err := addFile(tw, diffID+"/"+layerTarName, mustRead(layerTar)); err != nil {
		return err
	}
	if err := addFile(tw, "manifest.json", manData); err != nil {
		return err
	}
	return nil
}

// writeLayerTar 把应用目录与 launch 层写成未压缩 tar。
func writeLayerTar(appDir, layersDir, dest string, includeSource bool) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	tw := tar.NewWriter(out)
	defer func() { _ = tw.Close() }()

	if includeSource {
		if err := writeTree(tw, appDir, workPrefix); err != nil {
			return err
		}
	}
	return writeLaunchLayers(tw, layersDir)
}

// writeLaunchLayers 把标记 launch=true 的层内容并入 workspace/。
func writeLaunchLayers(tw *tar.Writer, layersDir string) error {
	if layersDir == "" {
		return nil
	}
	entries, err := os.ReadDir(layersDir)
	if err != nil {
		return nil // 没有层目录时忽略
	}
	for _, bp := range entries {
		if !bp.IsDir() || bp.Name() == "cache" || bp.Name() == "oci" {
			continue
		}
		layers, err := os.ReadDir(filepath.Join(layersDir, bp.Name()))
		if err != nil {
			continue
		}
		for _, l := range layers {
			dir := filepath.Join(layersDir, bp.Name(), l.Name())
			if !isLaunchLayer(dir) {
				continue
			}
			if err := writeTree(tw, dir, workPrefix); err != nil {
				return err
			}
		}
	}
	return nil
}

// isLaunchLayer 判断层是否声明 launch=true。
func isLaunchLayer(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "launch.toml"))
	if err == nil && len(data) > 0 {
		return true
	}
	meta, err := os.ReadFile(dir + ".toml")
	if err != nil {
		return false
	}
	return strings.Contains(string(meta), "launch  = true") ||
		strings.Contains(string(meta), "launch = true")
}

// writeTree 把目录内容写入归档，路径加 prefix 前缀。
func writeTree(tw *tar.Writer, root, prefix string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(prefix, rel))
		hdr := &tar.Header{
			Name:    name,
			Mode:    fileMode,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}
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

// fileDigest 返回文件的十六进制 sha256 与大小。
func fileDigest(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// addFile 向归档写入一个文件。
func addFile(tw *tar.Writer, name string, data []byte) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: fileMode, Size: int64(len(data))}); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

// mustRead 读取文件内容（仅用于刚写完的临时文件）。
func mustRead(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}
