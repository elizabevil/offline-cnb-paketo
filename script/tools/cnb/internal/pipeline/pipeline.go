// Package pipeline 按执行计划依次运行物料、镜像与命令阶段。
package pipeline

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"learn-cnb-paketo/tools/cnb/internal/buildmeta"
	"learn-cnb-paketo/tools/cnb/internal/profile"

	"learn-cnb-paketo/tools/cnb/internal/cnbconst"
)

// 阶段类型与外部脚本路径。
const (
	KindImage   = cnbconst.KindImage
	KindCommand = cnbconst.KindCommand
	KindFetch   = cnbconst.KindFetch

	scriptFetch = "script/fetch.sh"
	scriptImage = "script/lib/image-build.sh"

	contextBundle     = cnbconst.ContextBundle
	contextBuildpacks = cnbconst.ContextBuildpacks

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
	Verbose    bool
	Out        io.Writer
}

// Runner 执行计划。
type Runner struct {
	opts Options
	log  *slog.Logger
	out  io.Writer
}

// New 创建执行器。
func New(opts Options, log *slog.Logger) *Runner {
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	return &Runner{opts: opts, log: log, out: out}
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
	fmt.Fprintf(r.out, "\n✓ pipeline done: %s\n", plan.Project)
	return nil
}

// selected 判断某阶段是否应执行。
func (r *Runner) selected(name string) bool {
	return r.opts.Only == "" || r.opts.Only == name
}

// fetch 拉取计划用到的物料。
func (r *Runner) fetch(plan *profile.Plan) error {
	start := time.Now()
	fmt.Fprintf(r.out, "\n[fetch] materials (%d)\n", len(plan.Assets))
	args := append([]string{"fetch", "--platform", plan.Platform}, plan.Assets...)
	if err := r.exec(r.opts.Root, os.Environ(), filepath.Join(r.opts.Root, scriptFetch), args); err != nil {
		fmt.Fprintf(r.out, "        ✗ fetch (%v)\n", err)
		return fmt.Errorf("material fetch failed: %w", err)
	}
	fmt.Fprintf(r.out, "        ✓ fetch  %s\n", time.Since(start).Round(time.Millisecond))
	return nil
}

// runStage 执行单个阶段（含跳过规则）。
func (r *Runner) runStage(plan *profile.Plan, idx int, st profile.PlanStage) error {
	total := len(plan.Stages)
	if r.opts.NoCommands && st.Kind == KindCommand {
		fmt.Fprintf(r.out, "\n[%d/%d] %-7s %s  – skipped (--no-commands)\n", idx+1, total, st.Kind, st.Name)
		return nil
	}
	if !r.selected(st.Name) {
		fmt.Fprintf(r.out, "\n[%d/%d] %-7s %s  – skipped\n", idx+1, total, st.Kind, st.Name)
		return nil
	}
	fmt.Fprintf(r.out, "\n[%d/%d] %-7s %s\n", idx+1, total, st.Kind, st.Name)

	start := time.Now()
	err := r.dispatch(plan, st)
	if err != nil {
		fmt.Fprintf(r.out, "        ✗ %s  %s\n", st.Name, time.Since(start).Round(time.Millisecond))
		fmt.Fprintf(r.out, "        retry: cnb run %s --only %s\n", plan.ConfigPath, st.Name)
		return fmt.Errorf("stage failed: %s: %w", st.Name, err)
	}
	fmt.Fprintf(r.out, "        ✓ %s  %s\n", st.Name, time.Since(start).Round(time.Millisecond))
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
	for _, k := range sortedKeys(st.Args) {
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
	if m, err := buildmeta.Generate(plan); err == nil {
		if js, err := m.JSON(); err == nil {
			args = append(args, flagLabel, labelBuilderMeta+js)
		}
	}
	if sid := buildmeta.StackID(plan); sid != "" {
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
	c := execCommand(name, args...)
	c.Dir = dir
	c.Env = env
	c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
	return c.Run()
}

// sortedKeys 返回排序后的 map 键。
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
