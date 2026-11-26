package executors

import (
	"strings"

	"bksmartpacker/pkg/container"
)

// argBuilder 是外部 CLI 的通用参数构造器。
// 只负责参数形状，不绑定 run/import/save 等具体命令。
type argBuilder struct {
	values []string
}

func newArgBuilder(command ...string) *argBuilder {
	return &argBuilder{values: append([]string{}, command...)}
}

func (b *argBuilder) add(values ...string) {
	b.values = append(b.values, values...)
}

func (b *argBuilder) flag(name string) {
	b.add(name)
}

func (b *argBuilder) option(name, value string) {
	b.add(name, value)
}

func (b *argBuilder) env(key, value string) {
	b.add("-e", key+"="+value)
}

func (b *argBuilder) bindMount(source, target string, readOnly bool) {
	b.add("--mount", bindMountSpec(source, target, readOnly))
}

func bindMountSpec(source, target string, readOnly bool) string {
	parts := []string{"type=bind", "source=" + source, "target=" + target}
	if readOnly {
		parts = append(parts, "readonly")
	}
	return strings.Join(parts, ",")
}

func volumeMountSpec(source, target string) string {
	return strings.Join([]string{"type=volume", "source=" + source, "target=" + target}, ",")
}

func bindMountArgs(source, target string, readOnly bool) []string {
	return []string{"--mount", bindMountSpec(source, target, readOnly)}
}

func (b *argBuilder) args() []string {
	return b.values
}

// imageSaveArgs 生成镜像导出参数，供 save 和 fallback 复用。
func imageSaveArgs(cfg container.Config, dest, image string) []string {
	b := newArgBuilder("image", "save")
	if cfg.IsPodman() {
		b.option("--format", "oci-archive")
	}
	b.option("-o", dest)
	b.add(image)
	return b.args()
}

// dockerArchiveSaveArgs 生成 docker-archive；run image archive 需要 manifest.json。
func dockerArchiveSaveArgs(dest, image string) []string {
	b := newArgBuilder("image", "save")
	b.option("-o", dest)
	b.add(image)
	return b.args()
}

// imageImportArgs 生成 fake runner 导入参数。
func imageImportArgs(platform, image string) []string {
	b := newArgBuilder("import")
	b.option("-c", "ENV HOME=/app")
	b.option("-c", "WORKDIR /app")
	if platform != "" {
		osName, arch := splitPlatform(platform)
		b.option("--os", osName)
		b.option("--arch", arch)
	}
	b.add("-", image)
	return b.args()
}
