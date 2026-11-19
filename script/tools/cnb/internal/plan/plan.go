// Package plan 把 profile 配置展开为可执行的阶段计划（绝对路径 + 最终 argv）。
package plan

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"time"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
	"learn-cnb-paketo/tools/cnb/internal/spec/profile"
)

// DefaultTimeout 是未声明 timeout 时的默认值。
const DefaultTimeout = 10 * time.Minute

// Stage is one resolved step.
type Stage struct {
	Name string
	// Description is informational only.
	Description     string
	Argv            []string
	Workdir         string
	Env             []string
	Timeout         time.Duration
	ContinueOnError bool
}

// Plan is a full execution plan.
type Plan struct {
	Project  string
	Platform string
	Bundle   string
	Stages   []Stage
	// Dockerfiles are resolved spec.checks targets (absolute paths).
	Dockerfiles []DockerfileTarget
}

// DockerfileTarget is one Dockerfile plus the contexts its COPY uses.
type DockerfileTarget struct {
	Name       string
	Dockerfile string
	Context    string
	Contexts   map[string]string
	Args       map[string]string
}

// Options are the resolved roots a plan is built from.
type Options struct {
	// Project is the directory holding the pipeline config (absolute).
	// Stage workdir and project-local paths resolve against it.
	Project string
	// Materials is the directory holding the manifest (absolute).
	Materials string
	// Repo is the workspace root (absolute), used for shared assets
	// such as buildpacks.
	Repo string
	// Bundle is the material directory (absolute).
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
func Build(f *profile.File, lock *manifest.File, opts Options) (*Plan, []errs.Warning, error) {
	vars, err := newVars(f, lock, opts)
	if err != nil {
		return nil, nil, err
	}
	p := &Plan{Project: f.Metadata.Name, Platform: f.Spec.Project.Platform, Bundle: opts.Bundle}
	var allWarns []errs.Warning
	for _, st := range f.Spec.Stages {
		expanded, err := expandStage(st, vars, opts.Project)
		if err != nil {
			return nil, nil, err
		}
		p.Stages = append(p.Stages, expanded)
	}
	targets, err := expandChecks(f, vars, opts.Project, opts.Repo)
	if err != nil {
		return nil, nil, err
	}
	p.Dockerfiles = targets
	return p, allWarns, nil
}

// expandStage 展开单个阶段。
func expandStage(st profile.Stage, vars vars, project string) (Stage, error) {
	argv := make([]string, 0, len(st.Run))
	for _, arg := range st.Run {
		value, err := vars.expand(arg)
		if err != nil {
			return Stage{}, &errs.Invalid{Field: "spec.stages[" + st.Name + "].run", Value: arg, Reason: err.Error()}
		}
		argv = append(argv, value)
	}
	workdir, err := absPath(project, st.Workdir)
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
		Name: st.Name, Description: st.Description, Argv: argv, Workdir: workdir, Env: env,
		Timeout: timeout, ContinueOnError: st.ContinueOnError,
	}, nil
}

// expandChecks resolves spec.checks.dockerfiles with the same placeholder table
// as stages, so ${repo} / ${bundle} mean exactly what they mean in run.
//
// path and context are relative to the project root (the config directory);
// contexts are relative to the workspace root (shared assets live there).
func expandChecks(f *profile.File, v vars, project, repo string) ([]DockerfileTarget, error) {
	if len(f.Spec.Checks.Dockerfiles) == 0 {
		return nil, nil
	}
	projectDir := project
	out := make([]DockerfileTarget, 0, len(f.Spec.Checks.Dockerfiles))
	for i, dc := range f.Spec.Checks.Dockerfiles {
		field := fmt.Sprintf("spec.checks.dockerfiles[%d]", i)
		pathValue, err := v.expand(dc.Path)
		if err != nil {
			return nil, &errs.Invalid{Field: field + ".path", Value: dc.Path, Reason: err.Error()}
		}
		dockerfile := joinAbs(projectDir, pathValue)

		contextDir := projectDir
		if dc.Context != "" {
			contextValue, err := v.expand(dc.Context)
			if err != nil {
				return nil, &errs.Invalid{Field: field + ".context", Value: dc.Context, Reason: err.Error()}
			}
			contextDir = joinAbs(projectDir, contextValue)
		}

		contexts := make(map[string]string, len(dc.Contexts))
		for _, name := range slices.Sorted(maps.Keys(dc.Contexts)) {
			value, err := v.expand(dc.Contexts[name])
			if err != nil {
				return nil, &errs.Invalid{Field: field + ".contexts." + name, Value: dc.Contexts[name], Reason: err.Error()}
			}
			contexts[name] = joinAbs(repo, value)
		}
		args := make(map[string]string, len(dc.Args))
		for _, name := range slices.Sorted(maps.Keys(dc.Args)) {
			value, err := v.expand(dc.Args[name])
			if err != nil {
				return nil, &errs.Invalid{Field: field + ".args." + name, Value: dc.Args[name], Reason: err.Error()}
			}
			args[name] = value
		}
		out = append(out, DockerfileTarget{Name: dc.Path, Dockerfile: dockerfile, Context: contextDir, Contexts: contexts, Args: args})
	}
	return out, nil
}

// joinAbs resolves p against base unless p is already absolute.
func joinAbs(base, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(base, p)
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
