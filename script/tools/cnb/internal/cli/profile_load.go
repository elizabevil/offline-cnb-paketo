package cli

import (
	"os"

	specprofile "learn-cnb-paketo/tools/cnb/internal/spec/profile"
	specwarn "learn-cnb-paketo/tools/cnb/internal/spec/warn"
)

// loadProfileFile reads and parses a pipeline profile.
// Errors are already wrapped as exitErrors suitable for returning from RunE.
func loadProfileFile(path string) (*specprofile.File, []specwarn.Warning, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, failExecution(err)
	}
	f, warns, err := specprofile.Parse(data)
	if err != nil {
		return nil, nil, failExecution(err)
	}
	return f, warns, nil
}
