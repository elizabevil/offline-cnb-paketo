package main

import (
	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/materials"
)

// newSyncCmd 下载 / 解包 / 打标记。
func newSyncCmd() *cobra.Command {
	var (
		offline bool
		check   bool
		jobs    int
	)
	cmd := &cobra.Command{
		Use:   "sync [ids...]",
		Short: "download, verify and extract materials",
		RunE: func(_ *cobra.Command, ids []string) error {
			l, platform, bundleDir, err := open()
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			results, err := materials.Sync(l, ids, materials.Options{
				BundleDir: bundleDir,
				Platform:  platform,
				Jobs:      jobs,
				Offline:   offline,
				Check:     check,
				Logger:    logr,
			})
			for _, r := range results {
				logr.Info("material", "ref", r.Ref, "action", r.Action)
			}
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&offline, "offline", false, "verify only, never download")
	cmd.Flags().BoolVar(&check, "check", false, "verify only")
	cmd.Flags().IntVar(&jobs, "jobs", 4, "parallel downloads")
	return cmd
}
