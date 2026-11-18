package lock

import (
	"fmt"
	"sort"

	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
)

// UpdateRequest 描述一次物料版本更新。
type UpdateRequest struct {
	ID      string
	Version string            // 旧版本；空表示按 id 唯一匹配
	New     string            // 新版本；空表示沿用旧版本
	URLs    map[string]string // 平台 → 显式 URL（可选）
}

// UpdateVersion 更新物料版本及 URL；返回是否有改动。
func UpdateVersion(path string, req UpdateRequest) (bool, error) {
	return Update(path, func(file *manifest.File) error {
		idx, err := locateAsset(file, req)
		if err != nil {
			return err
		}
		a := &file.Assets[idx]
		if req.New != "" {
			a.Version = req.New
		}
		for platform, url := range req.URLs {
			src, ok := a.Platform[platform]
			if !ok {
				return fmt.Errorf("%s 缺少平台 %s", a.ID, platform)
			}
			src.URL = url
			a.Platform[platform] = src
		}
		return nil
	})
}

// locateAsset 按 id（可带版本）定位物料。
func locateAsset(file *manifest.File, req UpdateRequest) (int, error) {
	var hits []int
	for i := range file.Assets {
		a := &file.Assets[i]
		if a.ID != req.ID {
			continue
		}
		if req.Version != "" && a.Version != req.Version {
			continue
		}
		hits = append(hits, i)
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return 0, fmt.Errorf("%s 未找到", req.ID)
	default:
		versions := make([]string, 0, len(hits))
		for _, i := range hits {
			versions = append(versions, file.Assets[i].Version)
		}
		sort.Strings(versions)
		return 0, fmt.Errorf("%s 存在多个版本（%v），请指定 id@version", req.ID, versions)
	}
}
