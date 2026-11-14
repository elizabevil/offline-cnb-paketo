package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var copyRe = regexp.MustCompile(`^(COPY|ADD)\s+(.*)$`)
var varRe = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)

// newDockerfileCmd 静态校验镜像构建所需的源文件是否存在。
func newDockerfileCmd() *cobra.Command {
	parent := &cobra.Command{Use: "dockerfile", Short: "Dockerfile 静态检查"}
	parent.AddCommand(&cobra.Command{
		Use:   "check <profile>",
		Short: "检查 COPY 源路径是否存在（含 --from=bundle）",
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
			missing := 0
			for _, st := range plan.Stages {
				if st.Kind != "image" {
					continue
				}
				env := map[string]string{"BUNDLE_ARCH": platform, "ARCH": strings.TrimPrefix(platform, "linux-")}
				for k, v := range st.Args {
					env[k] = v
				}
				for _, src := range copySources(st.Dockerfile, env) {
					base, path := splitFrom(src)
					var target string
					switch base {
					case "":
						target = filepath.Join(st.Context, path)
					case "bundle":
						target = filepath.Join(plan.BundleDir, path)
					default:
						continue // 其他镜像阶段，跳过
					}
					if _, err := os.Stat(target); err != nil {
						fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s: %s (resolved: %s)\n", st.Name, src, target)
						missing++
					}
				}
			}
			if missing > 0 {
				return &exitError{code: 1}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ Dockerfile 源路径检查通过（%s）\n", p.Name())
			return nil
		},
	})
	return parent
}

// copySources 提取 COPY/ADD 的源路径（已做变量替换），跳过 --from=<其他镜像>。
func copySources(dockerfile string, env map[string]string) []string {
	data, err := os.ReadFile(dockerfile)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		m := copyRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		fields := strings.Fields(m[2])
		var from string
		var srcs []string
		for _, f := range fields {
			if strings.HasPrefix(f, "--from=") {
				from = strings.TrimPrefix(f, "--from=")
				continue
			}
			if strings.HasPrefix(f, "--") {
				continue
			}
			srcs = append(srcs, f)
		}
		if len(srcs) < 2 {
			continue
		}
		if from != "" && from != "bundle" {
			continue
		}
		for _, s := range srcs[:len(srcs)-1] {
			s = varRe.ReplaceAllStringFunc(s, func(m string) string {
				key := varRe.FindStringSubmatch(m)[1]
				if v, ok := env[key]; ok {
					return v
				}
				return m
			})
			if strings.Contains(s, "${") {
				continue // 无法解析的变量，跳过
			}
			out = append(out, joinFrom(from, s))
		}
	}
	return out
}

func joinFrom(from, src string) string { return from + "|" + src }

func splitFrom(s string) (string, string) {
	base, path, _ := strings.Cut(s, "|")
	return base, path
}
