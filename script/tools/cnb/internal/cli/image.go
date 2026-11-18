package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/spec/imagefmt"
)

// newImageCmd 是镜像格式转换的通用入口（不涉及任何平台包结构）。
func newImageCmd() *cobra.Command {
	parent := &cobra.Command{Use: "image", Short: "镜像归档格式转换（docker-archive / oci-layout）"}

	var to string
	convert := &cobra.Command{
		Use:   "convert <src> <dst>",
		Short: "镜像格式互转（OCI layout 目录 ↔ docker-archive tar）",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			img, from, err := imagefmt.Open(src)
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			if to == "" {
				to = imagefmt.FormatOCILayout
			}
			if to == from {
				return &exitError{code: ExitUsage, msg: fmt.Sprintf("源与目标格式相同（%s）", to)}
			}
			if err := img.Write(to, dst); err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			layers, _ := img.V1.Layers()
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s → %s（%d 层，标签 %v）\n", from, to, len(layers), img.RepoTags)
			return nil
		},
	}
	convert.Flags().StringVar(&to, "to", imagefmt.FormatOCILayout, "目标格式：oci-layout | docker-archive")
	parent.AddCommand(convert)
	return parent
}
