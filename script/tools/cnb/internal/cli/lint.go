package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/plan"
)

// lintFinding 是一条 lint 结果。
type lintFinding struct {
	Stage   string
	Message string
	IsError bool
}

// newLintCmd 检查流水线的常见错误（尽力而为，不阻断执行）。
func newPipelineLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint <profile>",
		Short: "检查流水线的常见错误（构建参数、脚本存在性）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, err := loadPipeline(args[0])
			if err != nil {
				return err
			}
			var findings []lintFinding
			for _, st := range p.Stages {
				findings = append(findings, lintStage(st)...)
			}
			errsN := 0
			for _, f := range findings {
				mark := "  ⚠"
				if f.IsError {
					mark = "  ✗"
					errsN++
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", mark, f.Stage, f.Message)
			}
			if len(findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  ✓ 未发现问题")
			}
			if errsN > 0 {
				return &exitError{code: ExitFailure}
			}
			return nil
		},
	}
}

// lintStage 检查单个阶段。
func lintStage(st plan.Stage) []lintFinding {
	var out []lintFinding
	add := func(err bool, format string, a ...any) {
		out = append(out, lintFinding{Stage: st.Name, Message: fmt.Sprintf(format, a...), IsError: err})
	}
	if len(st.Argv) == 0 {
		add(true, "run 为空")
		return out
	}
	switch {
	case isContainerBuild(st.Argv):
		lintContainerBuild(st, add)
	case isShellScript(st.Argv):
		if !existsRel(st.Workdir, st.Argv[1]) {
			add(true, "脚本不存在: %s", st.Argv[1])
		}
	}
	return out
}

// isContainerBuild 判断是否为镜像构建命令。
func isContainerBuild(argv []string) bool {
	if len(argv) < 2 {
		return false
	}
	base := filepath.Base(argv[0])
	return (base == "podman" || base == "docker") && argv[1] == "build"
}

// isShellScript 判断是否为“解释器 + 脚本”形式。
func isShellScript(argv []string) bool {
	if len(argv) < 2 {
		return false
	}
	switch filepath.Base(argv[0]) {
	case "bash", "sh":
		return !strings.HasPrefix(argv[1], "-")
	}
	return false
}

// lintContainerBuild 检查构建命令的常见错误。
func lintContainerBuild(st plan.Stage, add func(bool, string, ...any)) {
	argv := st.Argv
	hasTag, hasFile := false, false
	for i := 2; i < len(argv); i++ {
		switch argv[i] {
		case "-t", "--tag":
			if i+1 < len(argv) && argv[i+1] != "" {
				hasTag = true
			}
		case "-f", "--file":
			if i+1 >= len(argv) {
				add(true, "%s 缺少参数", argv[i])
				continue
			}
			hasFile = true
			if !existsRel(st.Workdir, argv[i+1]) {
				add(true, "Dockerfile 不存在: %s", argv[i+1])
			}
		case "--build-context", "--build-contexts":
			if i+1 >= len(argv) {
				add(true, "%s 缺少参数", argv[i])
				continue
			}
			if _, path, ok := strings.Cut(argv[i+1], "="); ok && !existsRel(st.Workdir, path) {
				add(true, "构建上下文不存在: %s", path)
			}
		}
	}
	if !hasFile {
		add(false, "未指定 -f，将使用默认 Dockerfile")
	}
	if !hasTag {
		add(true, "缺少 -t 镜像标签")
	}
	last := argv[len(argv)-1]
	if !strings.HasPrefix(last, "-") && !existsRel(st.Workdir, last) {
		add(true, "构建上下文不存在: %s", last)
	}
}

// existsRel 判断相对路径（相对 workdir）是否存在。
func existsRel(workdir, p string) bool {
	if filepath.IsAbs(p) {
		_, err := os.Stat(p)
		return err == nil
	}
	_, err := os.Stat(filepath.Join(workdir, p))
	return err == nil
}
