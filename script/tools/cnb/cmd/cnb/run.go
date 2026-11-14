package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/pipeline"
	"learn-cnb-paketo/tools/cnb/internal/profile"
	"learn-cnb-paketo/tools/cnb/internal/render"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// newPlanCmd 生成执行计划而不执行。
func newPlanCmd() *cobra.Command {
	var (
		format string
		full   bool
	)
	cmd := &cobra.Command{
		Use:   "plan <profile>",
		Short: "print the execution plan (shell / json / yaml)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := loadPlan(args[0])
			if err != nil {
				return err
			}
			if !full {
				out, err := render.CompactPlan(plan, format)
				if err != nil {
					return &exitError{code: ExitUsage, msg: err.Error()}
				}
				fmt.Fprint(cmd.OutOrStdout(), out)
				return nil
			}
			out, err := render.Format(plan, format)
			if err != nil {
				return &exitError{code: ExitUsage, msg: err.Error()}
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "yaml", "output format: yaml | json | table | text | shell")
	cmd.Flags().BoolVar(&full, "full", false, "print the full plan (assets, build args, stages detail)")
	return cmd
}

// newRunCmd 按配置执行流水线。
func newRunCmd() *cobra.Command {
	var (
		dryRun     bool
		only       string
		level      string
		format     string
		noCommands bool
	)
	cmd := &cobra.Command{
		Use:   "run <profile>",
		Short: "run the pipeline (materials → images → commands)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := newLogger(level)
			plan, err := loadPlan(args[0])
			if err != nil {
				return err
			}
			if dryRun {
				return printDryRun(cmd, plan, format, noCommands)
			}
			// 执行前打印精简计划（yaml/json），比单行属性更好读
			if out, err := render.CompactPlan(plan, format); err == nil {
				fmt.Fprint(cmd.OutOrStdout(), out)
			}
			root, err := resolveRepo()
			if err != nil {
				return err
			}
			_, l, platform, err := openProfile(args[0])
			if err != nil {
				return err
			}
			ctx, err := workspace.Open(workspace.Options{Repo: root})
			if err != nil {
				return err
			}
			bundleDir := ctx.Bundle
			runner := pipeline.New(pipeline.Options{
				Root:       root,
				BundleDir:  bundleDir,
				Lock:       l,
				Platform:   platform,
				Only:       only,
				NoCommands: noCommands,
			}, logger)
			if err := runner.Run(plan); err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan only")
	cmd.Flags().StringVar(&format, "format", "yaml", "dry-run output format: table | yaml | json | text | shell")
	cmd.Flags().StringVar(&only, "only", "", "run a single stage")
	cmd.Flags().StringVar(&level, "log-level", "info", "log level: debug|info|warn|error")
	cmd.Flags().BoolVar(&noCommands, "no-commands", false, "skip command stages")
	return cmd
}

// loadPlan 读取配置并生成执行计划。
func loadPlan(profilePath string) (*profile.Plan, error) {
	p, l, platform, err := openProfile(profilePath)
	if err != nil {
		return nil, err
	}
	plan, err := p.BuildPlan(platform, l)
	if err != nil {
		return nil, &exitError{code: ExitFailure, msg: err.Error()}
	}
	if abs, err := filepath.Abs(profilePath); err == nil {
		plan.ConfigPath = abs
	}
	return plan, nil
}

// printDryRun 输出计划而不执行。
func printDryRun(cmd *cobra.Command, plan *profile.Plan, format string, noCommands bool) error {
	if noCommands {
		fmt.Fprintln(os.Stderr, "  (--no-commands: command stages will be skipped)")
	}
	out, err := render.Format(plan, format)
	if err != nil {
		return &exitError{code: ExitUsage, msg: err.Error()}
	}
	fmt.Fprint(cmd.OutOrStdout(), out)
	return nil
}

// newLogger 构建紧凑日志器。
func newLogger(level string) *slog.Logger {
	var lv slog.Level
	switch level {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	return slog.New(render.NewCompactHandler(os.Stderr, lv))
}
