package imagefmt

import "encoding/json"

// StandardPathDirs 是运行期必须出现在 PATH 里的目录。
var StandardPathDirs = []string{
	"/usr/local/sbin", "/usr/local/bin", "/usr/sbin", "/usr/bin", "/sbin", "/bin",
}

// EnsureStandardPath 保证 config 的 PATH 含标准目录（缺则追加）。
// 只改动 config.Env 中 PATH 一项，其余字段原样保留。
func (i *Image) EnsureStandardPath() error {
	if len(i.ConfigRaw) == 0 {
		return nil
	}
	var root map[string]any
	if err := json.Unmarshal(i.ConfigRaw, &root); err != nil {
		return err
	}
	cfg, _ := root["config"].(map[string]any)
	if cfg == nil {
		return nil
	}
	envs, _ := cfg["Env"].([]any)
	seen := map[string]bool{}
	for _, e := range envs {
		s, _ := e.(string)
		if k, _, ok := cutKV(s); ok {
			seen[k] = true
		}
	}
	pathValue := ""
	for idx, e := range envs {
		s, _ := e.(string)
		if k, v, ok := cutKV(s); ok && k == "PATH" {
			pathValue = v
			envs[idx] = "PATH=" + withStandardDirs(v)
		}
	}
	if pathValue == "" {
		envs = append(envs, "PATH="+withStandardDirs(""))
	}
	cfg["Env"] = envs
	data, err := json.Marshal(root)
	if err != nil {
		return err
	}
	i.ConfigRaw = data
	return nil
}

// withStandardDirs 把缺失的标准目录追加到 PATH 末尾。
func withStandardDirs(path string) string {
	have := map[string]bool{}
	for _, p := range splitPath(path) {
		have[p] = true
	}
	out := path
	for _, d := range StandardPathDirs {
		if have[d] {
			continue
		}
		if out == "" {
			out = d
		} else if len(out) > 0 && out[len(out)-1] == ':' {
			out += d
		} else {
			out += ":" + d
		}
	}
	return out
}

// splitPath 按冒号切分 PATH（忽略空段）。
func splitPath(p string) []string {
	var out []string
	start := 0
	for idx := 0; idx <= len(p); idx++ {
		if idx == len(p) || p[idx] == ':' {
			if idx > start {
				out = append(out, p[start:idx])
			}
			start = idx + 1
		}
	}
	return out
}

// cutKV 拆分 K=V。
func cutKV(s string) (key, value string, ok bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}
