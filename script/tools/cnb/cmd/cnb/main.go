// cnb — 基座小工具的命令行入口。
//
// CLI 层（cobra、flag、输出格式）只在这里；业务逻辑在 internal/：
//
//	internal/lock     物料清单解析与校验
//	internal/bundle   物料落盘状态校验 / 哈希固定
//	internal/profile  流水线配置（校验 + 渲染）
//	internal/repack   SmartPackage 规范化
//	internal/shell    shell 字面量转义
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/bundle"
	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/profile"
	"learn-cnb-paketo/tools/cnb/internal/repack"
	"learn-cnb-paketo/tools/cnb/internal/version"
)

// 版本由 internal/version 的 buildinfo 提供

type options struct {
	lockPath  string
	bundle    string
	platform  string
	strict    bool
	quiet     bool
	repo      string
	configDir string
}

// exitError 携带特定退出码；msg 为空表示错误已由调用方打印。
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string {
	if e.msg == "" {
		return fmt.Sprintf("exit %d", e.code)
	}
	return e.msg
}

var opts options

func main() {
	if err := newRootCmd().Execute(); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			if ee.msg != "" {
				fmt.Fprintf(os.Stderr, "cnb: %s\n", ee.msg)
			}
			os.Exit(ee.code)
		}
		fmt.Fprintf(os.Stderr, "cnb: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "cnb",
		Short: "基座小工具：lock / profile / verify / pin / repack",
		Long: "cnb 读取 assets.lock.toml 与 config/projects/*.toml，\n" +
			"提供物料解析、流水线渲染与 SmartPackage 规范化能力。",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.Get().Short(),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveRepo()
			if err != nil {
				return err
			}
			profile.SetRepoRoot(root)
			return nil
		},
	}
	root.SetVersionTemplate("cnb {{.Version}}\n")

	pf := root.PersistentFlags()
	pf.StringVar(&opts.lockPath, "file", "", "lock 文件路径（默认 <exe>/../../assets.lock.toml 或 LOCK_FILE）")
	pf.StringVar(&opts.bundle, "bundle", "", "物料目录（默认 <lock 所在目录>/bundle）")
	pf.StringVar(&opts.platform, "platform", "", "目标平台：linux-amd64 | linux-arm64")
	pf.BoolVar(&opts.strict, "strict", false, "verify 时未固定 sha256 也算失败")
	pf.BoolVar(&opts.quiet, "quiet", false, "verify 仅输出失败项")
	pf.StringVar(&opts.repo, "repo", "", "仓库根（默认 $CNB_REPO_ROOT 或由可执行文件位置推导）")
	pf.StringVar(&opts.configDir, "config-dir", "config", "配置目录（相对仓库根）")

	root.AddCommand(
		newListCmd(), newGetCmd(), newValidateCmd(), newPlatformsCmd(),
		newVerifyCmd(), newPinCmd(), newRepackCmd(), newProfileCmd(), newLockCmd(),
		newPlanCmd(), newRunCmd(), newDockerfileCmd(), newBuilderCmd(), newDoctorCmd(), newBuildpackCmd(), newPublishCmd(), newLifecycleCmd(), newVersionCmd(),
	)
	return root
}

// ── 公共上下文 ──────────────────────────────────────────────────────────────

func open() (*lock.File, string, string, error) {
	path, err := resolveLockPath(opts.lockPath)
	if err != nil {
		return nil, "", "", err
	}
	l, err := lock.Load(path)
	if err != nil {
		return nil, "", "", err
	}
	platform := opts.platform
	if platform == "" {
		platform = hostPlatform()
	}
	bundleDir := opts.bundle
	if bundleDir == "" {
		bundleDir = filepathDir(path) + "/bundle"
	}
	return l, platform, bundleDir, nil
}

// ── 子命令 ──────────────────────────────────────────────────────────────────

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "物料清单（TSV：id version kind platforms）", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, _, _, err := open()
			if err != nil {
				return err
			}
			l.List(cmd.OutOrStdout())
			return nil
		},
	}
}

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "get <id|id@version> <platform>", Short: "输出 shell 赋值语句（供 fetch.sh eval）",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, _, _, err := open()
			if err != nil {
				return err
			}
			if err := l.Get(cmd.OutOrStdout(), args[0], args[1]); err != nil {
				return &exitError{code: 2, msg: err.Error()}
			}
			return nil
		},
	}
}

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use: "validate", Short: "校验 lock 结构", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			l, _, _, err := open()
			if err != nil {
				return err
			}
			if l.Validate() > 0 {
				return &exitError{code: 1}
			}
			return nil
		},
	}
}

