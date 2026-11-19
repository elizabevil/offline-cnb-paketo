// Package plan 把 profile 配置展开为可执行的阶段计划（绝对路径 + 最终 argv）。
package plan

import (
	"maps"
	"path/filepath"
	"slices"
	"time"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
	"learn-cnb-paketo/tools/cnb/internal/spec/profile"
	"learn-cnb-paketo/tools/cnb/internal/spec/warn"
)

// DefaultTimeout 是未声明 timeout 时的默认值。
const DefaultTimeout = 10 * time.Minute

// Stage is one resolved step.
type Stage struct {
	Name            string
	Argv            []string
	Workdir         string
	Env             []string
	Timeout         time.Duration
	ContinueOnError bool
}

// Plan 是一次执行的完整计划。
type Plan struct {
	Project  string
	Platform string
	Bundle   string
	Stages   []Stage
}

// Options 是构建计划所需的运行环境。
type Options struct {
	// Repo 是仓库根（绝对路径），workdir 与路径参数以它为基准。
	Repo string
	// Bundle 是物料目录（绝对路径），对应 ${bundle}。
	Bundle string
}

// Placeholders returns the supported variables and their resolved values.
func Placeholders(f *profile.File, lock *manifest.File, opts Options) ([]Placeholder, error) {
	v, err := newVars(f, lock, opts)
	if err != nil {
		return nil, err
	}
	return v.Table(), nil
}

// Build 展开占位符、绝对化路径，生成执行计划。
func Build(f *profile.File, lock *manifest.File, opts Options) (*Plan, []warn.Warning, error) {
	vars, err := newVars(f, lock, opts)
	if err != nil {
		return nil, nil, err
	}
	p := &Plan{Project: f.Metadata.Name, Platform: f.Spec.Project.Platform, Bundle: opts.Bundle}
	var allWarns []warn.Warning
	for _, st := range f.Spec.Stages {
		expanded, err := expandStage(st, vars, opts.Repo)
		if err != nil {
			return nil, nil, err
		}
		p.Stages = append(p.Stages, expanded)
	}
	return p, allWarns, nil
}

// expandStage 展开单个阶段。
func expandStage(st profile.Stage, vars vars, repo string) (Stage, error) {
	argv := make([]string, 0, len(st.Run))
	for _, arg := range st.Run {
		value, err := vars.expand(arg)
		if err != nil {
			return Stage{}, &errs.Invalid{Field: "spec.stages[" + st.Name + "].run", Value: arg, Reason: err.Error()}
		}
		argv = append(argv, value)
	}
	workdir, err := absPath(repo, st.Workdir)
	if err != nil {
		return Stage{}, err
	}
	timeout := DefaultTimeout
	if st.Timeout != "" {
		if timeout, err = time.ParseDuration(st.Timeout); err != nil {
			return Stage{}, &errs.Invalid{Field: "spec.stages[" + st.Name + "].timeout", Value: st.Timeout, Reason: "expect duration like 10m"}
		}
	}
	env := make([]string, 0, len(st.Env))
	for _, k := range slices.Sorted(maps.Keys(st.Env)) {
		env = append(env, k+"="+st.Env[k])
	}
	return Stage{
		Name: st.Name, Argv: argv, Workdir: workdir, Env: env,
		Timeout: timeout, ContinueOnError: st.ContinueOnError,
	}, nil
}

// absPath 把相对路径按仓库根绝对化。
func absPath(repo, p string) (string, error) {
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	joined := filepath.Join(repo, p)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", errs.Wrap("resolve path "+p, err)
	}
	return abs, nil
}
