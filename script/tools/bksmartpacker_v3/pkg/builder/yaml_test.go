package builder

import "gopkg.in/yaml.v3"

func yamlUnmarshal(data string, out any) error {
	return yaml.Unmarshal([]byte(data), out)
}
