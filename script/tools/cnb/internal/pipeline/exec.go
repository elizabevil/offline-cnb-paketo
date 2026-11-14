package pipeline

import "os/exec"

// execCommand 是 exec.Command 的薄封装，便于测试替换。
func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
