package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/materials"
	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// newUpdateCmd updates a material version (optionally rewriting platform URLs).
func newUpdateCmd() *cobra.Command {
	var urls []string
	cmd := &cobra.Command{
		Use:   "update <id[@version]> [new-version]",
		Short: "update a material version and re-render URLs from {version}",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx, err := workspace.Open(workspace.Options{
				Repo: opts.repo, LockPath: opts.lockPath, Bundle: opts.bundle, Platform: lock.Platform(opts.platform),
			})
			if err != nil {
				return failExecution(err)
			}
			req := struct {
				ID, Version, New string
				URLs             map[string]string
			}{URLs: map[string]string{}}
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
					return &exitError{code: ExitUsage, msg: "URL override must be <platform>=<url>"}
				}
				req.URLs[platform] = url
			}
			newVersion := req.New
			if newVersion == "" {
				newVersion = req.Version
			}
			if newVersion == "" {
				return &exitError{code: ExitUsage, msg: "missing new version"}
			}
			if len(req.URLs) > 0 {
				return &exitError{code: ExitUsage, msg: "update only changes the version; edit assets.toml by hand and run materials lock for URL changes"}
			}
			changed, err := manifest.EditVersion(ctx.LockPath, req.ID, req.Version, newVersion)
			if err != nil {
				return failExecution(err)
			}
			if !changed {
				logr.Info("no change", "ref", req.ID)
				return nil
			}
			m, err := lock.Load(ctx.LockPath)
			if err != nil {
				return failExecution(err)
			}
			hash, err := manifest.Hash(m)
			if err != nil {
				return failExecution(err)
			}
			if err := materials.Generate(m, ctx.LockFile, []string{req.ID}, materials.Options{
				BundleDir: ctx.Bundle, Platform: ctx.Platform, LockFile: ctx.LockFile,
				LockFilePath: ctx.LockFilePath, Manifest: m, ManifestHash: hash, Logger: logr,
			}); err != nil {
				return failExecution(err)
			}
			logr.Info("updated", "ref", req.ID, "version", newVersion)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&urls, "url", nil, "override a platform URL as <platform>=<url> (repeatable)")
	return cmd
}
