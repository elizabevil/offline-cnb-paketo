package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/checks"
)

func newBuilderCmd() *cobra.Command {
	var format string
	meta := &cobra.Command{
		Use:   "metadata <profile>",
		Short: "生成 io.buildpacks.builder.metadata（json 或 --label 形式）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, l, platform, err := openProfile(args[0])
			if err != nil {
				return err
			}
			plan, err := p.BuildPlan(platform, l)
			if err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			m, err := checks.Generate(plan)
			if err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			js, err := m.JSON()
			if err != nil {
				return err
			}
			switch format {
			case "json", "":
				fmt.Fprintln(cmd.OutOrStdout(), js)
			case "label":
				fmt.Fprintf(cmd.OutOrStdout(), "io.buildpacks.builder.metadata=%s\n", js)
			default:
				return &exitError{code: 2, msg: "不支持的格式: " + format}
			}
			return nil
		},
	}
	meta.Flags().StringVar(&format, "format", "json", "输出格式：json | label")
	return meta
}
