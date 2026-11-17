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
		Use:   "convert <src-dir> <dst-dir>",
		Short: "把镜像目录从一种格式转换为另一种（自动探测输入）",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			img, from, err := imagefmt.Read(src)
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			if to == "" {
				to = imagefmt.FormatOCILayout
			}
			if to == from.Name() {
				return &exitError{code: ExitUsage, msg: fmt.Sprintf("源与目标格式相同（%s）", to)}
			}
			if err := imagefmt.Write(img, to, dst); err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s → %s（%d 层，标签 %v）\n", from.Name(), to, len(img.Layers), img.RepoTags)
			return nil
		},
	}
	convert.Flags().StringVar(&to, "to", imagefmt.FormatOCILayout, "目标格式：oci-layout | docker-archive")
	parent.AddCommand(convert)
	return parent
}
