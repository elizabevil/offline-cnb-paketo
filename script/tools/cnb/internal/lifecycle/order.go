package lifecycle

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// OrderDoc 是 CNB order.toml 的结构化表示。
type OrderDoc struct {
	Order []OrderEntry `toml:"order"`
}

// OrderEntry 是一个 order 块（可含多个 group）。
type OrderEntry struct {
	Group []Group `toml:"group"`
}

// Group 是 order 中的一个 buildpack 引用。
type Group struct {
	ID       string `toml:"id"`
	Version  string `toml:"version,omitempty"`
	Optional bool   `toml:"optional,omitempty"`
}

// Aliases 把外部平台（BK）的 buildpack 名映射到本项目 order 里的 id。
// 命中判定会同时尝试原名与别名，因此 heroku（order 用 bk-buildpack-*）
// 与 paketo（order 用 paketo-buildpacks/*）都能匹配。
var Aliases = map[string]string{
	"bk-buildpack-go":     "paketo-buildpacks/go",
	"bk-buildpack-nodejs": "paketo-buildpacks/nodejs",
	"bk-buildpack-python": "paketo-buildpacks/python",
}

// normalize 把 / 与 _ 视作等价，便于跨命名风格比较。
func normalize(s string) string {
	return strings.NewReplacer("/", "_", "_", "_").Replace(strings.ToLower(s))
}

// FilterOrder 读取 src，只保留被 required 命中的 order 块。
// required 形如 "type name ... ver;type name ... ver"；为空则原样返回。
func FilterOrder(src, required string, aliases map[string]string) ([]byte, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(required) == "" {
		return data, nil
	}

	var doc OrderDoc
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("解析 %s: %w", src, err)
	}

	want := map[string]bool{}
	for entry := range strings.SplitSeq(required, ";") {
		fields := strings.Fields(entry)
		if len(fields) < 2 {
			continue
		}
		name := fields[1]
		want[normalize(name)] = true
		if alias, ok := aliases[name]; ok {
			want[normalize(alias)] = true
		}
	}

	var kept OrderDoc
	for _, entry := range doc.Order {
		for _, g := range entry.Group {
			if want[normalize(g.ID)] {
				kept.Order = append(kept.Order, entry)
				break
			}
		}
	}
	if len(kept.Order) == 0 {
		var ids []string
		for _, e := range doc.Order {
			for _, g := range e.Group {
				ids = append(ids, g.ID)
			}
		}
		return nil, fmt.Errorf("REQUIRED_BUILDPACKS 与 %s 无交集（需要 %v，实际 %v）", src, keys(want), ids)
	}
	return toml.Marshal(kept)
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
