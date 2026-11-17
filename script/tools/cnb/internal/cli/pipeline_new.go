package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/exec"
	"learn-cnb-paketo/tools/cnb/internal/plan"
	"learn-cnb-paketo/tools/cnb/internal/render"
	specprofile "learn-cnb-paketo/tools/cnb/internal/spec/profile"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// loadPipeline 解析流水线配置并构建执行计划所需上下文。
func loadPipeline(path string) (*specprofile.File, *plan.Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, &exitError{code: ExitFailure, msg: err.Error()}
	}
	f, warns, err := specprofile.Parse(data)
	if err != nil {
		return nil, nil, &exitError{code: ExitFailure, msg: err.Error()}
	}
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "  ! %s\n", w.String())
	}
	if errsOut := f.Validate(); len(errsOut) > 0 {
		for _, e := range errsOut {
			fmt.Fprintf(os.Stderr, "  ✗ %v\n", e)
		}
		return nil, nil, &exitError{code: ExitFailure}
	}
	ctx, err := workspace.Open(workspace.Options{Repo: opts.repo})
	if err != nil {
		return nil, nil, err
	}
	p, _, err := plan.Build(f, ctx.Lock, plan.Options{Repo: ctx.Root, Bundle: ctx.Bundle})
	if err != nil {
		return nil, nil, &exitError{code: ExitFailure, msg: err.Error()}
	}
	return f, p, nil
}

// newPlanCmd 打印执行计划（每个阶段的最终 argv）。
func newPlanCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "plan <profile>",
		Short: "打印执行计划（阶段 + 最终 argv）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, p, err := loadPipeline(args[0])
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if format == "json" {
				fmt.Fprintf(out, "{\"project\":%q,\"platform\":%q,\"stages\":[\n", f.Metadata.Name, p.Platform)
				for i, st := range p.Stages {
					sep := ","
					if i == len(p.Stages)-1 {
						sep = ""
					}
					fmt.Fprintf(out, "  {\"name\":%q,\"workdir\":%q,\"argv\":%q}%s\n", st.Name, st.Workdir, st.Argv, sep)
				}
				fmt.Fprintln(out, "]}")
				return nil
			}
			fmt.Fprintf(out, "project: %s\nplatform: %s\nbundle: %s\nstages:\n", f.Metadata.Name, p.Platform, p.Bundle)
			for i, st := range p.Stages {
				fmt.Fprintf(out, "  %d. %s\n     cd %s\n     %v\n", i+1, st.Name, st.Workdir, st.Argv)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text | json")
	return cmd
}

// newRunCmd 按计划执行阶段。
func newRunCmd() *cobra.Command {
	var (
		dryRun   bool
		from     string
		until    string
		logLevel string
	)
	cmd := &cobra.Command{
		Use:   "run <profile>",
		Short: "执行流水线（按 stages 顺序）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, err := loadPipeline(args[0])
			if err != nil {
				return err
			}
			logger := render.NewLogger(os.Stderr, logLevel)
			if dryRun {
				_, err := exec.Run(context.Background(), p, exec.Options{DryRun: true}, logger)
				return err
			}
			if _, err := exec.Run(context.Background(), p, exec.Options{From: from, Until: until}, logger); err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			logger.Info("pipeline done", "project", p.Project)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只打印命令，不执行")
	cmd.Flags().StringVar(&from, "from", "", "从指定阶段开始（跳过之前阶段）")
	cmd.Flags().StringVar(&until, "until", "", "执行到指定阶段（含）为止")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "log level: debug|info|warn|error")
	return cmd
}
