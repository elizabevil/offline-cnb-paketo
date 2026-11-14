package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/buildmeta"
)

// newDoctorCmd 体检：容器运行时、API socket、builder 契约。
func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor <profile>",
		Short: "环境与镜像契约体检（pack 前置条件）",
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
			problems := 0
			check := func(ok bool, msg, hint string) {
				if ok {
					fmt.Printf("  ✓ %s\n", msg)
					return
				}
				problems++
				fmt.Printf("  ✗ %s\n", msg)
				if hint != "" {
					fmt.Printf("      → %s\n", hint)
				}
			}

			// 1. 运行时
			tool := plan.Runtime
			check(tool != "none", "容器运行时: "+tool,
				"安装 podman 或 docker")

			// 2. Docker API（pack 需要）
			if tool == "podman" {
				sock := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "podman/podman.sock")
				if os.Getenv("XDG_RUNTIME_DIR") == "" {
					sock = fmt.Sprintf("/run/user/%d/podman/podman.sock", os.Getuid())
				}
				_, statErr := os.Stat(sock)
				check(statErr == nil, "podman API socket: "+sock,
					"systemctl --user enable --now podman.socket")
			}

			// 3. builder 指标（若已构建）
			builderTag := ""
			for _, st := range plan.Stages {
				if st.Kind == "image" && strings.Contains(st.Name, "builder") {
					builderTag = st.Tag
				}
			}
			if builderTag != "" && tool != "none" {
				env := inspectField(tool, builderTag, "{{range .Config.Env}}{{println .}}{{end}}")
				check(strings.Contains(env, "CNB_USER_ID="), "builder env CNB_USER_ID",
					"在 builder Dockerfile 中加 ENV CNB_USER_ID=1000")
				check(strings.Contains(env, "CNB_GROUP_ID="), "builder env CNB_GROUP_ID",
					"在 builder Dockerfile 中加 ENV CNB_GROUP_ID=1000")

				labels := inspectField(tool, builderTag, "{{json .Config.Labels}}")
				check(strings.Contains(labels, "io.buildpacks.stack.id"), "builder label stack.id",
					"cnb run 会自动注入；若手写请补齐")
				check(strings.Contains(labels, "runImage"), "builder metadata 含 runImage",
					"确认 metadata 由 cnb 生成（含 stack.runImage）")
			}

			// 4. metadata 可生成（不需要镜像）
			if _, err := buildmeta.Generate(plan); err != nil {
				check(false, "可生成 builder metadata", err.Error())
			} else {
				check(true, "可生成 builder metadata", "")
			}

			if problems > 0 {
				fmt.Printf("\n  共 %d 项待处理\n", problems)
				return &exitError{code: 1}
			}
			fmt.Println("\n  ✓ 体检通过")
			return nil
		},
	}
}

func inspectField(tool, image, format string) string {
	cmd := exec.Command(tool, "inspect", "--format", format, image)
	done := make(chan struct{})
	var out []byte
	var err error
	go func() {
		out, err = cmd.Output()
		close(done)
	}()
	select {
	case <-done:
		if err != nil {
			return ""
		}
		return string(out)
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		return ""
	}
}
