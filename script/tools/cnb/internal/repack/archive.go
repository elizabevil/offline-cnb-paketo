package repack

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"os/user"
)

// extractTar 解包 tar / tar.gz 到 dest（按文件头类型还原目录、链接、权限、mtime）。
//
//nolint:cyclop,funlen // TODO(protocol-layers): 条目类型分支拆成处理函数
func extractTar(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	reader := bufio.NewReader(f)
	var src io.Reader = reader
	if magic, _ := reader.Peek(2); len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return err
		}
		defer func() { _ = gz.Close() }()
		src = gz
	}

	tr := tar.NewReader(src)
	cleanDest := filepath.Clean(dest) + string(os.PathSeparator)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, filepath.FromSlash(hdr.Name))
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) {
			return &errs.Invalid{Field: "path", Value: hdr.Name, Reason: "path traversal"}
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), fsutil.DirMode); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close() // 写入已完成，关闭错误不影响产物
				return err
			}
			_ = out.Close() // 写入已完成，关闭错误不影响产物
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), fsutil.DirMode); err != nil {
				return err
			}
			// 覆盖已存在文件时忽略删除失败（随后会重建）
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			if err := os.MkdirAll(filepath.Dir(target), fsutil.DirMode); err != nil {
				return err
			}
			// 覆盖已存在文件时忽略删除失败（随后会重建）
			_ = os.Remove(target)
			if err := os.Link(filepath.Join(dest, filepath.FromSlash(hdr.Linkname)), target); err != nil {
				return err
			}
		case tar.TypeXGlobalHeader:
			continue
		default:
			continue
		}

		if hdr.Typeflag != tar.TypeSymlink && !hdr.ModTime.IsZero() {
			// 保留时间戳失败不影响内容正确性
			_ = os.Chtimes(target, hdr.ModTime, hdr.ModTime)
		}
	}
	return nil
}

// packDirToTar 把目录打成 plain tar：顶层条目按名排序，目录递归展开（与 Python 版一致）。
func packDirToTar(srcDir, destTar string) error {
	tmp := destTar + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(f)

	entries, err := os.ReadDir(srcDir) // os.ReadDir 已按文件名排序
	if err != nil {
		_ = f.Close() // 写入已完成，关闭错误不影响产物
		return err
	}
	for _, entry := range entries {
		if err := addTree(tw, filepath.Join(srcDir, entry.Name()), entry.Name()); err != nil {
			_ = tw.Close() // 写入已完成，关闭错误不影响产物
			_ = f.Close()  // 写入已完成，关闭错误不影响产物
			return err
		}
	}
	if err := tw.Close(); err != nil {
		_ = f.Close() // 写入已完成，关闭错误不影响产物
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, destTar)
}

// packDirToTarGz 用于重建 SmartPackage（gzip 压缩级别与 Python 不同，内容等价）。
func packDirToTarGz(srcDir, destTarGz string) error {
	tmp := destTarGz + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		_ = f.Close() // 写入已完成，关闭错误不影响产物
		return err
	}
	for _, entry := range entries {
		if err := addTree(tw, filepath.Join(srcDir, entry.Name()), entry.Name()); err != nil {
			_ = tw.Close() // 写入已完成，关闭错误不影响产物
			_ = gz.Close() // 写入已完成，关闭错误不影响产物
			_ = f.Close()  // 写入已完成，关闭错误不影响产物
			return err
		}
	}
	if err := tw.Close(); err != nil {
		_ = f.Close() // 写入已完成，关闭错误不影响产物
		return err
	}
	if err := gz.Close(); err != nil {
		_ = f.Close() // 写入已完成，关闭错误不影响产物
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, destTarGz)
}

func addTree(tw *tar.Writer, fsPath, arcName string) error {
	info, err := os.Lstat(fsPath)
	if err != nil {
		return err
	}

	link := ""
	if info.Mode()&os.ModeSymlink != 0 {
		if link, err = os.Readlink(fsPath); err != nil {
			return err
		}
	}

	hdr, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	hdr.Name = arcName
	if info.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
		hdr.Name += "/"
	}
	// 对齐 Python tarfile 的 uname/gname 填充行为
	if cur, err := user.Current(); err == nil {
		hdr.Uname = cur.Username
		if grp, err := user.LookupGroupId(cur.Gid); err == nil {
			hdr.Gname = grp.Name
		}
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	// 目录必须递归：Python 的 tar.add 对目录会自动展开子项
	if info.IsDir() {
		children, err := os.ReadDir(fsPath)
		if err != nil {
			return err
		}
		for _, child := range children {
			if err := addTree(tw, filepath.Join(fsPath, child.Name()), path.Join(arcName, child.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return nil
	}

	f, err := os.Open(fsPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(tw, f)
	return err
}
