package manifest

import (
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// Platform 是目标平台名，形如 <os>-<arch>。
type Platform string

// 支持的 OS 与架构（清单协议的一部分）。
const (
	OSLinux   = "linux"
	OSDarwin  = "darwin"
	OSWindows = "windows"

	ArchAMD64 = "amd64"
	ArchARM64 = "arm64"
)

// knownOS / knownArch 是平台解析的白名单。
var (
	knownOS   = map[string]bool{OSLinux: true, OSDarwin: true, OSWindows: true}
	knownArch = map[string]bool{ArchAMD64: true, ArchARM64: true}
)

// ParsePlatform 解析并校验 <os>-<arch>（如 linux-amd64、darwin-arm64）。
func ParsePlatform(s string) (Platform, error) {
	osName, arch, ok := strings.Cut(s, "-")
	if !ok {
		return "", &errs.Invalid{Field: fieldPlatform, Value: s, Reason: "expect <os>-<arch>"}
	}
	if !knownOS[osName] {
		return "", &errs.Invalid{Field: fieldPlatform, Value: s, Reason: "unsupported os " + osName}
	}
	if !knownArch[arch] {
		return "", &errs.Invalid{Field: fieldPlatform, Value: s, Reason: "unsupported arch " + arch}
	}
	return Platform(s), nil
}

// OS 返回平台的操作系统部分。
func (p Platform) OS() string {
	osName, _, _ := strings.Cut(string(p), "-")
	return osName
}

// IsLinux 判断平台是否为 Linux。
func (p Platform) IsLinux() bool { return p.OS() == OSLinux }

// IsDarwin 判断平台是否为 macOS。
func (p Platform) IsDarwin() bool { return p.OS() == OSDarwin }

// IsWindows 判断平台是否为 Windows。
func (p Platform) IsWindows() bool { return p.OS() == OSWindows }

// SupportedPlatforms 返回受支持的平台列表（用于错误提示与文档）。
func SupportedPlatforms() []string {
	out := make([]string, 0, len(knownOS)*len(knownArch))
	for _, osName := range []string{OSLinux, OSDarwin, OSWindows} {
		for _, arch := range []string{ArchAMD64, ArchARM64} {
			out = append(out, osName+"-"+arch)
		}
	}
	return out
}
