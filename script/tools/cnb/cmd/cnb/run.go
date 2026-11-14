package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/buildmeta"
	"learn-cnb-paketo/tools/cnb/internal/profile"
	"learn-cnb-paketo/tools/cnb/internal/render"
)

func newPlanCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "plan <profile>",
		Short: "生成执行计划（shell / json）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, l, platform, err := openProfile(args[0])
			if err != nil {
				return err
			}
			plan, err := p.BuildPlan(platform, l)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ %v\n", err)
				return &exitError{code: 1}
			}
			out, err := render.Format(plan, format)
			if err != nil {
				return &exitError{code: 2, msg: err.Error()}
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "shell", "输出格式：text | shell | json | yaml")
	return cmd
}

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
		Short: "按配置执行流水线（物料 → 镜像 → 命令）",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			logger := newLogger(level)
			p, l, platform, err := openProfile(args[0])
			if err != nil {
				return err
			}
			plan, err := p.BuildPlan(platform, l)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ %v\n", err)
				return &exitError{code: 1}
			}
			if abs, err := filepath.Abs(args[0]); err == nil {
				plan.ConfigPath = abs
			}
			if dryRun {
				if noCommands {
					fmt.Fprintln(os.Stderr, "  （--no-commands：command 阶段不会执行）")
				}
				out, err := render.Format(plan, format)
				if err != nil {
					return &exitError{code: 2, msg: err.Error()}
				}
				fmt.Fprint(os.Stdout, out)
				return nil
			}
			if text, err := render.Text(plan); err == nil {
				fmt.Print(text)
			}
			return executePlan(logger, plan, only, noCommands)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只打印计划，不执行")
	cmd.Flags().StringVar(&format, "format", "text", "dry-run 输出格式：text | shell | json | yaml")
	cmd.Flags().StringVar(&only, "only", "", "只执行指定阶段")
	cmd.Flags().StringVar(&level, "log-level", "info", "日志级别：debug|info|warn|error")
	cmd.Flags().BoolVar(&noCommands, "no-commands", false, "跳过 command 阶段（只构建镜像）")
	return cmd
}

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

func executePlan(log *slog.Logger, plan *profile.Plan, only string, noCommands bool) error {
	root, err := resolveRepo()
	if err != nil {
		return err
	}
	ctx := plan.Context
	total := len(plan.Stages)
	if only != "" {
		fmt.Printf("\nonly=%s（其余阶段跳过）\n", only)
	}

	if len(plan.Assets) > 0 && (only == "" || only == "fetch") {
		start := time.Now()
		fmt.Printf("\n[fetch] materials (%d)\n", len(plan.Assets))
		args := append([]string{"fetch", "--platform", plan.Platform}, plan.Assets...)
		if err := runCmd(root, os.Environ(), filepath.Join(root, "script/fetch.sh"), args, log); err != nil {
			fmt.Printf("        ✗ fetch (%v)\n", err)
			return &exitError{code: 1, msg: "material fetch failed"}
		}
		fmt.Printf("        ✓ fetch  %s\n", time.Since(start).Round(time.Millisecond))
	}

	for i, st := range plan.Stages {
		if noCommands && st.Kind == "command" {
			fmt.Printf("\n[%d/%d] %-7s %s  – skipped (--no-commands)\n", i+1, total, st.Kind, st.Name)
			continue
		}
		if only != "" && only != st.Name {
			fmt.Printf("\n[%d/%d] %-7s %s  – skipped\n", i+1, total, st.Kind, st.Name)
			continue
		}
		fmt.Printf("\n[%d/%d] %-7s %s\n", i+1, total, st.Kind, st.Name)
		start := time.Now()
		var err error
		switch st.Kind {
		case "image":
			keys := make([]string, 0, len(st.Args))
			for k := range st.Args {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			args := []string{ctx, st.Dockerfile, st.Tag, "--build-context", "bundle=" + st.BundleCtx}
			// builder 镜像：用生成的 metadata/stack 覆盖 Dockerfile 里的静态标签
			if strings.Contains(st.Name, "builder") {
				if m, err := buildmeta.Generate(plan); err == nil {
					if js, err := m.JSON(); err == nil {
						args = append(args, "--label", "io.buildpacks.builder.metadata="+js)
					}
				}
				if sid := buildmeta.StackID(plan); sid != "" {
					args = append(args, "--label", "io.buildpacks.stack.id="+sid)
				}
			}
			for _, k := range keys {
				args = append(args, "--build-arg", k+"="+st.Args[k])
			}
			err = runCmd(root, os.Environ(), filepath.Join(root, "script/lib/image-build.sh"), args, log)
		case "command":
			env := os.Environ()
			for k, v := range st.Env {
				env = append(env, k+"="+v)
			}
			err = runCmd(st.Workdir, env, "bash", []string{"-c", st.Run}, log)
		}
		if err != nil {
			fmt.Printf("        ✗ %s  %s\n", st.Name, time.Since(start).Round(time.Millisecond))
			fmt.Printf("        retry: cnb run %s --only %s\n", plan.ConfigPath, st.Name)
			return &exitError{code: 1, msg: "stage failed: " + st.Name}
		}
		fmt.Printf("        ✓ %s  %s\n", st.Name, time.Since(start).Round(time.Millisecond))
	}
	fmt.Printf("\n✓ pipeline done: %s\n", plan.Project)
	return nil
}

func runCmd(dir string, env []string, name string, args []string, log *slog.Logger) error {
	log.Debug("exec", "cmd", name, "args", args, "dir", dir)
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Env = env
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}
