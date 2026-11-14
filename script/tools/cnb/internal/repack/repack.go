package repack

// repack — 把 bksmartpacker 产出的 docker-save 风格 web.tgz 规范化为 OCI layout。
//
//	cnb repack <SmartPackage 目录>      规范化目录下的 web.tgz
//	cnb repack <package.tar.gz>        解包 → 规范化 web.tgz → 重打包覆盖
//
// 幂等：web.tgz 内已有 oci-layout + index.json 时原样返回，不重写。
//
// 唯一判据是 repack_test.go 的 golden 常量（不依赖任何历史实现）：
//   - 四个 JSON 载荷字节级一致
//   - OCI manifest blob 与 index.json 中引用的 digest 自洽
//   - 层 / config blob 内容原样搬运
//   - 二次执行幂等

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"io"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
)

// ── OCI / docker 载荷结构（字段顺序即 JSON 键顺序，不能随意调整）─────────────

type blobDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type ociManifest struct {
	SchemaVersion int              `json:"schemaVersion"`
	MediaType     string           `json:"mediaType"`
	Config        blobDescriptor   `json:"config"`
	Layers        []blobDescriptor `json:"layers"`
}

type indexAnnotations struct {
	ContainerdName string `json:"io.containerd.image.name"`
	RefName        string `json:"org.opencontainers.image.ref.name"`
}

type indexEntry struct {
	MediaType   string           `json:"mediaType"`
	Digest      string           `json:"digest"`
	Size        int64            `json:"size"`
	Annotations indexAnnotations `json:"annotations"`
}

type ociIndex struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Manifests     []indexEntry `json:"manifests"`
}

type ociLayout struct {
	ImageLayoutVersion string `json:"imageLayoutVersion"`
}

type layerSource struct {
	digest string
	desc   blobDescriptor
}

// layerSources 保持插入顺序（Go map 会随机化键顺序）。
type layerSources struct{ items []layerSource }

func (l layerSources) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteByte('{')
	for i, it := range l.items {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := marshalCompact(it.digest)
		if err != nil {
			return nil, err
		}
		val, err := marshalCompact(it.desc)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(val)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

type legacyManifestEntry struct {
	Config       string       `json:"Config"`
	RepoTags     []string     `json:"RepoTags"`
	Layers       []string     `json:"Layers"`
	LayerSources layerSources `json:"LayerSources"`
}

type dockerManifestEntry struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

// ── 入口 ────────────────────────────────────────────────────────────────────

// Run 规范化目标（目录或 *.tar.gz）。
func Run(target string) error {
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("目标不存在: %s", target)
	}
	if info.IsDir() {
		return normalizePackageDir(abs)
	}
	if info.Mode().IsRegular() && strings.HasSuffix(abs, ".tar.gz") {
		return normalizePackageTarball(abs)
	}
	return fmt.Errorf("不支持的目标: %s（只接受目录或 *.tar.gz）", target)
}

func normalizePackageDir(dir string) error {
	webTar := filepath.Join(dir, "web.tgz")
	if _, err := os.Stat(webTar); err != nil {
		return fmt.Errorf("未在 %s 找到 web.tgz", dir)
	}
	return normalizeWebTar(webTar)
}

