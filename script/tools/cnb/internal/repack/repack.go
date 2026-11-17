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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
)

// ── OCI / docker 载荷结构 ───────────────────────────────────────────────────
//
// 这里的 JSON 键名由规范固定，不是本仓库的 snake_case 约定：
//   OCI image-spec → mediaType / digest / size / schemaVersion / config / layers
//   docker-archive → Config / RepoTags / Layers（见其他文件）
// 改名会导致 skopeo / containerd / lifecycle 无法解析，故保持原样。
// 字段顺序即输出键顺序，也不要随意调整。

// ── 入口 ────────────────────────────────────────────────────────────────────

// Run 规范化目标（目录或 *.tar.gz）。
func Run(target string) error {
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return &errs.NotFound{Kind: "target", Ref: target}
	}
	if info.IsDir() {
		return normalizePackageDir(abs)
	}
	if info.Mode().IsRegular() && strings.HasSuffix(abs, ArchiveSuffix) {
		return normalizePackageTarball(abs)
	}
	return &errs.Invalid{Field: "target", Value: target, Reason: "expect a directory or *.tar.gz"}
}

func normalizePackageDir(dir string) error {
	webTar := filepath.Join(dir, WebTarName)
	if _, err := os.Stat(webTar); err != nil {
		return &errs.NotFound{Kind: WebTarName, Ref: dir}
	}
	return normalizeWebTar(webTar)
}

func normalizePackageTarball(pkgTar string) error {
	tempRoot, err := os.MkdirTemp("", TempPkgPrefix)
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tempRoot) }() // 临时目录清理失败不影响结果

	pkgDir := filepath.Join(tempRoot, DirPkg)
	if err := os.MkdirAll(pkgDir, fsutil.DirMode); err != nil {
		return err
	}
	if err := extractTar(pkgTar, pkgDir); err != nil {
		return errs.Wrap("extract "+filepath.Base(pkgTar), err)
	}
	if err := normalizePackageDir(pkgDir); err != nil {
		return err
	}

	rebuilt := filepath.Join(tempRoot, RebuiltTar)
	if err := packDirToTarGz(pkgDir, rebuilt); err != nil {
		return err
	}

	// 先写临时文件再 rename，避免中途失败留下半个包
	tmp := pkgTar + ".tmp"
	if err := fsutil.CopyFile(rebuilt, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, pkgTar)
}

// ── web.tgz 规范化 ──────────────────────────────────────────────────────────

//nolint:funlen // TODO(protocol-layers): manifest 解析、层复制、索引写入拆开
func normalizeWebTar(webTar string) error {
	tempRoot, err := os.MkdirTemp("", TempWebPrefix)
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tempRoot) }() // 临时目录清理失败不影响结果

	legacyDir := filepath.Join(tempRoot, DirLegacy)
	if err := os.MkdirAll(legacyDir, fsutil.DirMode); err != nil {
		return err
	}
	if err := extractTar(webTar, legacyDir); err != nil {
		return errs.Wrap("extract web.tgz", err)
	}

	// 已是 OCI layout → 幂等返回
	if fileExists(filepath.Join(legacyDir, LayoutVersionName)) && fileExists(filepath.Join(legacyDir, IndexName)) {
		return nil
	}

	manifestBytes, err := os.ReadFile(filepath.Join(legacyDir, ManifestName))
	if err != nil {
		return errs.Wrap("missing manifest.json", err)
	}
	var legacy []dockerManifestEntry
	if err := json.Unmarshal(manifestBytes, &legacy); err != nil {
		return errs.Wrap("parse manifest.json", err)
	}
	if len(legacy) == 0 || len(legacy[0].RepoTags) == 0 {
		return &errs.Missing{What: "manifest.json RepoTags"}
	}

	entry := legacy[0]
	repoTag := entry.RepoTags[0]
	repoName, repoVersion := parseRepoTag(repoTag)

	configName := filepath.Base(entry.Config)
	configDigest, _, _ := strings.Cut(configName, ".")
	configSrc := filepath.Join(legacyDir, configName)

	outDir := filepath.Join(tempRoot, DirOCI)
	blobsDir := filepath.Join(outDir, BlobsDir, SHA256Dir)
	if err := os.MkdirAll(blobsDir, fsutil.DirMode); err != nil {
		return err
	}

	if err := fsutil.CopyFile(configSrc, filepath.Join(blobsDir, configDigest)); err != nil {
		return errs.Wrap("copy config", err)
	}
	configSize, err := fsutil.SizeOf(configSrc)
	if err != nil {
		return err
	}
	configDesc := blobDescriptor{
		MediaType: MediaTypeOCIConfig,
		Digest:    DigestPrefix + configDigest,
		Size:      configSize,
	}

	var layers []blobDescriptor
	var legacyLayers []string
	sources := map[string]blobDescriptor{}
	lastLayerDigest := ""

	for _, layerRef := range entry.Layers {
		layerName := filepath.Base(layerRef)
		layerDigest, _, _ := strings.Cut(layerName, ".")
		lastLayerDigest = layerDigest

		layerSrc := filepath.Join(legacyDir, layerName)
		if err := fsutil.CopyFile(layerSrc, filepath.Join(blobsDir, layerDigest)); err != nil {
			return errs.Wrap("copy layer "+layerName, err)
		}
		size, err := fsutil.SizeOf(layerSrc)
		if err != nil {
			return err
		}
		desc := blobDescriptor{
			MediaType: MediaTypeOCILayer,
			Digest:    DigestPrefix + layerDigest,
			Size:      size,
		}
		layers = append(layers, desc)
		legacyLayers = append(legacyLayers, "blobs/sha256/"+layerDigest)
		sources[DigestPrefix+layerDigest] = desc
	}

	manifest := ociManifest{
		SchemaVersion: 2,
		MediaType:     MediaTypeOCIManifest,
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
		MediaType:     MediaTypeOCIIndex,
		Manifests: []indexEntry{{
			MediaType: MediaTypeOCIManifest,
			Digest:    "sha256:" + manifestDigest,
			Size:      int64(len(manifestJSON)),
			Annotations: indexAnnotations{
				ContainerdName: repoTag,
				RefName:        repoVersion,
			},
		}},
	}
	if err := writeJSONFile(filepath.Join(outDir, IndexName), index); err != nil {
		return err
	}

	legacyOut := []legacyManifestEntry{{
		Config:       "blobs/sha256/" + configDigest,
		RepoTags:     []string{strings.TrimPrefix(repoTag, "docker.io/")},
		Layers:       legacyLayers,
		LayerSources: sources,
	}}
	if err := writeJSONFile(filepath.Join(outDir, ManifestName), legacyOut); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(outDir, LayoutVersionName), ociLayout{ImageLayoutVersion: "1.0.0"}); err != nil {
		return err
	}
	repos := map[string]map[string]string{repoName: {repoVersion: lastLayerDigest}}
	if err := writeJSONFile(filepath.Join(outDir, "repositories"), repos); err != nil {
		return err
	}

	// 用规范化后的目录整体替换 web.tgz（plain tar，与 Python 版一致）
	return packDirToTar(outDir, webTar)
}

// ── tar / gzip 基础设施 ─────────────────────────────────────────────────────

// ── 杂项 ────────────────────────────────────────────────────────────────────
