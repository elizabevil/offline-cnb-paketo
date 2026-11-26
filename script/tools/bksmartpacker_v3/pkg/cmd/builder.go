package cmd

import (
	"context"

	"github.com/pkg/errors"

	"bksmartpacker/pkg/builder"
	"bksmartpacker/pkg/builder/executors"
	"bksmartpacker/pkg/container"
	"bksmartpacker/pkg/plan"
	"bksmartpacker/pkg/types"
)

// Executor 是构建计划执行器。
type Executor interface {
	Execute(ctx context.Context) error
}

// BuildSMartPackage 业务逻辑
func BuildSMartPackage(params *types.Params) error {
	timeout := params.BuildTimeout
	if timeout <= 0 {
		timeout = plan.DefaultBuildTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

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
	if err := executor.Execute(ctx); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errors.Wrapf(err, "build timed out after %s", timeout)
		}
		return err
	}
	return nil
}
