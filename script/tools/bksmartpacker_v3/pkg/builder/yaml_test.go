package builder

import "gopkg.in/yaml.v3"

func yamlUnmarshal(data string, out interface{}) error {
	return yaml.Unmarshal([]byte(data), out)
}
