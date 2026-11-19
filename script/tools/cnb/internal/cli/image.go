package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/spec/imagefmt"
)

// newImageCmd is the generic image format conversion entry point.
func newImageCmd() *cobra.Command {
	parent := &cobra.Command{Use: "image", Short: "image archive format conversion (docker-archive / oci-layout)"}

	var to string
	convert := &cobra.Command{
		Use:   "convert <src> <dst>",
		Short: "convert between OCI layout and docker-archive",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			img, from, err := imagefmt.Open(src)
			if err != nil {
				return failExecution(err)
			}
			if to == "" {
				to = imagefmt.FormatOCILayout
			}
			if to == from {
				return &exitError{code: ExitUsage, msg: fmt.Sprintf("source and target formats are identical (%s)", to)}
			}
			if err := img.Write(to, dst); err != nil {
				return failExecution(err)
			}
			layers, _ := img.V1.Layers()
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s → %s (%d layers, tags %v)\n", from, to, len(layers), img.RepoTags)
			return nil
		},
	}
	convert.Flags().StringVar(&to, "to", imagefmt.FormatOCILayout, "target format: oci-layout | docker-archive")
	parent.AddCommand(convert)
	return parent
}
