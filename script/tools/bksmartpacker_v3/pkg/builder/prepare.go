package builder

import (
	"bksmartpacker/pkg/appdesc"
	"bksmartpacker/pkg/log"
	"bksmartpacker/pkg/plan"
	"bksmartpacker/pkg/types"
	"bksmartpacker/pkg/utils"
)

// PrepareBuildPlan 解析参数，输出构建计划。
func PrepareBuildPlan(params *types.Params) (*plan.BuildPlan, error) {
	appDesc, rawAppDesc, err := appdesc.Load(params.Src)
	if err != nil {
		return nil, err
	}
	if err := appDesc.Validate(); err != nil {
		return nil, err
	}
	if err := appdesc.Filter(&appDesc, params.Modules, params.AllowPartial); err != nil {
		return nil, err
	}

	logoFileName := utils.DetectLogoFile(params.Src)
	if logoFileName == "" {
		log.Warn("缺少 logo 文件", "expect", "app_desc.yaml 同级 logo.png")
	} else {
		log.Info("检测到 logo 文件", "file", logoFileName)
	}
	return plan.New(appDesc, rawAppDesc, logoFileName, params)
}
