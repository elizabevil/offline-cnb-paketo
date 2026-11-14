package profile

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// schemaVersion 是配置文件的协议版本。
const schemaVersion = 1

// 校验结果的字段前缀（统一在这里维护，便于错误聚合）。
const (
	prefixProject  = "[project]"
	prefixDocker   = "[docker]"
	prefixAssets   = "[assets.%s]"
	prefixImages   = "[images.%s]"
	prefixCommands = "[commands.%s]"
)

// Validate 校验配置与 lock 的一致性，返回错误清单。
func (p *File) Validate(platform string, l *lock.File) []string {
	var errs []string
	errs = append(errs, p.validateBasic(platform)...)
	errs = append(errs, p.validateAssets(l)...)
	errs = append(errs, p.validateImages()...)
	errs = append(errs, p.validateCommands()...)
	errs = append(errs, p.validateStages()...)
	return errs
}

// validateBasic 校验 schema、项目目录、平台与 bundle 布局。
func (p *File) validateBasic(platform string) []string {
	var errs []string
	if p.Schema != schemaVersion {
		errs = append(errs, fmt.Sprintf("%s schema must be %d, got %d", prefixProject, schemaVersion, p.Schema))
	}
	if p.Project.Name == "" {
		errs = append(errs, prefixProject+" name is required")
	}
	if p.Project.Dir == "" {
		errs = append(errs, prefixProject+" dir is required (relative to repo root)")
	} else if !isDir(filepath.Join(repoRoot(), p.Project.Dir)) {
		errs = append(errs, fmt.Sprintf("%s dir not found: %s", prefixProject, p.Project.Dir))
	}
	if !lock.KnownPlatforms[platform] {
		errs = append(errs, fmt.Sprintf("unsupported platform: %s", platform))
	}
	switch p.Docker.BundleLayout {
	case "", lock.LayoutArchFirst, lock.LayoutKindFirst:
	default:
		errs = append(errs, fmt.Sprintf("%s bundle_layout must be %q or %q, got %q",
			prefixDocker, lock.LayoutArchFirst, lock.LayoutKindFirst, p.Docker.BundleLayout))
	}
	return errs
}

// validateAssets 校验物料引用与平台支持。
func (p *File) validateAssets(l *lock.File) []string {
	var errs []string
	for _, id := range slices.Sorted(maps.Keys(p.Assets)) {
		a := p.Assets[id]
		asset, err := lock.Find(l, a.ref(id))
		if err != nil {
			errs = append(errs, fmt.Sprintf(prefixAssets+" %v", id, err))
			continue
		}
		for _, pf := range a.Platforms {
			if msg := checkPlatform(asset, pf); msg != "" {
				errs = append(errs, fmt.Sprintf(prefixAssets+" %s", id, msg))
			}
		}
	}
	return errs
}

// validateImages 校验镜像定义。
func (p *File) validateImages() []string {
	var errs []string
	ctxBase := filepath.Join(repoRoot(), mustDefault(p.Docker.Context, "."))
	for _, name := range slices.Sorted(maps.Keys(p.Images)) {
		img := p.Images[name]
		if img.Dockerfile == "" {
			errs = append(errs, fmt.Sprintf(prefixImages+" dockerfile is required", name))
		} else if !fileExists(filepath.Join(ctxBase, img.Dockerfile)) {
			errs = append(errs, fmt.Sprintf(prefixImages+" dockerfile not found: %s", name, img.Dockerfile))
		}
		if img.Tag == "" {
			errs = append(errs, fmt.Sprintf(prefixImages+" tag is required", name))
		}
		for _, id := range img.InjectVersions {
			if _, ok := p.Assets[id]; !ok {
				errs = append(errs, fmt.Sprintf(prefixImages+" inject_versions references undeclared asset: %s", name, id))
			}
		}
		for _, pf := range img.Platforms {
			if !lock.KnownPlatforms[pf] {
				errs = append(errs, fmt.Sprintf(prefixImages+" unknown platform %s", name, pf))
			}
		}
	}
	return errs
}

