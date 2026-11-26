package cmd

import (
	"github.com/pkg/errors"

	"bksmartpacker/pkg/builder"
	"bksmartpacker/pkg/builder/executors"
	"bksmartpacker/pkg/container"
	"bksmartpacker/pkg/types"
)

// Executor 是构建计划执行器。
type Executor interface {
	Execute() error
}

// BuildSMartPackage 业务逻辑
func BuildSMartPackage(params *types.Params) error {
	plan, err := builder.PrepareBuildPlan(params)
	if err != nil {
		return errors.Wrap(err, "failed to parse build plan, please check your app_desc.yaml")
	}

	runtimeCfg, err := container.ResolveFromEnv()
	if err != nil {
		return errors.Wrap(err, "failed to resolve container runtime")
	}

	var executor Executor
	executor, err = executors.NewContainerExecutor(*plan, runtimeCfg)
	if err != nil {
		return errors.Wrap(err, "failed to new executors")
	}
	return executor.Execute()
}
