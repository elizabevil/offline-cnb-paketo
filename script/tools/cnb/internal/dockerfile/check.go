// Package dockerfile 静态检查镜像构建所需的源文件是否存在。
package dockerfile

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/profile"
)

// 解析规则。
var (
	copyPattern = regexp.MustCompile(`^(COPY|ADD)\s+(.*)$`)
	varPattern  = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)
)

// 命名构建上下文（与 profile 约定一致）。
const (
	ContextBundle     = "bundle"
	ContextBuildpacks = "buildpacks"

	// KindImage 是计划中的镜像阶段类型。
	KindImage = "image"

	// ArchPrefix 是平台名中的架构前缀。
	ArchPrefix = "linux-"
)

// Missing 是一条缺失的源路径。
type Missing struct {
	Stage  string
	Source string
	Target string
}

// Report 是检查结果。
type Report struct {
	Missing []Missing
}

// OK 判断是否全部存在。
func (r Report) OK() bool { return len(r.Missing) == 0 }

// Check 遍历计划中的镜像阶段，校验 COPY/ADD 源路径。
func Check(plan *profile.Plan, platform string) Report {
	var rep Report
	for _, st := range plan.Stages {
		if st.Kind != KindImage {
			continue
		}
		env := baseEnv(platform)
		maps.Copy(env, st.Args)
		for _, src := range copySources(st.Dockerfile, env) {
			target, ok := resolve(plan, st, src)
			if !ok {
				continue
			}
			if _, err := os.Stat(target); err != nil {
				rep.Missing = append(rep.Missing, Missing{Stage: st.Name, Source: src, Target: target})
			}
		}
	}
	return rep
}

// baseEnv 返回构建参数的默认值。
func baseEnv(platform string) map[string]string {
	return map[string]string{
		"BUNDLE_ARCH": platform,
		"ARCH":        strings.TrimPrefix(platform, ArchPrefix),
	}
}

// resolve 把 COPY 源映射到宿主机路径；不认识的 --from 返回 false。
func resolve(plan *profile.Plan, st profile.PlanStage, src string) (string, bool) {
	from, path := splitFrom(src)
	switch from {
	case "":
		return filepath.Join(st.Context, path), true
	case ContextBuildpacks:
		return joinContext(plan.RepoRoot, st.BuildpacksCtx, path), true
	case ContextBundle:
		return filepath.Join(plan.BundleDir, path), true
	default:
		return "", false
	}
}

// joinContext 拼接上下文根：ctx 可能是绝对路径，也可能相对仓库根。
func joinContext(repoRoot, ctx, path string) string {
	if filepath.IsAbs(ctx) {
		return filepath.Join(ctx, path)
	}
	return filepath.Join(repoRoot, ctx, path)
}

// splitFrom 拆分 "ctx /path" 形式，ctx 为空表示普通上下文。
func splitFrom(s string) (ctx, path string) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "", ""
	}
	if strings.HasPrefix(fields[0], "--from=") {
		return strings.TrimPrefix(fields[0], "--from="), fields[len(fields)-1]
	}
	return "", fields[len(fields)-1]
}

// copySources 从 Dockerfile 中提取 COPY/ADD 的源（保留 --from= 前缀）。
func copySources(file string, env map[string]string) []string {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var out []string
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := copyPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		fields := strings.Fields(m[2])
		if len(fields) < 2 {
			continue
		}
		src := expandVars(fields[len(fields)-2], env)
		if strings.HasPrefix(fields[0], "--from=") {
			src = fields[0] + " " + src
		}
		out = append(out, src)
	}
	return out
}

// expandVars 展开 ${VAR} 形式。
func expandVars(s string, env map[string]string) string {
	return varPattern.ReplaceAllStringFunc(s, func(m string) string {
		key := strings.TrimSuffix(strings.TrimPrefix(m, "${"), "}")
		if v, ok := env[key]; ok {
			return v
		}
		return m
	})
}
