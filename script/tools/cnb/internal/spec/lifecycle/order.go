// Package lifecycle 提供 CNB lifecycle 文件协议的解析与序列化。
package lifecycle

import (
	"bytes"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// Order 是 order.toml 的结构化表示。
type Order struct {
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

// ParseOrder 解析 order.toml。
func ParseOrder(data []byte) (*Order, []errs.Warning, error) {
	var o Order
	if err := toml.Unmarshal(data, &o); err != nil {
		return nil, nil, err
	}
	return &o, orderWarnings(data), nil
}

// Encode 序列化为 TOML。
func (o *Order) Encode() ([]byte, error) {
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.SetIndentTables(true)
	if err := enc.Encode(o); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Filter 只保留被 required 命中的 order 块；required 为空时返回原样。
// required 形如 "type name ... ver;type name ... ver"。
func (o *Order) Filter(required string, aliases map[string]string) *Order {
	if strings.TrimSpace(required) == "" {
		return o
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
	kept := &Order{}
	for _, e := range o.Order {
		for _, g := range e.Group {
			if want[normalize(g.ID)] {
				kept.Order = append(kept.Order, e)
				break
			}
		}
	}
	return kept
}

// normalize 把 / 与 _ 视作等价，便于跨命名风格比较。
func normalize(s string) string {
	return strings.NewReplacer("/", "_", "_", "_").Replace(strings.ToLower(s))
}

// orderWarnings 报告顶层未知键（协议升级信号）。
func orderWarnings(data []byte) []errs.Warning {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var out []errs.Warning
	for k := range raw {
		if k != "order" {
			out = append(out, errs.Warning{Path: k, Message: "unknown top-level key in order.toml"})
		}
	}
	return out
}
