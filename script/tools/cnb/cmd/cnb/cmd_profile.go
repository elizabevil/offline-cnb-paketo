package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/render"
	specprofile "learn-cnb-paketo/tools/cnb/internal/spec/profile"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// newProfileCmd — 流水线配置的生成、校验与展示。
func newProfileCmd() *cobra.Command {
	parent := &cobra.Command{Use: "profile", Short: "流水线配置：生成 / 校验 / 查看"}

	var (
		initDir      string
		initPlatform string
		initDistro   string
	)
	initCmd := &cobra.Command{
		Use: "init <name>", Short: "生成新项目配置模板", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			dir := initDir
			if dir == "" {
				dir = name
			}
			platform := initPlatform
			if platform == "" {
				platform = "linux-amd64"
			}
			out := filepath.Join("config", "projects", name+".toml")
			if _, err := os.Stat(out); err == nil {
				return &exitError{code: ExitFailure, msg: "already exists: " + out}
			}
			if err := os.MkdirAll(filepath.Dir(out), fsutil.DirMode); err != nil {
				return err
			}
			content, err := specprofile.Template(name, dir, platform, initDistro)
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			if err := os.WriteFile(out, []byte(content), fsutil.FileMode); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ 已生成 %s\n", out)
			return nil
		},
	}
	initCmd.Flags().StringVar(&initDir, "dir", "", "项目目录（相对仓库根，默认 = name）")
	initCmd.Flags().StringVar(&initPlatform, "platform", "", "默认平台（默认 linux-amd64）")
	initCmd.Flags().StringVar(&initDistro, "distro", "debian", "目标发行版：debian | ubuntu | alpine")
	parent.AddCommand(initCmd)

	parent.AddCommand(&cobra.Command{
		Use: "validate <file>", Short: "校验流水线配置", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			f, warns, err := specprofile.Parse(data)
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			failed := 0
			for _, w := range warns {
				fmt.Fprintf(cmd.OutOrStdout(), "  ! %s\n", w.String())
			}
			for _, e := range f.Validate() {
				failed++
				fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %v\n", e)
			}
			if failed > 0 {
				return &exitError{code: ExitFailure}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ 流水线合法：%s（%d 个物料，%d 个阶段，平台 %s）\n",
				f.Metadata.Name, len(f.Spec.Assets), len(f.Spec.Stages), f.Spec.Project.Platform)
			return nil
		},
	})

	var showFormat string
	showCmd := &cobra.Command{
		Use: "show <file>", Short: "结构化展示配置", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			f, _, err := specprofile.Parse(data)
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			out, err := render.FormatStruct(f, showFormat)
			if err != nil {
				return &exitError{code: ExitUsage, msg: err.Error()}
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	showCmd.Flags().StringVar(&showFormat, "format", "yaml", "output format: yaml | json")
	parent.AddCommand(showCmd)

	return parent
}

// assertWorkspace 保留 workspace 导入的显式使用（便于后续扩展）。
var _ = workspace.Open
