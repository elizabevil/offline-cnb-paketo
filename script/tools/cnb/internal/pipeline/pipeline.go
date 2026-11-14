// Package pipeline 按执行计划依次运行物料、镜像与命令阶段。
package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"learn-cnb-paketo/tools/cnb/internal/checks"
	"learn-cnb-paketo/tools/cnb/internal/profile"

	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// 阶段类型与外部脚本路径。
const (
	KindImage   = lock.KindImage
	KindCommand = lock.KindCommand
	KindFetch   = lock.KindFetch

	scriptFetch = "script/fetch.sh"
	scriptImage = "script/lib/image-build.sh"

	contextBundle     = lock.ContextBundle
	contextBuildpacks = lock.ContextBuildpacks

	flagBuildContext = "--build-context"
	flagBuildArg     = "--build-arg"
	flagLabel        = "--label"

	labelBuilderMeta = "io.buildpacks.builder.metadata="
	labelStackID     = "io.buildpacks.stack.id="
)

// Options 是一次执行的选项。
type Options struct {
	Root       string
	Only       string
	NoCommands bool
}

// Runner 执行计划。
type Runner struct {
	opts Options
	log  *slog.Logger
}

// New 创建执行器。
func New(opts Options, log *slog.Logger) *Runner {
	return &Runner{opts: opts, log: log}
}

// Run 执行整个计划。
func (r *Runner) Run(plan *profile.Plan) error {
	if len(plan.Assets) > 0 && r.selected(KindFetch) {
		if err := r.fetch(plan); err != nil {
			return err
		}
	}
	for i, st := range plan.Stages {
		if err := r.runStage(plan, i, st); err != nil {
			return err
		}
	}
	r.log.Info("pipeline done", "project", plan.Project)
	return nil
}

// selected 判断某阶段是否应执行。
func (r *Runner) selected(name string) bool {
	return r.opts.Only == "" || r.opts.Only == name
}

// fetch 拉取计划用到的物料。
func (r *Runner) fetch(plan *profile.Plan) error {
	start := time.Now()
	r.log.Info("fetch materials", "count", len(plan.Assets))
	args := append([]string{"fetch", "--platform", plan.Platform}, plan.Assets...)
	if err := r.exec(r.opts.Root, os.Environ(), filepath.Join(r.opts.Root, scriptFetch), args); err != nil {
		r.log.Error("fetch failed", "err", err)
		return fmt.Errorf("material fetch failed: %w", err)
	}
	r.log.Info("fetch done", "took", time.Since(start).Round(time.Millisecond))
	return nil
}

// runStage 执行单个阶段（含跳过规则）。
func (r *Runner) runStage(plan *profile.Plan, idx int, st profile.PlanStage) error {
	total := len(plan.Stages)
	if r.opts.NoCommands && st.Kind == KindCommand {
		r.log.Info("stage skipped", "stage", st.Name, "kind", st.Kind, "reason", "no-commands", "index", idx+1, "total", total)
		return nil
	}
	if !r.selected(st.Name) {
		r.log.Info("stage skipped", "stage", st.Name, "kind", st.Kind, "index", idx+1, "total", total)
		return nil
	}
	r.log.Info("stage start", "stage", st.Name, "kind", st.Kind, "index", idx+1, "total", total)

	start := time.Now()
	err := r.dispatch(plan, st)
	if err != nil {
		r.log.Error("stage failed", "stage", st.Name, "took", time.Since(start).Round(time.Millisecond))
		r.log.Info("retry hint", "cmd", "cnb pipeline run "+plan.ConfigPath+" --only "+st.Name)
		return fmt.Errorf("stage failed: %s: %w", st.Name, err)
	}
	r.log.Info("stage done", "stage", st.Name, "took", time.Since(start).Round(time.Millisecond))
	return nil
}

// dispatch 按阶段类型选择执行方式。
func (r *Runner) dispatch(plan *profile.Plan, st profile.PlanStage) error {
	switch st.Kind {
	case KindImage:
		return r.buildImage(plan, st)
	case KindCommand:
		return r.runCommand(st)
	default:
		return nil
	}
}

// buildImage 调用镜像构建脚本。
func (r *Runner) buildImage(plan *profile.Plan, st profile.PlanStage) error {
	args := []string{plan.Context, st.Dockerfile, st.Tag, flagBuildContext, contextBundle + "=" + st.BundleCtx}
	if st.BuildpacksCtx != "" {
		args = append(args, flagBuildContext, contextBuildpacks+"="+st.BuildpacksCtx)
	}
	args = append(args, r.builderLabels(plan, st)...)
	for _, k := range slices.Sorted(maps.Keys(st.Args)) {
		args = append(args, flagBuildArg, k+"="+st.Args[k])
	}
	return r.exec(r.opts.Root, os.Environ(), filepath.Join(r.opts.Root, scriptImage), args)
}

// builderLabels 为 builder 镜像生成 metadata 与 stack.id 标签。
func (r *Runner) builderLabels(plan *profile.Plan, st profile.PlanStage) []string {
	if !strings.Contains(st.Name, "builder") {
		return nil
	}
	var args []string
	if m, err := checks.Generate(plan); err == nil {
		if js, err := m.JSON(); err == nil {
			args = append(args, flagLabel, labelBuilderMeta+js)
		}
	}
	if sid := checks.StackID(plan); sid != "" {
		args = append(args, flagLabel, labelStackID+sid)
	}
	return args
}

// runCommand 执行自定义命令阶段。
func (r *Runner) runCommand(st profile.PlanStage) error {
	env := os.Environ()
	for k, v := range st.Env {
		env = append(env, k+"="+v)
	}
	return r.exec(st.Workdir, env, "bash", []string{"-c", st.Run})
}

// exec 运行外部命令并转发输出。
func (r *Runner) exec(dir string, env []string, name string, args []string) error {
	r.log.Debug("exec", "cmd", name, "args", args, "dir", dir)
	c := exec.CommandContext(context.Background(), name, args...)
	c.Dir = dir
	c.Env = env
	c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
	return c.Run()
}
