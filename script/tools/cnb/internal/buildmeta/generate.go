package buildmeta

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/profile"
)

// Generate 从 profile + lock + 项目 buildpacks 推导 builder metadata。
func Generate(plan *profile.Plan) (*Metadata, error) {
	meta := &Metadata{Description: plan.Project}
	if findOfficial(plan, meta) {
		// 官方 metadata 已含 buildpacks 与 lifecycle，不再扫描
		applyRunImage(plan, meta)
		return meta, nil
	}
	applyRunImage(plan, meta)
	applyLifecycle(plan, meta)
	meta.Lifecycle.API = API{Buildpack: BuildpackAPI, Platform: PlatformAPI}

	bps, err := scanBuildpacks(plan)
	if err != nil {
		return nil, err
	}
	meta.Buildpacks = bps

	if meta.Stack.RunImage.Image == "" {
		return nil, fmt.Errorf("no image stage named %q; cannot derive stack.runImage", StageRun)
	}
	return meta, nil
}

// findOfficial 尝试采用随官方 builder 保存的 metadata。
func findOfficial(plan *profile.Plan, meta *Metadata) bool {
	if plan.BuildpacksDir == "" {
		return false
	}
	cand := filepath.Join(filepath.Dir(plan.BuildpacksDir), FileOfficialMetadata)
	data, err := os.ReadFile(cand)
	if err != nil {
		return false
	}
	var official Metadata
	if err := json.Unmarshal(data, &official); err != nil || len(official.Buildpacks) == 0 {
		return false
	}
	meta.Buildpacks = official.Buildpacks
	meta.Lifecycle = official.Lifecycle
	return true
}

// applyRunImage 取流程中名为 run 的镜像阶段。
func applyRunImage(plan *profile.Plan, meta *Metadata) {
	for _, st := range plan.Stages {
		if st.Kind == profile.StageImage && st.Name == StageRun {
			meta.Stack.RunImage.Image = st.Tag
		}
	}
}

// applyLifecycle 从流程物料中取 lifecycle 版本。
func applyLifecycle(plan *profile.Plan, meta *Metadata) {
	for _, ref := range plan.Assets {
		if after, ok := strings.CutPrefix(ref, AssetLifecyclePrefix); ok {
			meta.Lifecycle.Version = after
		}
	}
}

// scanBuildpacks 扫描项目内的 buildpack 目录（支持扁平与 <id>/<version> 两种布局）。
func scanBuildpacks(plan *profile.Plan) ([]Buildpack, error) {
	dir := plan.BuildpacksDir
	if dir == "" {
		dir = filepath.Join(plan.Dir, "docker", "buildpacks")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errs.Wrap("read buildpacks dir", err)
	}

	var out []Buildpack
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifest := findManifest(filepath.Join(dir, e.Name()))
		data, err := os.ReadFile(manifest)
		if err != nil {
			continue
		}
		if bp := parseBuildpack(string(data)); bp.ID != "" {
			out = append(out, bp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// findManifest 返回 buildpack 元数据路径：自身优先，其次第一个版本子目录。
func findManifest(dir string) string {
	self := filepath.Join(dir, FileBuildpackManifest)
	if _, err := os.Stat(self); err == nil {
		return self
	}
	subs, err := os.ReadDir(dir)
	if err != nil {
		return self
	}
	for _, s := range subs {
		if s.IsDir() {
			return filepath.Join(dir, s.Name(), FileBuildpackManifest)
		}
	}
	return self
}

// parseBuildpack 从 buildpack.toml 文本中提取条目。
func parseBuildpack(text string) Buildpack {
	return Buildpack{
		ID:       firstMatch(idRe, text),
		Version:  firstMatch(verRe, text),
		API:      firstMatch(apiRe, text),
		Homepage: firstMatch(homeRe, text),
	}
}
