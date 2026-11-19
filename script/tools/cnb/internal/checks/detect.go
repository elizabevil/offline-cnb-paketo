package checks

import (
	"path/filepath"
	"slices"
	"strings"
)

// shellCommands are interpreters that may run a script argument.
var shellCommands = []string{"bash", "sh", "zsh", "ash", "dash"}

// IsShellScript reports whether argv is "<interpreter> <script>" rather than
// "<interpreter> -flag".
func IsShellScript(argv []string) bool {
	if len(argv) < 2 || strings.HasPrefix(argv[1], "-") {
		return false
	}
	return slices.Contains(shellCommands, filepath.Base(argv[0]))
}
