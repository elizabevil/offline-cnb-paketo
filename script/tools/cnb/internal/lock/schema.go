package lock

// TOML 段落名与键名是清单协议的一部分，集中在此维护：
// 约定变化（如改段落命名）只需改这里的常量与构造函数。
const (
	// SectionAsset 是每个物料的顶层段落。
	SectionAsset = "[[asset]]"
	// SectionPlatformPrefix 是平台段前缀，后缀为平台名。
	SectionPlatformPrefix = "[asset.platform."
	// SectionPlatformSuffix 是平台段后缀。
	SectionPlatformSuffix = "]"
	// SectionLocal 是本地构建物料的段落。
	SectionLocal = "[asset.local]"

	// KeySHA256 / KeySize / KeySource 是写回时用到的键名。
	KeySHA256 = "sha256"
	KeySize   = "size"
	KeySource = "source"
)

// PlatformSection 返回平台段名，例如 [asset.platform.linux-amd64]。
func PlatformSection(platform Platform) string {
	return SectionPlatformPrefix + platform + SectionPlatformSuffix
}

// LocalSection 返回本地物料段名。
func LocalSection() string { return SectionLocal }
