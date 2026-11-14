package strutil

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
