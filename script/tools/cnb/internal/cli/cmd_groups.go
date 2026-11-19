package cli

import "github.com/spf13/cobra"

// newMaterialsCmd is the materials group: listing, sync, hashes, local publish.
func newMaterialsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "materials",
		Short: "asset management (lock, bundle, hashes)",
	}
	cmd.AddCommand(
		newListCmd(), newValidateCmd(), newPlatformsCmd(),
		newVerifyCmd(), newPinCmd(), newUpdateCmd(), newPublishCmd(), newLockCmd(), newSyncCmd(),
	)
	return cmd
}

// newPipelineCmd is the pipeline group: plan, run, doctor, static checks.
func newPipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "build pipeline (plan, run, checks)",
	}
	cmd.AddCommand(
		newPlanCmd(), newRunCmd(), newProfileCmd(), newVariablesCmd(),
	)
	return cmd
}

// newBuildpackCmd is the buildpack group: layout checks and builder metadata.
func newBuildpackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buildpack",
		Short: "buildpack tooling (lint, metadata)",
	}
	cmd.AddCommand(newLintCmd())
	return cmd
}