func normalizePackageTarball(pkgTar string) error {
	tempRoot, err := os.MkdirTemp("", "cnb-repack-pkg-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tempRoot) }() // 临时目录清理失败不影响结果

	pkgDir := filepath.Join(tempRoot, "pkg")
	if err := os.MkdirAll(pkgDir, fsutil.DirMode); err != nil {
		return err
	}
	if err := extractTar(pkgTar, pkgDir); err != nil {
		return fmt.Errorf("解包 %s: %w", filepath.Base(pkgTar), err)
	}
	if err := normalizePackageDir(pkgDir); err != nil {
		return err
	}

	rebuilt := filepath.Join(tempRoot, "rebuilt.tar.gz")
	if err := packDirToTarGz(pkgDir, rebuilt); err != nil {
		return err
	}

	// 先写临时文件再 rename，避免中途失败留下半个包
	tmp := pkgTar + ".tmp"
	if err := copyFile(rebuilt, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, pkgTar)
}

// ── web.tgz 规范化 ──────────────────────────────────────────────────────────

func normalizeWebTar(webTar string) error {
	tempRoot, err := os.MkdirTemp("", "cnb-repack-web-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tempRoot) }() // 临时目录清理失败不影响结果

	legacyDir := filepath.Join(tempRoot, "legacy")
	if err := os.MkdirAll(legacyDir, fsutil.DirMode); err != nil {
		return err
	}
	if err := extractTar(webTar, legacyDir); err != nil {
		return fmt.Errorf("解包 web.tgz: %w", err)
	}

	// 已是 OCI layout → 幂等返回
	if fileExists(filepath.Join(legacyDir, "oci-layout")) && fileExists(filepath.Join(legacyDir, "index.json")) {
		return nil
	}

	manifestBytes, err := os.ReadFile(filepath.Join(legacyDir, "manifest.json"))
	if err != nil {
		return fmt.Errorf("缺少 manifest.json: %w", err)
	}
	var legacy []dockerManifestEntry
	if err := json.Unmarshal(manifestBytes, &legacy); err != nil {
		return fmt.Errorf("解析 manifest.json: %w", err)
	}
	if len(legacy) == 0 || len(legacy[0].RepoTags) == 0 {
		return fmt.Errorf("manifest.json 缺少 RepoTags")
	}

	entry := legacy[0]
	repoTag := entry.RepoTags[0]
	repoName, repoVersion := parseRepoTag(repoTag)

	configName := filepath.Base(entry.Config)
	configDigest, _, _ := strings.Cut(configName, ".")
	configSrc := filepath.Join(legacyDir, configName)

	outDir := filepath.Join(tempRoot, "oci")
	blobsDir := filepath.Join(outDir, "blobs", "sha256")
	if err := os.MkdirAll(blobsDir, fsutil.DirMode); err != nil {
		return err
	}

	if err := copyFile(configSrc, filepath.Join(blobsDir, configDigest)); err != nil {
		return fmt.Errorf("复制 config: %w", err)
	}
	configSize, err := fileSizeOf(configSrc)
	if err != nil {
		return err
	}
	configDesc := blobDescriptor{
		MediaType: "application/vnd.oci.image.config.v1+json",
		Digest:    "sha256:" + configDigest,
		Size:      configSize,
	}

	var layers []blobDescriptor
	var legacyLayers []string
	var sources layerSources
	lastLayerDigest := ""

	for _, layerRef := range entry.Layers {
		layerName := filepath.Base(layerRef)
		layerDigest, _, _ := strings.Cut(layerName, ".")
		lastLayerDigest = layerDigest

		layerSrc := filepath.Join(legacyDir, layerName)
		if err := copyFile(layerSrc, filepath.Join(blobsDir, layerDigest)); err != nil {
			return fmt.Errorf("复制层 %s: %w", layerName, err)
		}
		size, err := fileSizeOf(layerSrc)
		if err != nil {
			return err
		}
		desc := blobDescriptor{
			MediaType: "application/vnd.oci.image.layer.v1.tar",
			Digest:    "sha256:" + layerDigest,
			Size:      size,
		}
		layers = append(layers, desc)
		legacyLayers = append(legacyLayers, "blobs/sha256/"+layerDigest)
		sources.items = append(sources.items, layerSource{digest: "sha256:" + layerDigest, desc: desc})
	}

	manifest := ociManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config:        configDesc,
		Layers:        layers,
	}
	manifestJSON, err := marshalCompact(manifest)
	if err != nil {
		return err
	}
	manifestDigest := sha256Hex(manifestJSON)
	if err := os.WriteFile(filepath.Join(blobsDir, manifestDigest), manifestJSON, fsutil.FileMode); err != nil {
		return err
	}

	index := ociIndex{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.index.v1+json",
		Manifests: []indexEntry{{
			MediaType: "application/vnd.oci.image.manifest.v1+json",
			Digest:    "sha256:" + manifestDigest,
			Size:      int64(len(manifestJSON)),
			Annotations: indexAnnotations{
				ContainerdName: repoTag,
				RefName:        repoVersion,
			},
		}},
	}
	if err := writeJSONFile(filepath.Join(outDir, "index.json"), index); err != nil {
		return err
	}

	legacyOut := []legacyManifestEntry{{
		Config:       "blobs/sha256/" + configDigest,
		RepoTags:     []string{strings.TrimPrefix(repoTag, "docker.io/")},
		Layers:       legacyLayers,
		LayerSources: sources,
	}}
	if err := writeJSONFile(filepath.Join(outDir, "manifest.json"), legacyOut); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(outDir, "oci-layout"), ociLayout{ImageLayoutVersion: "1.0.0"}); err != nil {
		return err
	}
	repos := map[string]map[string]string{repoName: {repoVersion: lastLayerDigest}}
	if err := writeJSONFile(filepath.Join(outDir, "repositories"), repos); err != nil {
		return err
	}

	// 用规范化后的目录整体替换 web.tgz（plain tar，与 Python 版一致）
	return packDirToTar(outDir, webTar)
}

// parseRepoTag 拆分 repo:tag，忽略 docker.io/ 前缀。
func parseRepoTag(repoTag string) (repo, tag string) {
	normalized := strings.TrimPrefix(repoTag, "docker.io/")
	slash := strings.LastIndex(normalized, "/")
	lastSegment := normalized[slash+1:]
	if idx := strings.LastIndex(lastSegment, ":"); idx >= 0 {
		repo = normalized[:slash+1+idx]
		tag = lastSegment[idx+1:]
		return repo, tag
	}
	return normalized, "latest"
}

// ── tar / gzip 基础设施 ─────────────────────────────────────────────────────

