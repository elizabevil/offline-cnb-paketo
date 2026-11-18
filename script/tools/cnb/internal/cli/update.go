package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// newUpdateCmd 更新物料版本（可选改写各平台 URL）。
func newUpdateCmd() *cobra.Command {
	var urls []string
	cmd := &cobra.Command{
		Use:   "update <id[@version]> [new-version]",
		Short: "更新物料版本，并按 {version} 重新渲染各平台 URL",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			path, err := workspaceLockPath(opts.lockPath)
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			req := lock.UpdateRequest{URLs: map[string]string{}}
			id := args[0]
			if at := strings.LastIndex(id, "@"); at > 0 {
				req.ID, req.Version = id[:at], id[at+1:]
			} else {
				req.ID = id
			}
			if len(args) == 2 {
				req.New = args[1]
			}
			for _, item := range urls {
				platform, url, ok := strings.Cut(item, "=")
				if !ok || platform == "" || url == "" {
					return &exitError{code: ExitUsage, msg: "URL 覆盖格式应为 <platform>=<url>"}
				}
				req.URLs[platform] = url
			}
			changed, err := lock.UpdateVersion(path, req)
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			if !changed {
				logr.Info("no change", "ref", req.ID)
				return nil
			}
			logr.Info("updated", "ref", req.ID, "version", req.New)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&urls, "url", nil, "覆盖某平台 URL，格式 <platform>=<url>（可重复）")
	return cmd
}
