// Package buildmeta 生成 pack 要求的 builder metadata（io.buildpacks.builder.metadata）。
package buildmeta

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/profile"
)

type Metadata struct {
	Description string      `json:"description"`
	Stack       Stack       `json:"stack"`
	Buildpacks  []Buildpack `json:"buildpacks"`
	Lifecycle   Lifecycle   `json:"lifecycle"`
}

type Stack struct {
	RunImage RunImage `json:"runImage"`
}

type RunImage struct {
	Image string `json:"image"`
}

type Buildpack struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	API      string `json:"api"`
	Homepage string `json:"homepage,omitempty"`
}

type Lifecycle struct {
	Version string `json:"version"`
	API     API    `json:"api"`
}

type API struct {
	Buildpack string `json:"buildpack"`
	Platform  string `json:"platform"`
}

var (
	idRe    = regexp.MustCompile(`(?m)^\s*id\s*=\s*"([^"]+)"`)
	verRe   = regexp.MustCompile(`(?m)^\s*version\s*=\s*"([^"]+)"`)
	apiRe   = regexp.MustCompile(`(?m)^\s*api\s*=\s*"([^"]+)"`)
	homeRe  = regexp.MustCompile(`(?m)^\s*homepage\s*=\s*"([^"]+)"`)
	labelRe = regexp.MustCompile(`io\.buildpacks\.stack\.id="([^"]+)"`)
)

// Generate 从 profile + lock + 项目 buildpacks 推导 builder metadata。
func Generate(plan *profile.Plan) (*Metadata, error) {
	meta := &Metadata{Description: plan.Project}
	// 若有官方 builder 的 metadata（提取时保存），直接采用其 buildpacks 列表
	if plan.BuildpacksDir != "" {
		cand := filepath.Join(filepath.Dir(plan.BuildpacksDir), "builder-metadata.json")
		if data, err := os.ReadFile(cand); err == nil {
			var official Metadata
			if json.Unmarshal(data, &official) == nil && len(official.Buildpacks) > 0 {
				meta.Buildpacks = official.Buildpacks
				meta.Lifecycle = official.Lifecycle
			}
		}
	}
	// run image：取流程中名为 run 的阶段
	for _, st := range plan.Stages {
		if st.Kind == "image" && st.Name == "run" {
			meta.Stack.RunImage.Image = st.Tag
		}
	}
	// lifecycle：从流程物料里找 lifecycle
	for _, ref := range plan.Assets {
		if strings.HasPrefix(ref, "lifecycle@") {
			meta.Lifecycle.Version = strings.TrimPrefix(ref, "lifecycle@")
		}
	}
	meta.Lifecycle.API = API{Buildpack: "0.12", Platform: "0.15"}

	// buildpacks：扫项目 docker/buildpacks/*/buildpack.toml
	dir := plan.BuildpacksDir
	if dir == "" {
		dir = filepath.Join(plan.Dir, "docker", "buildpacks")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取 buildpacks 目录: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cand := filepath.Join(dir, e.Name(), "buildpack.toml")
		if _, err := os.Stat(cand); err != nil {
			// 官方布局：<id>/<version>/buildpack.toml
			subs, readErr := os.ReadDir(filepath.Join(dir, e.Name()))
			if readErr != nil {
				continue
			}
			for _, s := range subs {
				if s.IsDir() {
					cand = filepath.Join(dir, e.Name(), s.Name(), "buildpack.toml")
					break
				}
			}
		}
		data, err := os.ReadFile(cand)
		if err != nil {
			continue
		}
		text := string(data)
		bp := Buildpack{
			ID:       firstMatch(idRe, text),
			Version:  firstMatch(verRe, text),
			API:      firstMatch(apiRe, text),
			Homepage: firstMatch(homeRe, text),
		}
		if bp.ID != "" {
			meta.Buildpacks = append(meta.Buildpacks, bp)
		}
	}
	sort.Slice(meta.Buildpacks, func(i, j int) bool { return meta.Buildpacks[i].ID < meta.Buildpacks[j].ID })

	if meta.Stack.RunImage.Image == "" {
		return nil, fmt.Errorf("流程中没有名为 run 的镜像阶段，无法生成 stack.runImage")
	}
	return meta, nil
}

// StackID 从构建该镜像的 Dockerfile 里读 stack id 标签。
func StackID(plan *profile.Plan) string {
	for _, st := range plan.Stages {
		if st.Kind != "image" {
			continue
		}
		data, err := os.ReadFile(st.Dockerfile)
		if err != nil {
			continue
		}
		if m := labelRe.FindStringSubmatch(string(data)); m != nil {
			return m[1]
		}
	}
	return ""
}

// JSON 输出紧凑的 metadata。
func (m *Metadata) JSON() (string, error) {
	b, err := json.Marshal(m)
	return string(b), err
}

func firstMatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}