// extractTar 解包 tar / tar.gz 到 dest（按文件头类型还原目录、链接、权限、mtime）。
func extractTar(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	reader := bufio.NewReader(f)
	var src io.Reader = reader
	if magic, _ := reader.Peek(2); len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return err
		}
		defer func() { _ = gz.Close() }()
		src = gz
	}

	tr := tar.NewReader(src)
	cleanDest := filepath.Clean(dest) + string(os.PathSeparator)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, filepath.FromSlash(hdr.Name))
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) {
			return fmt.Errorf("非法路径（疑似路径穿越）: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), fsutil.DirMode); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close() // 写入已完成，关闭错误不影响产物
				return err
			}
			_ = out.Close() // 写入已完成，关闭错误不影响产物
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), fsutil.DirMode); err != nil {
				return err
			}
			// 覆盖已存在文件时忽略删除失败（随后会重建）
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			if err := os.MkdirAll(filepath.Dir(target), fsutil.DirMode); err != nil {
				return err
			}
			// 覆盖已存在文件时忽略删除失败（随后会重建）
			_ = os.Remove(target)
			if err := os.Link(filepath.Join(dest, filepath.FromSlash(hdr.Linkname)), target); err != nil {
				return err
			}
		case tar.TypeXGlobalHeader:
			continue
		default:
			continue
		}

		if hdr.Typeflag != tar.TypeSymlink && !hdr.ModTime.IsZero() {
			// 保留时间戳失败不影响内容正确性
			_ = os.Chtimes(target, hdr.ModTime, hdr.ModTime)
		}
	}
	return nil
}

// packDirToTar 把目录打成 plain tar：顶层条目按名排序，目录递归展开（与 Python 版一致）。
func packDirToTar(srcDir, destTar string) error {
	tmp := destTar + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(f)

	entries, err := os.ReadDir(srcDir) // os.ReadDir 已按文件名排序
	if err != nil {
		_ = f.Close() // 写入已完成，关闭错误不影响产物
		return err
	}
	for _, entry := range entries {
		if err := addTree(tw, filepath.Join(srcDir, entry.Name()), entry.Name()); err != nil {
			_ = tw.Close() // 写入已完成，关闭错误不影响产物
			_ = f.Close()  // 写入已完成，关闭错误不影响产物
			return err
		}
	}
	if err := tw.Close(); err != nil {
		_ = f.Close() // 写入已完成，关闭错误不影响产物
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, destTar)
}

// packDirToTarGz 用于重建 SmartPackage（gzip 压缩级别与 Python 不同，内容等价）。
func packDirToTarGz(srcDir, destTarGz string) error {
	tmp := destTarGz + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		_ = f.Close() // 写入已完成，关闭错误不影响产物
		return err
	}
	for _, entry := range entries {
		if err := addTree(tw, filepath.Join(srcDir, entry.Name()), entry.Name()); err != nil {
			_ = tw.Close() // 写入已完成，关闭错误不影响产物
			_ = gz.Close() // 写入已完成，关闭错误不影响产物
			_ = f.Close()  // 写入已完成，关闭错误不影响产物
			return err
		}
	}
	if err := tw.Close(); err != nil {
		_ = f.Close() // 写入已完成，关闭错误不影响产物
		return err
	}
	if err := gz.Close(); err != nil {
		_ = f.Close() // 写入已完成，关闭错误不影响产物
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, destTarGz)
}

func addTree(tw *tar.Writer, fsPath, arcName string) error {
	info, err := os.Lstat(fsPath)
	if err != nil {
		return err
	}

	link := ""
	if info.Mode()&os.ModeSymlink != 0 {
		if link, err = os.Readlink(fsPath); err != nil {
			return err
		}
	}

	hdr, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	hdr.Name = arcName
	if info.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
		hdr.Name += "/"
	}
	// 对齐 Python tarfile 的 uname/gname 填充行为
	if cur, err := user.Current(); err == nil {
		hdr.Uname = cur.Username
		if grp, err := user.LookupGroupId(cur.Gid); err == nil {
			hdr.Gname = grp.Name
		}
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	// 目录必须递归：Python 的 tar.add 对目录会自动展开子项
	if info.IsDir() {
		children, err := os.ReadDir(fsPath)
		if err != nil {
			return err
		}
		for _, child := range children {
			if err := addTree(tw, filepath.Join(fsPath, child.Name()), path.Join(arcName, child.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return nil
	}

	f, err := os.Open(fsPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(tw, f)
	return err
}

// ── 杂项 ────────────────────────────────────────────────────────────────────

// marshalCompact 输出紧凑 JSON（无空格、不转义 HTML），与 Python json.dumps 一致。
func marshalCompact(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func writeJSONFile(path string, v any) error {
	data, err := marshalCompact(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, fsutil.FileMode)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	if err := os.MkdirAll(filepath.Dir(dst), fsutil.DirMode); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close() // 写入已完成，关闭错误不影响产物
		return err
	}
	return out.Close()
}

func fileSizeOf(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
