package cli

// failExecution wraps an error as an execution failure.
func failExecution(err error) error {
	return &exitError{code: ExitFailure, msg: err.Error()}
}

// failUsage wraps an error as a usage error.
func failUsage(err error) error {
	return &exitError{code: ExitUsage, msg: err.Error()}
}
