package cmd

import (
	"github.com/pkg/errors"

	"bksmartpacker/pkg/builder"
	"bksmartpacker/pkg/builder/executors"
	"bksmartpacker/pkg/types"
)

type Executor interface {
	Execute() error
}

// BuildSMartPackage 业务逻辑
func BuildSMartPackage(params *types.Params) error {
	plan, err := builder.PrepareBuildPlan(params)
	if err != nil {
		return errors.Wrap(err, "failed to parse build plan, please check your app_desc.yaml")
	}

	var executor Executor

	executor, err = executors.NewDockerExecutor(*plan)

	if err != nil {
		return errors.Wrap(err, "failed to new executors")
	}
	return executor.Execute()
}
