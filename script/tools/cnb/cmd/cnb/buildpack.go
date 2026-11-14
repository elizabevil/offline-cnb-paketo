package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var (
	apiRe   = regexp.MustCompile(`(?m)^\s*api\s*=\s*"([^"]+)"`)
	idRe2   = regexp.MustCompile(`(?m)^\s*id\s*=\s*"([^"]+)"`)
	verRe2  = regexp.MustCompile(`(?m)^\s*version\s*=\s*"([^"]+)"`)
	nameRe  = regexp.MustCompile(`(?m)^\s*name\s*=\s*"([^"]+)"`)
	orderRe = regexp.MustCompile(`(?m)^\s*\[\[order\]\]`)
	groupRe = regexp.MustCompile(`(?ms)\[\[order\.group\]\](.*?)(?:\[\[|\z)`)
	gidRe   = regexp.MustCompile(`(?m)^\s*id\s*=\s*"([^"]+)"`)
	sbomRe  = regexp.MustCompile(`(?m)^\s*sbom-formats\s*=`)
	depsRe  = regexp.MustCompile(`(?m)^\s*\[\[metadata\.dependencies\]\]`)
)

type lintResult struct{ errors, warnings int }

// newBuildpackCmd — buildpack 结构与契约的静态检查
func newBuildpackCmd() *cobra.Command {
	parent := &cobra.Command{Use: "buildpack", Short: "buildpack 维护"}
	parent.AddCommand(&cobra.Command{
		Use:   "lint [dir…]",
		Short: "按官方模板检查 buildpack 结构",
		RunE: func(cmd *cobra.Command, args []string) error {
			dirs := args
			if len(dirs) == 0 {
				d, _ := resolveRepo()
				dirs, _ = filepath.Glob(filepath.Join(d, "project", "*", "docker", "buildpacks", "*"))
			}
			var res lintResult
			for _, d := range dirs {
				lintBuildpack(cmd, d, &res)
			}
			fmt.Printf("\n  %d error, %d warning\n", res.errors, res.warnings)
			if res.errors > 0 {
				return &exitError{code: 1}
			}
			return nil
		},
	})
	return parent
}

func lintBuildpack(cmd *cobra.Command, dir string, res *lintResult) {
	// 支持官方布局：<id>/<version>/buildpack.toml
	if !exists(filepath.Join(dir, "buildpack.toml")) {
		if subs, err := os.ReadDir(dir); err == nil {
			for _, sub := range subs {
				cand := filepath.Join(dir, sub.Name())
				if sub.IsDir() && exists(filepath.Join(cand, "buildpack.toml")) {
					dir = cand
					break
				}
			}
		}
	}
	leaf := filepath.Base(filepath.Dir(dir))
	if leaf == "." || leaf == "" {
		leaf = filepath.Base(dir)
	}
	report := func(ok bool, msg string) {
		if ok {
			return
		}
		res.errors++
		fmt.Printf("  ✗ %s: %s\n", leaf, msg)
	}
	warn := func(ok bool, msg string) {
		if ok {
			return
		}
		res.warnings++
		fmt.Printf("  ! %s: %s\n", leaf, msg)
	}

	path := filepath.Join(dir, "buildpack.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		report(false, "缺少 buildpack.toml")
		return
	}
	text := string(data)

	report(apiRe.MatchString(text), "缺少 api")
	id := firstOf(idRe2, text)
	ver := firstOf(verRe2, text)
	name := firstOf(nameRe, text)
	report(id != "", "缺少 [buildpack] id")
	report(ver != "", "缺少 [buildpack] version")
	report(name != "", "缺少 [buildpack] name")

	// id 与目录名一致（允许 paketo-buildpacks_x 或 x）
	if id != "" && leaf != "x" {
		want := "paketo-buildpacks_" + strings.TrimPrefix(id, "paketo-buildpacks/")
		last := id[strings.LastIndex(id, "/")+1:]
		report(leaf == want || leaf == last, fmt.Sprintf("目录名 %s 与 id %s 不一致（应为 %s）", leaf, id, want))
	}

	isMeta := orderRe.MatchString(text)
	detectPath := filepath.Join(dir, "bin", "detect")
	buildPath := filepath.Join(dir, "bin", "build")

	if isMeta {
		report(!exists(detectPath) && !exists(buildPath), "meta buildpack 不应包含 bin/ 脚本")
		// order 引用的组件需在本仓库存在
		root := filepath.Dir(dir)
		for _, g := range groupRe.FindAllStringSubmatch(text, -1) {
			gid := firstOf(gidRe, g[1])
			if gid == "" {
				continue
			}
			leafID := "paketo-buildpacks_" + strings.TrimPrefix(gid, "paketo-buildpacks/")
			report(exists(filepath.Join(root, leafID)) || exists(filepath.Join(root, strings.TrimPrefix(gid, "paketo-buildpacks/"))),
				fmt.Sprintf("order 引用的 %s 在本仓库不存在", gid))
		}
		return
	}

	report(exists(detectPath), "缺少 bin/detect")
	report(exists(buildPath), "缺少 bin/build")
	for _, f := range []string{detectPath, buildPath} {
		if info, err := os.Stat(f); err == nil && info.Mode()&0o111 == 0 {
			report(false, filepath.Base(f)+" 缺少可执行权限")
		}
	}
	warn(sbomRe.MatchString(text), "未声明 sbom-formats")
	warn(depsRe.MatchString(text), "未声明 [[metadata.dependencies]]")
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func firstOf(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}
