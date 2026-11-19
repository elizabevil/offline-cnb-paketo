package errs

import "fmt"

// Warning is a non-fatal problem found while parsing a protocol file
// (unknown field, suspicious value, …).
type Warning struct {
	// Path is the offending field path, e.g. "order[0].group[1].id".
	Path string
	// Message explains the problem.
	Message string
}

// String renders a warning for logs.
func (w Warning) String() string { return fmt.Sprintf("%s: %s", w.Path, w.Message) }
