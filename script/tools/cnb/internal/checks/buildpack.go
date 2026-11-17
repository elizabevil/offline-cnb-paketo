// Package checks 提供静态检查与元数据生成：buildpack 结构、Dockerfile 源、环境体检、builder metadata。
package checks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	speclifecycle "learn-cnb-paketo/tools/cnb/internal/spec/lifecycle"
)

// 检查规则与约定。
const (
	ManifestName   = "buildpack.toml"
	DetectScript   = "detect"
	BuildScript    = "build"
	BinDir         = "bin"
	IDPrefix       = "paketo-buildpacks_"
	IDPrefixSlash  = "paketo-buildpacks/"
	FileModeExec   = 0o111
	ProviderMarker = ".sbom."
)

// Providers 是必须声明 [[metadata.dependencies]] 的组件（提供外部 SDK）。
var Providers = map[string]bool{
	"node-engine": true,
	"cpython":     true,
	"go-dist":     true,
}

// Finding 是一条检查结果。
type Finding struct {
	Name    string
	Message string
	IsError bool
}

// BuildpackReport 是一个 buildpack 目录的检查结果。
type BuildpackReport struct {
	Findings []Finding
}

// Errors 返回错误条数。
func (r BuildpackReport) Errors() int {
	n := 0
	for _, f := range r.Findings {
		if f.IsError {
			n++
		}
	}
	return n
}

// Warnings 返回警告条数。
func (r BuildpackReport) Warnings() int { return len(r.Findings) - r.Errors() }

// OK 判断是否无错误。
func (r BuildpackReport) OK() bool { return r.Errors() == 0 }

// CheckBuildpack 检查单个 buildpack 目录。
func CheckBuildpack(dir string) BuildpackReport {
	var rep BuildpackReport
	leaf, versionDir := locate(dir)
	add := func(ok bool, msg string) {
		if !ok {
			rep.Findings = append(rep.Findings, Finding{Name: leaf, Message: msg, IsError: true})
		}
	}
	warn := func(ok bool, msg string) {
		if !ok {
			rep.Findings = append(rep.Findings, Finding{Name: leaf, Message: msg})
		}
	}

	data, err := os.ReadFile(filepath.Join(versionDir, ManifestName))
	if err != nil {
		add(false, "missing "+ManifestName)
		return rep
	}
	bp, warns, err := speclifecycle.ParseBuildpack(data)
	if err != nil {
		add(false, "invalid "+ManifestName+": "+err.Error())
		return rep
	}
	for _, w := range warns {
		rep.Findings = append(rep.Findings, Finding{Name: leaf, Message: w.String()})
	}

	add(bp.API != "", "missing api")
	add(bp.BuildpackInfo.ID != "", "missing [buildpack] id")
	add(bp.BuildpackInfo.Version != "", "missing [buildpack] version")
	add(bp.BuildpackInfo.Name != "", "missing [buildpack] name")
	add(idMatchesDir(bp.BuildpackInfo.ID, leaf),
		fmt.Sprintf("dir %s does not match id %s", leaf, bp.BuildpackInfo.ID))

	if len(bp.Order) > 0 {
		checkMeta(&rep, versionDir, bp.Order)
		return rep
	}
	checkComponent(&rep, versionDir, leaf, bp, warn)
	return rep
}

// locate 返回目录名与含 buildpack.toml 的目录（支持 <id>/<version>）。
func locate(dir string) (string, string) {
	leaf := filepath.Base(dir)
	if exists(filepath.Join(dir, ManifestName)) {
		return leaf, dir
	}
	subs, err := os.ReadDir(dir)
	if err != nil {
		return leaf, dir
	}
	for _, s := range subs {
		cand := filepath.Join(dir, s.Name())
		if s.IsDir() && exists(filepath.Join(cand, ManifestName)) {
			return leaf, cand
		}
	}
	return leaf, dir
}

// checkMeta 检查 meta buildpack：不得含脚本，order 引用的组件需存在。
func checkMeta(rep *BuildpackReport, dir string, order []speclifecycle.Group) {
	add := func(ok bool, msg string) {
		if !ok {
			rep.Findings = append(rep.Findings, Finding{Message: msg, IsError: true})
		}
	}
	detect := filepath.Join(dir, BinDir, DetectScript)
	build := filepath.Join(dir, BinDir, BuildScript)
	add(!exists(detect) && !exists(build), "meta buildpack must not contain bin scripts")

	root := filepath.Dir(dir)
	for _, g := range order {
		gid := g.ID
		if gid == "" {
			continue
		}
		name := IDPrefix + strings.TrimPrefix(gid, IDPrefixSlash)
		add(exists(filepath.Join(root, name)) ||
			exists(filepath.Join(root, strings.TrimPrefix(gid, IDPrefixSlash))),
			fmt.Sprintf("order references missing buildpack %s", gid))
	}
}

// checkComponent 检查实现型 buildpack：脚本、权限、SBOM 与依赖声明。
func checkComponent(rep *BuildpackReport, dir, leaf string, bp *speclifecycle.Buildpack, warn func(bool, string)) {
	add := func(ok bool, msg string) {
		if !ok {
			rep.Findings = append(rep.Findings, Finding{Message: msg, IsError: true})
		}
	}
	detect := filepath.Join(dir, BinDir, DetectScript)
	build := filepath.Join(dir, BinDir, BuildScript)
	add(exists(detect), "missing bin/detect")
	add(exists(build), "missing bin/build")
	for _, f := range []string{detect, build} {
		if info, err := os.Stat(f); err == nil && info.Mode()&FileModeExec == 0 {
			add(false, filepath.Base(f)+" is not executable")
		}
	}
	if writesSBOM(detect, build) {
		warn(len(bp.SBOMFormats) > 0, "writes SBOM but declares no sbom-formats")
	}
	if Providers[leaf] {
		warn(len(bp.MetadataSection.Dependencies) > 0, "provider buildpack without [[metadata.dependencies]]")
	}
}

// writesSBOM 判断脚本是否写出 SBOM。
func writesSBOM(paths ...string) bool {
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil && strings.Contains(string(data), ProviderMarker) {
			return true
		}
	}
	return false
}

// idMatchesDir 判断目录名与 id 是否匹配（兼容 paketo-buildpacks/x 与 x）。
func idMatchesDir(id, leaf string) bool {
	if id == "" {
		return true
	}
	last := id[strings.LastIndex(id, "/")+1:]
	return leaf == IDPrefix+strings.TrimPrefix(id, IDPrefixSlash) || leaf == last
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }
