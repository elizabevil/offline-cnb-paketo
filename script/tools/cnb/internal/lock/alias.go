package lock

import "learn-cnb-paketo/tools/cnb/internal/spec/manifest"

// 类型别名：调用方继续使用 lock.X，实现位于 spec/manifest。
type (
	// File 是物料清单。
	File = manifest.File
	// Meta 是清单元信息。
	Meta = manifest.Meta
	// Storage 声明物料落盘位置。
	Storage = manifest.Storage
	// Asset 是单个物料。
	Asset = manifest.Asset
	// Info 是物料说明。
	Info = manifest.Info
	// Unpack 是解包规则。
	Unpack = manifest.Unpack
	// Source 是单平台下载源。
	Source = manifest.Source
	// Kind 是解包方式。
	Kind = manifest.Kind
	// Platform 是目标平台。
	Platform = manifest.Platform

	// ID / Version / Ref 与 string 互通。
	ID      = manifest.ID
	Version = manifest.Version
	Ref     = manifest.Ref
)

// 常量再导出。
const (
	KindTGZ              = manifest.KindTGZ
	KindTarGz            = manifest.KindTarGz
	KindZip              = manifest.KindZip
	KindBin              = manifest.KindBin
	PlatformAMD64        = manifest.PlatformAMD64
	PlatformARM64        = manifest.PlatformARM64
	ArchPrefix           = manifest.ArchPrefix
	KindImage            = manifest.KindImage
	KindCommand          = manifest.KindCommand
	KindFetch            = manifest.KindFetch
	ContextBundle        = manifest.ContextBundle
	ContextBuildpacks    = manifest.ContextBuildpacks
	DefaultLockName      = manifest.DefaultLockName
	DefaultBundle        = manifest.DefaultBundle
	DefaultConfig        = manifest.DefaultConfig
	DefaultStorageLayout = manifest.DefaultStorageLayout
	LayoutArchFirst      = manifest.LayoutArchFirst
	LayoutKindFirst      = manifest.LayoutKindFirst
	DefaultBundleDir     = manifest.DefaultBundleDir
	KeySHA256            = manifest.KeySHA256
	KeySize              = manifest.KeySize
)

// 函数再导出。
var (
	ParsePlatform      = manifest.ParsePlatform
	KnownPlatforms     = manifest.KnownPlatforms
	Find               = manifest.Find
	PlatformSection    = manifest.PlatformSection
	SupportedPlatforms = manifest.SupportedPlatforms
	ArchOf             = manifest.ArchOf
)
