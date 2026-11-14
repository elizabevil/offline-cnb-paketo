package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"learn-cnb-paketo/tools/cnb/internal/bundle"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/profile"
	"os"
	"path/filepath"
	"strings"
)

func newProfileCmd() *cobra.Command {
	parent := &cobra.Command{Use: "profile", Short: "流水线配置：校验与渲染"}

	var (
		initDir      string
		initPlatform string
		initScaffold bool
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
			out := mustProfilePath(name)
			if _, err := os.Stat(out); err == nil {
				return &exitError{code: 1, msg: fmt.Sprintf("已存在，拒绝覆盖: %s", out)}
			}
			if err := os.MkdirAll(filepath.Dir(out), fsutil.DirMode); err != nil {
				return err
			}
			if err := os.WriteFile(out, []byte(profile.Template(name, dir, platform)), fsutil.FileMode); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ 已生成 %s\n", out)

			if initScaffold {
				root, _ := resolveRepo()
				files, err := profile.Scaffold(name, dir)
				if err != nil {
					return err
				}
				for _, f := range files {
					target := root + "/" + f.Path
					if _, err := os.Stat(target); err == nil {
						fmt.Fprintf(cmd.OutOrStdout(), "  = 已存在，跳过 %s\n", f.Path)
						continue
					}
					if err := os.MkdirAll(filepath.Dir(target), fsutil.DirMode); err != nil {
						return err
					}
					if err := os.WriteFile(target, []byte(f.Content), os.FileMode(f.Mode)); err != nil {
						return err
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", f.Path)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  下一步: 按需修改 [assets]/[images]，再执行 cnb profile validate %s\n", out)
			return nil
		},
	}
	initCmd.Flags().StringVar(&initDir, "dir", "", "项目目录（相对仓库根，默认 = name）")
	initCmd.Flags().StringVar(&initPlatform, "platform", "", "默认平台（默认 linux-amd64）")
	initCmd.Flags().BoolVar(&initScaffold, "scaffold", false, "同时生成 docker/ 与 build.sh 骨架")
	parent.AddCommand(initCmd)

	parent.AddCommand(&cobra.Command{
		Use: "validate <file>", Short: "校验流水线配置", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			p, l, platform, err := openProfile(args[0])
			if err != nil {
				return err
			}
			if errs := p.Validate(platform, l); len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintf(os.Stderr, "  ✗ %s\n", e)
				}
				return &exitError{code: 1}
			}
			fmt.Printf("  ✓ 流水线合法：%s（%d 个物料，%d 个镜像，平台 %s）\n",
				p.Name(), len(p.Assets), len(p.Images), platform)
			return nil
		},
	})

	parent.AddCommand(&cobra.Command{
		Use: "verify <file>", Short: "校验该流程用到的物料（哈希 / marker / 产物）", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			p, l, platform, err := openProfile(args[0])
			if err != nil {
				return err
			}
			if errs := p.Validate(platform, l); len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintf(os.Stderr, "  ✗ %s\n", e)
				}
				return &exitError{code: 1}
			}
			_, _, bundleDir, _ := open()
			rep, err := bundle.Check(l, bundleDir, platform, p.Refs(platform))
			if err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			if !printReport(rep, opts.strict, opts.quiet) {
				return &exitError{code: 1}
			}
			return nil
		},
	})

	parent.AddCommand(&cobra.Command{
		Use: "assets <file>", Short: "列出该流程用到的物料引用（id@version）", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, l, platform, err := openProfile(args[0])
			if err != nil {
				return err
			}
			_ = l
			fmt.Fprintln(cmd.OutOrStdout(), strings.Join(p.Refs(platform), " "))
			return nil
		},
	})

	parent.AddCommand(&cobra.Command{
		Use: "show <file>", Short: "分组展示配置（不平铺）", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := profile.Load(args[0])
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), p.Show())
			return nil
		},
	})

	parent.AddCommand(&cobra.Command{
		Use: "render <file>", Short: "渲染为可执行 shell 片段", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, l, platform, err := openProfile(args[0])
			if err != nil {
				return err
			}
			out, err := p.Render(platform, l)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ %v\n", err)
				return &exitError{code: 1}
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	})
	return parent
}
