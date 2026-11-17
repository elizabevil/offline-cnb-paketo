package imagefmt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// imageConfigFile 是镜像 config JSON 的磁盘形态（兼容 docker/OCI 键名）。
type imageConfigFile struct {
	Architecture string           `json:"architecture"`
	OS           string           `json:"os"`
	ContainerCfg *containerConfig `json:"config,omitempty"`
	RootFS       rootFSFile       `json:"rootfs"`
}

type containerConfig struct {
	Entrypoint []string          `json:"Entrypoint,omitempty"`
	Env        []string          `json:"Env,omitempty"`
	WorkingDir string            `json:"WorkingDir,omitempty"`
	Labels     map[string]string `json:"Labels,omitempty"`
}

type rootFSFile struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

// readImageConfig 解析 config JSON 并填充 img。
func readImageConfig(path string, img *Image) error {
	var cf imageConfigFile
	if err := readJSON(path, &cf); err != nil {
		return err
	}
	img.OS, img.Arch = cf.OS, cf.Architecture
	img.Config.DiffIDs = cf.RootFS.DiffIDs
	if cf.ContainerCfg != nil {
		img.Config.Entrypoint = cf.ContainerCfg.Entrypoint
		img.Config.Env = cf.ContainerCfg.Env
		img.Config.WorkingDir = cf.ContainerCfg.WorkingDir
		img.Config.Labels = cf.ContainerCfg.Labels
	}
	return nil
}

// buildConfigFile 把 Image 转成磁盘 config 结构。
func buildConfigFile(img *Image) imageConfigFile {
	return imageConfigFile{
		Architecture: img.Arch,
		OS:           img.OS,
		ContainerCfg: &containerConfig{
			Entrypoint: img.Config.Entrypoint,
			Env:        img.Config.Env,
			WorkingDir: img.Config.WorkingDir,
			Labels:     img.Config.Labels,
		},
		RootFS: rootFSFile{Type: "layers", DiffIDs: img.Config.DiffIDs},
	}
}

// digestOfFile 返回文件内容的 sha256 digest。
func digestOfFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return DigestPrefix + hex.EncodeToString(sum[:])
}

// configBytes 返回写出用的 config JSON：优先原始字节。
func configBytes(img *Image) ([]byte, error) {
	if len(img.ConfigRaw) > 0 {
		return img.ConfigRaw, nil
	}
	return json.Marshal(buildConfigFile(img))
}

// writeImageConfig 写出 config JSON，返回文件名。
func writeImageConfig(img *Image, dir string) (string, error) {
	data, err := configBytes(img)
	if err != nil {
		return "", err
	}
	img.ConfigDigest = digestOfBytes(data)
	name := "config.json"
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return "", err
	}
	return name, nil
}

// writeBlobJSON 把对象写为 blob，返回 digest 与大小。
func writeBlobJSON(blobsDir string, v any) (string, int64, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", 0, err
	}
	return writeBlobData(blobsDir, data)
}

// digestOfBytes 返回字节内容的 sha256 digest。
func digestOfBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return DigestPrefix + hex.EncodeToString(sum[:])
}

// writeBlobData 写入 blob 并返回 sha256 digest 与大小。
func writeBlobData(blobsDir string, data []byte) (string, int64, error) {
	sum := sha256.Sum256(data)
	digest := DigestPrefix + hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(blobsDir, hex.EncodeToString(sum[:])), data, 0o644); err != nil {
		return "", 0, err
	}
	return digest, int64(len(data)), nil
}

// writeLayerBlob 复制层内容为 blob。
func writeLayerBlob(blobsDir string, l Layer, idx int) (string, int64, error) {
	data, err := os.ReadFile(l.Path)
	if err != nil {
		return "", 0, errs.Wrap(fmt.Sprintf("read layer %d", idx), err)
	}
	return writeBlobData(blobsDir, data)
}

// copyLayer 把层内容复制到 docker-archive 的 <digest>/layer.tar。
func copyLayer(l Layer, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	in, err := os.Open(l.Path)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(filepath.Join(dir, LayerTarName))
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}

// digestOf 生成占位的层目录名（无 digest 时使用）。
func digestOf(i int) string { return fmt.Sprintf("%064d", i+1) }

// fileSize 返回文件大小（失败为 0）。
func fileSize(p string) int64 {
	if fi, err := os.Stat(p); err == nil {
		return fi.Size()
	}
	return 0
}

// readJSON 读取并解析 JSON 文件。
func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return errs.Wrap("parse "+filepath.Base(path), err)
	}
	return nil
}

// writeJSON 写出 JSON 文件。
func writeJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
