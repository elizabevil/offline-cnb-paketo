package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/dockerfile"
	"learn-cnb-paketo/tools/cnb/internal/render"
)

// newDockerfileCmd 静态校验镜像构建所需的源文件是否存在。
func newDockerfileCmd() *cobra.Command {
	parent := &cobra.Command{Use: "dockerfile", Short: "Dockerfile static checks"}
	parent.AddCommand(&cobra.Command{
		Use:   "check <profile>",
		Short: "check COPY sources exist (supports --from=bundle/buildpacks)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, l, platform, err := openProfile(args[0])
			if err != nil {
				return err
			}
			plan, err := p.BuildPlan(platform, l)
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			rep := dockerfile.Check(plan, platform)
			out := cmd.OutOrStdout()
			for _, m := range rep.Missing {
				fmt.Fprintf(out, "  %s %s: %s (resolved: %s)\n", render.Failure(""), m.Stage, m.Source, m.Target)
			}
			if !rep.OK() {
				return &exitError{code: ExitFailure}
			}
			fmt.Fprintf(out, "  %s dockerfile sources ok (%s)\n", render.Success(""), plan.Project)
			return nil
		},
	})
	return parent
}