// validateCommands 校验命令定义。
func (p *File) validateCommands() []string {
	var errs []string
	for _, name := range slices.Sorted(maps.Keys(p.Commands)) {
		c := p.Commands[name]
		if c.Run == "" {
			errs = append(errs, fmt.Sprintf(prefixCommands+" run is required", name))
		}
		workdir := mustDefault(c.Workdir, p.Project.Dir)
		if !isDir(filepath.Join(repoRoot(), workdir)) {
			errs = append(errs, fmt.Sprintf(prefixCommands+" workdir not found: %s", name, workdir))
		}
	}
	return errs
}

// validateStages 校验流程顺序与引用。
func (p *File) validateStages() []string {
	var errs []string
	if len(p.Stages) == 0 {
		return []string{"at least one [[stage]] is required"}
	}
	used, seen := map[string]bool{}, map[string]bool{}
	for i, st := range p.Stages {
		switch {
		case st.Image == "" && st.Command == "":
			errs = append(errs, fmt.Sprintf("stage %d must set image or command", i+1))
		case st.Image != "" && st.Command != "":
			errs = append(errs, fmt.Sprintf("stage %d must set only one of image/command", i+1))
		case st.Image != "":
			errs = append(errs, p.checkImageStage(i, st.Image, used, seen)...)
		default:
			errs = append(errs, p.checkCommandStage(i, st.Command, used, seen)...)
		}
	}
	errs = append(errs, p.checkUnused(used)...)
	return errs
}

// checkImageStage 校验镜像阶段。
func (p *File) checkImageStage(idx int, name string, used, seen map[string]bool) []string {
	img, ok := p.Images[name]
	if !ok {
		return []string{fmt.Sprintf("stage %d references unknown image: %s", idx+1, name)}
	}
	var errs []string
	if used[name] {
		errs = append(errs, fmt.Sprintf("image %s appears more than once", name))
	}
	refs := append(tagRefNames(img.Tag), tagRefNamesInMap(img.Args)...)
	errs = append(errs, p.checkTagRefs(fmt.Sprintf(prefixImages, name), refs, seen)...)
	used[name], seen[name] = true, true
	return errs
}

// checkCommandStage 校验命令阶段。
func (p *File) checkCommandStage(idx int, name string, used, seen map[string]bool) []string {
	cmd, ok := p.Commands[name]
	if !ok {
		return []string{fmt.Sprintf("stage %d references unknown command: %s", idx+1, name)}
	}
	var errs []string
	if used[name] {
		errs = append(errs, fmt.Sprintf("command %s appears more than once", name))
	}
	errs = append(errs, p.checkTagRefs(fmt.Sprintf(prefixCommands, name), tagRefNamesInMap(cmd.Env), seen)...)
	used[name] = true
	return errs
}

// checkTagRefs 校验镜像标签引用：必须存在且已在流程中出现过。
func (p *File) checkTagRefs(prefix string, refs []string, seen map[string]bool) []string {
	var errs []string
	for _, ref := range refs {
		if _, ok := p.Images[ref]; !ok {
			errs = append(errs, fmt.Sprintf("%s references unknown image tag: %s", prefix, ref))
			continue
		}
		if !seen[ref] {
			errs = append(errs, fmt.Sprintf("%s references a later image tag: %s (move its stage earlier)", prefix, ref))
		}
	}
	return errs
}

// checkUnused 报告已定义但未进入流程的镜像与命令。
func (p *File) checkUnused(used map[string]bool) []string {
	var errs []string
	for _, name := range slices.Sorted(maps.Keys(p.Images)) {
		if !used[name] {
			errs = append(errs, fmt.Sprintf(prefixImages+" defined but not used in [[stage]]", name))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(p.Commands)) {
		if !used[name] {
			errs = append(errs, fmt.Sprintf(prefixCommands+" defined but not used in [[stage]]", name))
		}
	}
	return errs
}

// checkPlatform 返回物料不支持该平台的原因（空表示支持）。
func checkPlatform(a *lock.Asset, pf string) string {
	if !lock.KnownPlatforms[pf] {
		return "unknown platform " + pf
	}
	if _, ok := a.URL(lock.Platform(pf)); !ok {
		return "lock does not support platform " + pf
	}
	return ""
}

// isDir 判断路径是否为目录。
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// fileExists 判断文件是否存在。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
