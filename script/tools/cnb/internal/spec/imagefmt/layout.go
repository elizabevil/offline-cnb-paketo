package imagefmt

import (
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

const (
	fileIndexJSON = "index.json"
	fileOCILayout = "oci-layout"
)

// openLayout 读取 OCI layout 目录里的第一个镜像。
func openLayout(dir string) (*Image, error) {
	lp, err := layout.FromPath(dir)
	if err != nil {
		return nil, errs.Wrap("read oci layout", err)
	}
	idx, err := lp.ImageIndex()
	if err != nil {
		return nil, errs.Wrap("read index", err)
	}
	man, err := idx.IndexManifest()
	if err != nil {
		return nil, errs.Wrap("parse index", err)
	}
	if len(man.Manifests) == 0 {
		return nil, &errs.Invalid{Field: fileIndexJSON, Value: dir, Reason: "empty index"}
	}
	desc := man.Manifests[0]
	img, err := idx.Image(desc.Digest)
	if err != nil {
		return nil, errs.Wrap("read image", err)
	}
	imgRepoTags := []string{}
	if name := desc.Annotations["io.containerd.image.name"]; name != "" {
		imgRepoTags = append(imgRepoTags, strings.TrimPrefix(name, "docker.io/"))
	}
	return &Image{V1: img, RepoTags: imgRepoTags}, nil
}

// writeLayout 写出 OCI layout 目录，并补 docker 兼容文件。
func (img *Image) writeLayout(dest string) error {
	lp, err := layout.Write(dest, empty.Index)
	if err != nil {
		return errs.Wrap("write oci layout", err)
	}
	if err := lp.AppendImage(img.V1, layout.WithAnnotations(manifestAnnotations(img.repoTags()[0]))); err != nil {
		return errs.Wrap("append image", err)
	}
	return writeCompatFiles(dest, img)
}

// isLayoutDir 判断目录是否包含 OCI layout 标记。
func isLayoutDir(dir string) bool {
	if _, err := layout.FromPath(dir); err != nil {
		return false
	}
	if _, err := statFile(filepath.Join(dir, fileIndexJSON)); err != nil {
		return false
	}
	return true
}
