package repack

// repack 的 golden 测试：不依赖任何外部实现（Python 参照物已移除）。
//
// 判据（任一失败即回归）：
//  1. 四个 JSON 载荷与基准字节完全一致
//  2. OCI manifest blob 的 digest 与 index.json 中引用的一致
//  3. 层 blob 内容与输入逐字节相同
//  4. 二次执行幂等（web.tgz 的 sha256 不变）
//  5. 目录模式与 *.tar.gz 模式等价

import (
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"os"
	"path/filepath"
	"testing"
)

// 固定 fixture 产生的基准输出（改动 repack 语义时必须重新生成并复核）。
const (
	goldenIndexJSON = `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:d6890039ce8f51048ab394e06c9b1bc390c9b4ecc5bf6eb55b300b1a11e9c252","size":374,"annotations":{"io.containerd.image.name":"docker.io/demo/web:1.0","org.opencontainers.image.ref.name":"1.0"}}]}`

	goldenManifestJSON = `[{"Config":"blobs/sha256/cccc2222","RepoTags":["demo/web:1.0"],"Layers":["blobs/sha256/aaaa1111","blobs/sha256/bbbb3333"],"LayerSources":{"sha256:aaaa1111":{"mediaType":"application/vnd.oci.image.layer.v1.tar","digest":"sha256:aaaa1111","size":10},"sha256:bbbb3333":{"mediaType":"application/vnd.oci.image.layer.v1.tar","digest":"sha256:bbbb3333","size":10}}}]`

	goldenOCILayout = `{"imageLayoutVersion":"1.0.0"}`

	goldenRepositories = `{"demo/web":{"1.0":"bbbb3333"}}`

	goldenOCIManifest = `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:cccc2222","size":37},"layers":[{"mediaType":"application/vnd.oci.image.layer.v1.tar","digest":"sha256:aaaa1111","size":10},{"mediaType":"application/vnd.oci.image.layer.v1.tar","digest":"sha256:bbbb3333","size":10}]}`

	goldenManifestDigest = "d6890039ce8f51048ab394e06c9b1bc390c9b4ecc5bf6eb55b300b1a11e9c252"
)

const (
	fixtureConfig = `{"architecture":"amd64","os":"linux"}`
	fixtureLayer1 = "layer-one\n"
	fixtureLayer2 = "layer-two\n"
)

