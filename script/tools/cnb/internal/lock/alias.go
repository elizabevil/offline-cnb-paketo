package lock

import "learn-cnb-paketo/tools/cnb/internal/spec/manifest"

// 类型别名：调用方继续使用 lock.X，实现位于 spec/manifest。
type (
	// File 是物料清单。
	File = manifest.File
	// Meta 是清单元信息。
	Meta = manifest.Meta
	// Bundle declares where unpacked materials are stored.
	Bundle = manifest.Bundle
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

	// ID 是物料标识。
	ID = manifest.ID
	// Version 是物料版本号。
	Version = manifest.Version
	// Ref 是物料引用（id@version）。
	Ref = manifest.Ref
)

// 常量再导出。
const (
	KindTGZ              = manifest.KindTGZ
	KindTarGz            = manifest.KindTarGz
	KindZip              = manifest.KindZip
	KindBin              = manifest.KindBin
	KindHTTP             = manifest.KindHTTP
	PlatformAMD64        = manifest.PlatformAMD64
	PlatformARM64        = manifest.PlatformARM64
	ArchPrefix           = manifest.ArchPrefix
	KindImage            = manifest.KindImage
	KindCommand          = manifest.KindCommand
	KindFetch            = manifest.KindFetch
	ContextBundle        = manifest.ContextBundle
	ContextBuildpacks    = manifest.ContextBuildpacks
	DefaultManifestName  = manifest.DefaultManifestName
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
