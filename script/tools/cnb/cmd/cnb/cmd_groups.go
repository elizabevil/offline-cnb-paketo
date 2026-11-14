package main

import "github.com/spf13/cobra"

// newMaterialsCmd 是物料域：清单、落盘、哈希与本地发布。
func newMaterialsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "materials",
		Short: "asset management (lock, bundle, hashes)",
	}
	cmd.AddCommand(
		newListCmd(), newGetCmd(), newValidateCmd(), newPlatformsCmd(),
		newVerifyCmd(), newPinCmd(), newPublishCmd(), newLockCmd(), newSyncCmd(),
	)
	return cmd
}

// newPipelineCmd 是流水线域：计划、执行、体检与静态检查。
func newPipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "build pipeline (plan, run, checks)",
	}
	cmd.AddCommand(
		newPlanCmd(), newRunCmd(), newDoctorCmd(), newDockerfileCmd(),
		newProfileCmd(), newRepackCmd(),
	)
	return cmd
}

// newBuildpackCmd 是 buildpack 域：结构检查与 builder metadata。
func newBuildpackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buildpack",
		Short: "buildpack tooling (lint, metadata)",
	}
	cmd.AddCommand(newLintCmd(), newBuilderCmd())
	return cmd
}
