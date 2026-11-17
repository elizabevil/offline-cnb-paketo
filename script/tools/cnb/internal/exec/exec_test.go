package exec

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"learn-cnb-paketo/tools/cnb/internal/plan"
)

func logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func mkPlan(stages ...plan.Stage) *plan.Plan {
	for i := range stages {
		if stages[i].Timeout == 0 {
			stages[i].Timeout = plan.DefaultTimeout
		}
	}
	return &plan.Plan{Stages: stages}
}

func TestRunSuccess(t *testing.T) {
	p := mkPlan(
		plan.Stage{Name: "a", Argv: []string{"true"}, Workdir: "."},
		plan.Stage{Name: "b", Argv: []string{"true"}, Workdir: "."},
	)
	res, err := Run(context.Background(), p, Options{}, logger())
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].Err != nil || res[1].Err != nil {
		t.Fatalf("结果不符: %+v", res)
	}
}

func TestRunStopsOnFailure(t *testing.T) {
	p := mkPlan(
		plan.Stage{Name: "a", Argv: []string{"false"}, Workdir: "."},
		plan.Stage{Name: "b", Argv: []string{"true"}, Workdir: "."},
	)
	_, err := Run(context.Background(), p, Options{}, logger())
	if err == nil || !strings.Contains(err.Error(), "stage a failed") {
		t.Fatalf("应报 stage a 失败: %v", err)
	}
}

func TestRunContinueOnError(t *testing.T) {
	p := mkPlan(
		plan.Stage{Name: "a", Argv: []string{"false"}, Workdir: ".", ContinueOnError: true},
		plan.Stage{Name: "b", Argv: []string{"true"}, Workdir: "."},
	)
	res, err := Run(context.Background(), p, Options{}, logger())
	if err == nil {
		t.Fatal("最终应返回失败计数")
	}
	if len(res) != 2 || res[1].Err != nil || res[1].Skipped {
		t.Fatalf("后续阶段应继续执行: %+v", res)
	}
}

func TestRunFrom(t *testing.T) {
	p := mkPlan(
		plan.Stage{Name: "a", Argv: []string{"false"}, Workdir: "."},
		plan.Stage{Name: "b", Argv: []string{"true"}, Workdir: "."},
	)
	res, err := Run(context.Background(), p, Options{From: "b"}, logger())
	if err != nil {
		t.Fatal(err)
	}
	if !res[0].Skipped || res[1].Skipped {
		t.Fatalf("--from 跳过逻辑错误: %+v", res)
	}
}

func TestRunFromUnknown(t *testing.T) {
	p := mkPlan(plan.Stage{Name: "a", Argv: []string{"true"}, Workdir: "."})
	if _, err := Run(context.Background(), p, Options{From: "nope"}, logger()); err == nil {
		t.Fatal("未知 stage 应报错")
	}
}
