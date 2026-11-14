package main

import (
	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/checks"
)

// newDockerfileCmd 静态校验镜像构建所需的源文件是否存在。
func newDockerfileCmd() *cobra.Command {
	parent := &cobra.Command{Use: "dockerfile", Short: "Dockerfile static checks"}
	parent.AddCommand(&cobra.Command{
		Use:   "check <profile>",
		Short: "check COPY sources exist (supports --from=bundle/buildpacks)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			p, l, platform, err := openProfile(args[0])
			if err != nil {
				return err
			}
			plan, err := p.BuildPlan(platform, l)
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			rep := checks.CheckDockerfile(plan, string(platform))
			for _, m := range rep.Missing {
				logr.Error("missing COPY source", "stage", m.Stage, "source", m.Source, "resolved", m.Target)
			}
			if !rep.OK() {
				return &exitError{code: ExitFailure}
			}
			logr.Info("dockerfile sources ok", "project", plan.Project)
			return nil
		},
	})
	return parent
}