// makeDockerSaveFixture 造一个 docker-save 风格的目录：manifest.json + <config>.json + <layer>.tar
func makeDockerSaveFixture(t *testing.T) string {
	t.Helper()
	seed := filepath.Join(t.TempDir(), "seed")
	if err := os.MkdirAll(seed, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"cccc2222.json": fixtureConfig,
		"aaaa1111.tar":  fixtureLayer1,
		"bbbb3333.tar":  fixtureLayer2,
		"manifest.json": `[{"Config":"cccc2222.json","RepoTags":["docker.io/demo/web:1.0"],"Layers":["aaaa1111.tar","bbbb3333.tar"]}]`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(seed, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return seed
}

func makePackageDir(t *testing.T) string {
	t.Helper()
	seed := makeDockerSaveFixture(t)
	pkgDir := t.TempDir()
	if err := packDirToTar(seed, filepath.Join(pkgDir, "web.tgz")); err != nil {
		t.Fatal(err)
	}
	return pkgDir
}

func readFileT(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s: %v", path, err)
	}
	return string(data)
}

func TestRepackDirGolden(t *testing.T) {
	pkgDir := makePackageDir(t)
	if err := Run(pkgDir); err != nil {
		t.Fatalf("repack: %v", err)
	}

	out := t.TempDir()
	if err := extractTar(filepath.Join(pkgDir, "web.tgz"), out); err != nil {
		t.Fatalf("解包结果: %v", err)
	}

	for _, c := range []struct{ name, want string }{
		{"index.json", goldenIndexJSON},
		{"manifest.json", goldenManifestJSON},
		{"oci-layout", goldenOCILayout},
		{"repositories", goldenRepositories},
	} {
		if got := readFileT(t, filepath.Join(out, c.name)); got != c.want {
			t.Errorf("%s 与基准不一致\n got: %s\nwant: %s", c.name, got, c.want)
		}
	}

	// OCI manifest blob：内容与 digest 必须自洽
	blobPath := filepath.Join(out, "blobs", "sha256", goldenManifestDigest)
	if got := readFileT(t, blobPath); got != goldenOCIManifest {
		t.Errorf("OCI manifest blob 内容不符\n got: %s\nwant: %s", got, goldenOCIManifest)
	}
	if sum := sha256Hex([]byte(goldenOCIManifest)); sum != goldenManifestDigest {
		t.Errorf("manifest digest 不自洽: %s", sum)
	}

	// 层与 config blob：内容必须原样搬运
	for name, want := range map[string]string{
		"aaaa1111": fixtureLayer1,
		"bbbb3333": fixtureLayer2,
		"cccc2222": fixtureConfig,
	} {
		if got := readFileT(t, filepath.Join(out, "blobs", "sha256", name)); got != want {
			t.Errorf("blob %s 内容不符", name)
		}
	}
}

func TestRepackIdempotent(t *testing.T) {
	pkgDir := makePackageDir(t)
	webTar := filepath.Join(pkgDir, "web.tgz")

	if err := Run(pkgDir); err != nil {
		t.Fatalf("第一次 repack: %v", err)
	}
	first, err := fileSHA256(webTar)
	if err != nil {
		t.Fatal(err)
	}

	if err := Run(pkgDir); err != nil {
		t.Fatalf("第二次 repack: %v", err)
	}
	second, err := fileSHA256(webTar)
	if err != nil {
		t.Fatal(err)
	}

	if first != second {
		t.Fatalf("非幂等：web.tgz 哈希变化 %s -> %s", first, second)
	}
}

func TestRepackPackageTarball(t *testing.T) {
	pkgDir := makePackageDir(t)

	// 外层包：pkg.tar.gz 内含 web.tgz
	outerDir := t.TempDir()
	if err := fsutil.CopyFile(filepath.Join(pkgDir, "web.tgz"), filepath.Join(outerDir, "web.tgz")); err != nil {
		t.Fatal(err)
	}
	pkgTar := filepath.Join(t.TempDir(), "pkg.tar.gz")
	if err := packDirToTarGz(outerDir, pkgTar); err != nil {
		t.Fatal(err)
	}

	if err := Run(pkgTar); err != nil {
		t.Fatalf("repack tarball: %v", err)
	}

	// 解出 web.tgz 再解包，确认已是 OCI layout
	stage := t.TempDir()
	if err := extractTar(pkgTar, stage); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := extractTar(filepath.Join(stage, "web.tgz"), out); err != nil {
		t.Fatal(err)
	}
	if got := readFileT(t, filepath.Join(out, "index.json")); got != goldenIndexJSON {
		t.Errorf("tarball 模式产物与目录模式不一致")
	}
}

// 已规范化的包必须原样返回（幂等的另一个入口）。
func TestRepackAlreadyOCI(t *testing.T) {
	pkgDir := makePackageDir(t)
	if err := Run(pkgDir); err != nil {
		t.Fatal(err)
	}
	webTar := filepath.Join(pkgDir, "web.tgz")

	before, err := fileSHA256(webTar)
	if err != nil {
		t.Fatal(err)
	}
	// 再跑一次：命中 oci-layout + index.json 的短路判断
	if err := Run(pkgDir); err != nil {
		t.Fatal(err)
	}
	after, err := fileSHA256(webTar)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("已 OCI 的包被重写")
	}
}

// fileSHA256 测试辅助：整文件哈希（用于幂等断言）。
func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}
