// Package version 汇总构建信息：手动版本 + Go buildinfo（git/架构/时间）。
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Version 由 ldflags 注入：-X learn-cnb-paketo/tools/cnb/internal/version.Version=3.0.0
// 未注入时退化为 "dev"。
var Version = "dev"

// Info 是 `cnb version` 的结构化输出。
type Info struct {
	Version    string `json:"version"` // 手动版本
	Commit     string `json:"commit"`  // git 修订
	CommitTime string `json:"commit_time"`
	Modified   bool   `json:"modified"` // 工作区是否有未提交改动
	GoVersion  string `json:"go_version"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
}

// Get 读取 buildinfo 并合并手动版本。
func Get() Info {
	info := Info{
		Version:   Version,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			info.Commit = s.Value
		case "vcs.time":
			info.CommitTime = s.Value
		case "vcs.modified":
			info.Modified = s.Value == "true"
		}
	}
	// 若未用 ldflags 指定版本，尝试取模块版本（go install 场景）
	if info.Version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		info.Version = bi.Main.Version
	}
	return info
}

// String 输出单行摘要，如：
// cnb 3.0.0 (linux/amd64, go1.24.0, a1b2c3d4, dirty)
func (i Info) String() string {
	parts := []string{fmt.Sprintf("%s/%s", i.OS, i.Arch), i.GoVersion}
	if i.Commit != "" {
		c := i.Commit
		if len(c) > 8 {
			c = c[:8]
		}
		parts = append(parts, c)
	}
	if i.Modified {
		parts = append(parts, "dirty")
	}
	return fmt.Sprintf("cnb %s (%s)", i.Version, strings.Join(parts, ", "))
}

// Short 返回用于物料目录的版本号（手动版本，缺省 dev）。
func (i Info) Short() string { return i.Version }
