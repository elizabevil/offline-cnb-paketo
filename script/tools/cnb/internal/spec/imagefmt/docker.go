package imagefmt

import (
	"io"
	"os"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// writeTarball 写出 docker-archive tar。
func (img *Image) writeTarball(dest string) error {
	ref, err := name.ParseReference(img.repoTags()[0])
	if err != nil {
		return &errs.Invalid{Field: "repo tag", Value: img.repoTags()[0], Reason: err.Error()}
	}
	if err := tarball.WriteToFile(dest, ref, img.V1); err != nil {
		return errs.Wrap("write docker-archive", err)
	}
	return nil
}

// tarballTags 读取 docker-archive 中的 RepoTags。
func tarballTags(path string) ([]string, error) {
	entries, err := tarball.LoadManifest(func() (io.ReadCloser, error) { return os.Open(path) })
	if err != nil {
		return nil, errs.Wrap("read docker-archive manifest", err)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	return entries[0].RepoTags, nil
}
