// Package imagefmt 在 docker-archive（tar）与 OCI layout（目录）之间读写镜像。
//
// 解析与生成全部交给 github.com/google/go-containerregistry：
//   - OCI layout：layout.Path（oci-layout + index.json + blobs/sha256）
//   - docker-archive：tarball（manifest.json + 层 tar）
//
// 本包只负责格式探测、标签/注解与 docker 兼容文件的补齐。
package imagefmt

import (
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// 格式名。
const (
	FormatDockerArchive = "docker-archive"
	FormatOCILayout     = "oci-layout"
)

// Image 是一份待写出/已读入的镜像。
type Image struct {
	// V1 是 go-containerregistry 的镜像对象；config/manifest 的原始字节
	// 由库保证不变（created / history / 未知字段都不会丢）。
	V1 v1.Image
	// RepoTags 是镜像标签，用于 docker-archive 的 tag 与 OCI index 注解。
	RepoTags []string
}

// Open 读取镜像；返回格式名（docker-archive / oci-layout）。
func Open(path string) (*Image, string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, "", &errs.NotFound{Kind: "image", Ref: path}
	}
	if fi.IsDir() {
		img, err := openLayout(path)
		return img, FormatOCILayout, err
	}
	img, err := openTarball(path)
	return img, FormatDockerArchive, err
}

// Write 按格式写出镜像：oci-layout 写目录，docker-archive 写 tar 文件。
func (img *Image) Write(format, dest string) error {
	switch format {
	case FormatOCILayout:
		return img.writeLayout(dest)
	case FormatDockerArchive:
		return img.writeTarball(dest)
	default:
		return &errs.Invalid{Field: "format", Value: format, Reason: "unknown image format"}
	}
}

// repoTags 返回有效标签（缺省为 image:latest）。
func (img *Image) repoTags() []string {
	if len(img.RepoTags) > 0 {
		return img.RepoTags
	}
	return []string{"image:latest"}
}

func openTarball(path string) (*Image, error) {
	img, err := tarball.ImageFromPath(path, nil)
	if err != nil {
		return nil, errs.Wrap("read docker-archive", err)
	}
	tags, err := tarballTags(path)
	if err != nil {
		return nil, err
	}
	return &Image{V1: img, RepoTags: tags}, nil
}
