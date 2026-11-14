package lock

import (
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

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
		return "", &errs.Invalid{Field: "platform", Value: s, Reason: "expect <os>-<arch>"}
	}
	if !knownOS[osName] {
		return "", &errs.Invalid{Field: "platform", Value: s, Reason: "unsupported os " + osName}
	}
	if !knownArch[arch] {
		return "", &errs.Invalid{Field: "platform", Value: s, Reason: "unsupported arch " + arch}
	}
	return Platform(s), nil
}

// PlatformOS 返回平台的操作系统部分。
func PlatformOS(p Platform) string {
	osName, _, _ := strings.Cut(p, "-")
	return osName
}

// IsLinux 判断平台是否为 Linux。
func IsLinux(p Platform) bool { return PlatformOS(p) == OSLinux }

// IsDarwin 判断平台是否为 macOS。
func IsDarwin(p Platform) bool { return PlatformOS(p) == OSDarwin }

// IsWindows 判断平台是否为 Windows。
func IsWindows(p Platform) bool { return PlatformOS(p) == OSWindows }

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
