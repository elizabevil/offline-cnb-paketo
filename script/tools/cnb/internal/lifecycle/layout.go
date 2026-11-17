package lifecycle

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/archive"
	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/spec/imagefmt"
)

// PrepareRunImage 把 run 镜像预置到 layout 目录，返回 layout 内的引用
// （形如 <repo>@sha256:<config digest>）。
//
// layout 目录按「一镜像一目录」的标准 OCI layout 组织：
//
//	<layout>/<registry>/<repo>/<tag|sha256/<digest>>/index.json
func PrepareRunImage(layoutDir, runRef, runArchive string, log *slog.Logger) (string, error) {
	if runArchive == "" {
		return "", &errs.Missing{What: "run image archive for layout export"}
	}
	repo, _, _, err := NormalizeRef(runRef)
	if err != nil {
		return "", err
	}

	tempDir, err := os.MkdirTemp("", "cnb-run-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	extracted := filepath.Join(tempDir, "run")
	if err := os.MkdirAll(extracted, fsutil.DirMode); err != nil {
		return "", err
	}
	if err := archive.ExtractTar(runArchive, extracted); err != nil {
		return "", errs.Wrap("extract run image", err)
	}
	img, _, err := imagefmt.Read(extracted)
	if err != nil {
		return "", errs.Wrap("read run image", err)
	}
	if img.ConfigDigest == "" {
		return "", &errs.Missing{What: "run image config digest"}
	}
	hex := strings.TrimPrefix(img.ConfigDigest, imagefmt.DigestPrefix)

	target := filepath.Join(layoutDir, repo, "sha256", hex)
	if err := os.RemoveAll(target); err != nil {
		return "", err
	}
	if err := imagefmt.Write(img, imagefmt.FormatOCILayout, target); err != nil {
		return "", errs.Wrap("write run image to layout", err)
	}
	ref := repo + "@" + imagefmt.DigestPrefix + hex
	log.Info("run image prepared", "ref", ref, "path", target)
	return ref, nil
}

// NormalizeRef 把镜像引用归一化为 layout 内的目录路径。
//
//	run:latest        → index.docker.io/library/run
//	run@sha256:<d>    → index.docker.io/library/run（digest 单独返回）
//	localhost:5000/x  → localhost:5000/x
func NormalizeRef(ref string) (repo, tag, digest string, err error) {
	if strings.TrimSpace(ref) == "" {
		return "", "", "", &errs.Invalid{Field: "run image", Value: ref, Reason: "empty reference"}
	}
	name := ref
	if at := strings.LastIndex(name, "@"); at >= 0 {
		digest = name[at+1:]
		name = name[:at]
	} else if colon := strings.LastIndex(name, ":"); colon > strings.LastIndex(name, "/") {
		tag = name[colon+1:]
		name = name[:colon]
	}
	if name == "" {
		return "", "", "", &errs.Invalid{Field: "run image", Value: ref, Reason: "missing repository"}
	}
	if !strings.Contains(name, "/") {
		name = "index.docker.io/library/" + name
	} else if !strings.Contains(strings.SplitN(name, "/", 2)[0], ".") && !strings.Contains(strings.SplitN(name, "/", 2)[0], ":") {
		name = "index.docker.io/" + name
	}
	return name, tag, digest, nil
}
