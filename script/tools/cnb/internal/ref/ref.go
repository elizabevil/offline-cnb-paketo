// Package ref 提供物料引用的解析与构造，避免各处重复拆字符串。
package ref

import (
	"fmt"
	"strings"
)

// Ref 是解析后的物料引用 id@version。
type Ref struct {
	ID      string
	Version string
}

// Parse 解析 "id@version"；没有 @ 时 Version 为空。
func Parse(s string) (Ref, error) {
	if s == "" {
		return Ref{}, fmt.Errorf("empty ref")
	}
	id, version, hasVer := strings.Cut(s, "@")
	if id == "" {
		return Ref{}, fmt.Errorf("invalid ref %q: empty id", s)
	}
	if hasVer && version == "" {
		return Ref{}, fmt.Errorf("invalid ref %q: empty version", s)
	}
	return Ref{ID: id, Version: version}, nil
}

// MustParse 用于已知合法的静态字符串。
func MustParse(s string) Ref {
	r, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return r
}

// String 还原为 id@version 形式。
func (r Ref) String() string {
	if r.Version == "" {
		return r.ID
	}
	return r.ID + "@" + r.Version
}

// Matches 判断 ref 是否匹配通配符模式。
//
// 支持两种通配：
//
//   - 匹配除 "/" ":" "@" 之外的任意字符
//     ** 匹配任意字符（含分隔符）
func (r Ref) Matches(pattern string) bool {
	return Match(pattern, r.String())
}

// Match 是通配符匹配的通用实现。
func Match(pattern, s string) bool {
	return matchRunes([]rune(pattern), []rune(s))
}

// matchRunes 递归匹配，只支持 * 与 **。
func matchRunes(p, s []rune) bool {
	for len(p) > 0 {
		switch {
		case len(p) >= 2 && p[0] == '*' && p[1] == '*':
			rest := p[2:]
			for i := 0; i <= len(s); i++ {
				if matchRunes(rest, s[i:]) {
					return true
				}
			}
			return false
		case p[0] == '*':
			rest := p[1:]
			for i := 0; i <= len(s); i++ {
				if matchRunes(rest, s[i:]) {
					return true
				}
				if i == len(s) || isSeparator(s[i]) {
					break
				}
			}
			return false
		default:
			if len(s) == 0 || p[0] != s[0] {
				return false
			}
			p, s = p[1:], s[1:]
		}
	}
	return len(s) == 0
}

// isSeparator 判断字符是否阻断单星号匹配。
func isSeparator(r rune) bool {
	return r == '/' || r == ':' || r == '@'
}
