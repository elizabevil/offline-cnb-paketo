// Package exec 按计划顺序执行阶段。
package exec

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/plan"
)

// Options 是执行选项。
type Options struct {
	// From 从指定阶段开始（跳过之前的阶段）；为空表示从头执行。
	From string
	// DryRun 只打印命令，不执行。
	DryRun bool
	// Out 是命令输出目标；为空则写 stdout/stderr。
	Out *os.File
}

// Result 是单个阶段的执行结果。
type Result struct {
	Stage    string
	Duration time.Duration
	Skipped  bool
	Err      error
}

// Run 按计划顺序执行阶段，返回每个阶段的结果。
// 任一阶段失败且未声明 continueOnError 时立即返回错误。
func Run(ctx context.Context, p *plan.Plan, opts Options, log *slog.Logger) ([]Result, error) {
	startIdx, err := startIndex(p, opts.From)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(p.Stages))
	failed := 0
	for i, st := range p.Stages {
		if i < startIdx {
			results = append(results, Result{Stage: st.Name, Skipped: true})
			log.Info("stage skipped", "stage", st.Name, "reason", "before --from")
			continue
		}
		res := runStage(ctx, st, opts, log)
		results = append(results, res)
		if res.Err != nil {
			failed++
			log.Error("stage failed", "stage", st.Name, "err", res.Err)
			if !st.ContinueOnError {
				return results, fmt.Errorf("stage %s failed: %w", st.Name, res.Err)
			}
		}
	}
	if failed > 0 {
		return results, fmt.Errorf("%d stage(s) failed", failed)
	}
	return results, nil
}

// startIndex 计算 --from 对应的起始下标。
func startIndex(p *plan.Plan, from string) (int, error) {
	if from == "" {
		return 0, nil
	}
	for i, st := range p.Stages {
		if st.Name == from {
			return i, nil
		}
	}
	names := make([]string, 0, len(p.Stages))
	for _, st := range p.Stages {
		names = append(names, st.Name)
	}
	return 0, &errs.NotFound{Kind: "stage", Ref: from, Known: names}
}

// runStage 执行单个阶段。
func runStage(ctx context.Context, st plan.Stage, opts Options, log *slog.Logger) Result {
	start := time.Now()
	res := Result{Stage: st.Name}
	if len(st.Argv) == 0 {
		res.Err = &errs.Missing{What: "stage argv"}
		return res
	}
	if opts.DryRun {
		fmt.Fprintf(os.Stdout, "%s (cd %s): %v\n", st.Name, st.Workdir, st.Argv)
		res.Duration = time.Since(start)
		return res
	}

	cctx, cancel := context.WithTimeout(ctx, st.Timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, st.Argv[0], st.Argv[1:]...)
	cmd.Dir = st.Workdir
	cmd.Env = append(os.Environ(), st.Env...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if opts.Out != nil {
		cmd.Stdout = opts.Out
	}
	log.Info("stage start", "stage", st.Name, "argv", len(st.Argv), "workdir", st.Workdir)
	if err := cmd.Run(); err != nil {
		res.Err = errs.Wrap("run "+st.Name, err)
	}
	res.Duration = time.Since(start)
	return res
}
