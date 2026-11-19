package archive

import (
	"archive/tar"
	"os"
	"time"
)

// normalizeHeader 固定属主与时间，保证同内容产出相同字节。
func normalizeHeader(hdr *tar.Header, info os.FileInfo) {
	hdr.Uid, hdr.Gid = 0, 0
	hdr.Uname, hdr.Gname = "", ""
	hdr.ModTime = time.Unix(0, 0)
	hdr.AccessTime, hdr.ChangeTime = time.Time{}, time.Time{}
	switch {
	case info.IsDir(), info.Mode()&0o111 != 0:
		hdr.Mode = 0o755
	default:
		hdr.Mode = 0o644
	}
}

// PackTar 把目录打包为未压缩 tar（docker save 的 web.tgz 就是这种形态）。
// 属主固定为 0:0、时间固定，产出可复现。
func PackTar(srcDir, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	tw := tar.NewWriter(out)
	defer func() { _ = tw.Close() }()

	return writeTree(tw, srcDir)
}
