package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/doctor"
	"learn-cnb-paketo/tools/cnb/internal/render"
)

// newDoctorCmd 体检：容器运行时、API socket、builder 契约。
func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor <profile>",
		Short: "check build prerequisites (container runtime, builder contract)",
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
			rep := doctor.Run(plan)
			out := cmd.OutOrStdout()
			for _, it := range rep.Items {
				switch it.Level {
				case doctor.LevelOK:
					fmt.Fprintf(out, "  %s %s\n", render.Success(""), it.Title)
				case doctor.LevelSkip:
					fmt.Fprintf(out, "  ·  %s\n", it.Title)
				default:
					fmt.Fprintf(out, "  %s %s\n", render.Failure(""), it.Title)
					if it.Hint != "" {
						fmt.Fprintf(out, "      → %s\n", it.Hint)
					}
				}
			}
			if !rep.OK() {
				fmt.Fprintf(out, "\n  %d problem(s)\n", rep.Problems())
				return &exitError{code: ExitFailure}
			}
			fmt.Fprintln(out, "\n  doctor passed")
			return nil
		},
	}
}
