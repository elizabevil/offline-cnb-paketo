package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// 物料布局（与 script/lib/common.sh 保持一致）:
//
//	bundle/<platform>/archives/<id>/<version>/<basename(url)>
//	bundle/<platform>/binaries/<extract_to>/
//	bundle/<platform>/binaries/.meta/<id>.sha256    ← 解压标记，内容为归档 sha256 或 "unpinned"

func archivesDir(bundle, platform string) string {
	return filepath.Join(bundle, platform, "archives")
}

func binariesDir(bundle, platform string) string {
	return filepath.Join(bundle, platform, "binaries")
}

func markerPath(bundle, platform, id string) string {
	return filepath.Join(binariesDir(bundle, platform), ".meta", id+".sha256")
}

func archivePath(bundle, platform string, a *lock.Asset) string {
	return filepath.Join(archivesDir(bundle, platform), a.ID, a.Version, filepath.Base(mustURL(a, platform)))
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// markerMatches 判断解压标记是否与当前 lock 中的哈希一致。
func markerMatches(bundle, platform string, a *lock.Asset) bool {
	data, err := os.ReadFile(markerPath(bundle, platform, a.Ref()))
	if err != nil {
		return false
	}
	recorded := strings.TrimSpace(string(data))
	want := a.SHA256(platform)
	if want == "" {
		// 未固定哈希的物料：标记里写 unpinned，仍可判定"已解压"
		return recorded == "unpinned"
	}
	return recorded == want
}

// expectOK 检查解压后声明的产物是否齐全。
func expectOK(bundle, platform string, a *lock.Asset) bool {
	base := binariesDir(bundle, platform)
	for _, rel := range a.ExpectedPaths() {
		if _, err := os.Stat(filepath.Join(base, rel)); err != nil {
			return false
		}
	}
	return true
}

func isExtracted(bundle, platform string, a *lock.Asset) bool {
	return markerMatches(bundle, platform, a) && expectOK(bundle, platform, a)
}

// ── verify ──────────────────────────────────────────────────────────────────

// Verify 校验归档哈希、解压标记与预期产物。
func Verify(l *lock.File, bundleDir, platform string, ids []string, strict, quiet bool) error {
	if !lock.KnownPlatforms[platform] {
		return fmt.Errorf("不支持的平台: %s", platform)
	}
	assets, err := l.Select(ids)
	if err != nil {
		return err
	}

	fmt.Printf("\n━━━  校验物料 (%s)  \n", platform)

	failures, unpinned := 0, 0
	for i := range assets {
		a := &assets[i]
		var problems []string

		archive := archivePath(bundleDir, platform, a)
		if _, statErr := os.Stat(archive); statErr != nil {
			problems = append(problems, "归档缺失")
		} else if a.SHA256(platform) == "" {
			unpinned++
			problems = append(problems, "sha256 未固定")
		} else if got, shaErr := fileSHA256(archive); shaErr != nil {
			problems = append(problems, "哈希计算失败: "+shaErr.Error())
		} else if got != a.SHA256(platform) {
			problems = append(problems, "哈希不符")
		}

		if !isExtracted(bundleDir, platform, a) {
			problems = append(problems, "未解压或产物不全")
		}

		if len(problems) == 0 {
			if !quiet {
				fmt.Printf("  ✓  %s %s\n", a.ID, a.Version)
			}
			continue
		}
		fmt.Printf("  ✗  %s %s: %s\n", a.ID, a.Version, strings.Join(problems, "; "))
		failures++
	}

	if failures > 0 {
		if unpinned > 0 && !strict {
			fmt.Fprintf(os.Stderr, "  !  %d 个物料的 sha256 尚未固定（收紧供应链前必须处理）\n", unpinned)
			// 只有「未固定」这一条问题时，非 strict 模式放行
			if failures == unpinned {
				fmt.Println("\n━━━  校验通过（含未固定项）  ")
				return nil
			}
		}
		return fmt.Errorf("校验失败: %d 个物料不达标", failures)
	}

	fmt.Println("\n━━━  校验通过  ")
	return nil
}

// ── pin ─────────────────────────────────────────────────────────────────────

// pinAssets 打印建议的 TOML 片段；不写回 lock（写入必须人工确认）。
// Pin 由本地归档反推 sha256，只打印建议片段，不写回 lock。
func Pin(l *lock.File, bundleDir, platform string, ids []string) error {
	if !lock.KnownPlatforms[platform] {
		return fmt.Errorf("不支持的平台: %s", platform)
	}
	assets, err := l.Select(ids)
	if err != nil {
		return err
	}

	fmt.Printf("\n━━━  反推 sha256 (%s)  \n", platform)
	fmt.Println("     把下面的片段人工确认后回填 assets.lock.toml，并更新 meta.provenance")

	printed := 0
	for i := range assets {
		a := &assets[i]
		archive := archivePath(bundleDir, platform, a)
		if _, statErr := os.Stat(archive); statErr != nil {
			fmt.Fprintf(os.Stderr, "  !  %s: 归档不存在，跳过\n", a.ID)
			continue
		}
		sum, shaErr := fileSHA256(archive)
		if shaErr != nil {
			return fmt.Errorf("%s: 计算哈希失败: %w", a.ID, shaErr)
		}
		fmt.Printf("\n# ---- %s@%s （把下面一行填进 [asset.platform.%s]）----\n  sha256 = \"%s\"\n",
			a.ID, a.Version, platform, sum)
		printed++
	}
	if printed == 0 {
		return fmt.Errorf("没有可固定的归档")
	}
	return nil
}

// mustURL 返回展开后的下载地址（调用前已由 Select/Find 保证平台存在）。
func mustURL(a *lock.Asset, platform string) string {
	u, _ := a.URL(platform)
	return u
}
