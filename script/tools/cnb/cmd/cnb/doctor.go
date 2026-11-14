package main

import (
	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/checks"
)

// newDoctorCmd 体检：容器运行时、API socket、builder 契约。
func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor <profile>",
		Short: "check build prerequisites (container runtime, builder contract)",
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
			rep := checks.RunDoctor(plan)
			for _, it := range rep.Items {
				switch it.Level {
				case checks.LevelOK:
					logr.Info("doctor check", "title", it.Title, "result", "ok")
				case checks.LevelSkip:
					logr.Info("doctor check", "title", it.Title, "result", "skipped")
				default:
					logr.Error("doctor check", "title", it.Title, "hint", it.Hint)
				}
			}
			if !rep.OK() {
				logr.Error("doctor failed", "problems", rep.Problems())
				return &exitError{code: ExitFailure}
			}
			logr.Info("doctor passed")
			return nil
		},
	}
}