func newPlatformsCmd() *cobra.Command {
	return &cobra.Command{
		Use: "platforms", Short: "平台矩阵（逗号分隔）", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, _, _, err := open()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.Join(l.Platforms(), ","))
			return nil
		},
	}
}

func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use: "verify [ids...]", Short: "校验归档哈希、解压标记与预期产物",
		RunE: func(_ *cobra.Command, ids []string) error {
			l, platform, bundleDir, err := open()
			if err != nil {
				return err
			}
			if err := bundle.Verify(l, bundleDir, platform, ids, opts.strict, opts.quiet); err != nil {
				return &exitError{code: 1}
			}
			return nil
		},
	}
}

func newPinCmd() *cobra.Command {
	return &cobra.Command{
		Use: "pin [ids...]", Short: "由本地归档反推 sha256（只打印，不写回）",
		RunE: func(_ *cobra.Command, ids []string) error {
			l, platform, bundleDir, err := open()
			if err != nil {
				return err
			}
			if err := bundle.Pin(l, bundleDir, platform, ids); err != nil {
				return &exitError{code: 1}
			}
			return nil
		},
	}
}

func newRepackCmd() *cobra.Command {
	return &cobra.Command{
		Use: "repack <SmartPackage 目录 | *.tar.gz>", Short: "把 docker-save 风格的 web.tgz 规范化为 OCI layout",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := repack.Run(args[0]); err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			return nil
		},
	}
}

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
			if err := os.MkdirAll(filepathDir(out), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(out, []byte(profile.Template(name, dir, platform)), 0o644); err != nil {
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
					if err := os.MkdirAll(filepathDir(target), 0o755); err != nil {
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
			if err := bundle.Verify(l, bundleDir, platform, p.Refs(platform), opts.strict, opts.quiet); err != nil {
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

func newLockCmd() *cobra.Command {
	parent := &cobra.Command{Use: "lock", Short: "lock 维护"}
	parent.AddCommand(&cobra.Command{
		Use: "set-sha <ref> <platform> <sha256>", Short: "写入/校验某平台哈希", Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveLockPath(opts.lockPath)
			if err != nil {
				return err
			}
			changed, err := lock.SetSHA(path, args[0], args[1], args[2])
			if err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			if changed {
				fmt.Fprintf(cmd.OutOrStdout(), "  + 已固定 %s[%s]\n", args[0], args[1])
			}
			return nil
		},
	})
	return parent
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use: "version", Short: "版本信息", Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version.Get().String())
		},
	}
}

// openProfile 载入 profile 与 lock，并解析平台。
func openProfile(file string) (*profile.File, *lock.File, string, error) {
	l, platform, _, err := open()
	if err != nil {
		return nil, nil, "", err
	}
	p, err := profile.Load(file)
	if err != nil {
		return nil, nil, "", err
	}
	if opts.platform == "" && p.DefaultPlatform() != "" {
		platform = p.DefaultPlatform()
	}
	return p, l, platform, nil
}

// ── 路径与平台 ──────────────────────────────────────────────────────────────

// resolveRepo 解析仓库根：--repo > CNB_REPO_ROOT > 可执行文件位置推导。
func resolveRepo() (string, error) {
	if opts.repo != "" {
		return filepath.Abs(opts.repo)
	}
	if env := os.Getenv("CNB_REPO_ROOT"); env != "" {
		return filepath.Abs(env)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("无法定位可执行文件: %w", err)
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(exe))), nil
}

func resolveLockPath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if env := os.Getenv("LOCK_FILE"); env != "" {
		return env, nil
	}
	root, err := resolveRepo()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "assets.lock.toml"), nil
}

func filepathDir(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i]
	}
	return "."
}

func hostPlatform() string {
	if runtime.GOARCH == "arm64" {
		return "linux-arm64"
	}
	return "linux-amd64"
}

// mustProfilePath 返回 config/projects/<name>.toml（相对仓库根）。
func mustProfilePath(name string) string {
	root, _ := resolveRepo()
	return filepath.Join(root, opts.configDir, "projects", name+".toml")
}

// repoRootOf 由 config/projects/<name>.toml 反推仓库根。
