// Package scaffold creates a new project from a reference project.
//
// Rendering hand-written templates drifted from the real projects, so the
// source of truth is now a working project directory: copy it, then rewrite
// the project name and directory.
package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
)

// Options are the inputs for a scaffold run.
type Options struct {
	// Name is the new project name.
	Name string
	// Dir is the new project directory (e.g. project/demo).
	Dir string
	// From is the reference project directory.
	From string
	// Only / Skip filter files by relative path prefix (e.g. "docker").
	Only []string
	Skip []string
	// Assets is an optional assets.toml to copy into the project.
	Assets string
}

// File is one output file.
type File struct {
	// Path is relative to Options.Dir.
	Path    string
	Content []byte
}

// Render copies the reference project and rewrites name/directory references.
//
// Every candidate file — including the optional assets.toml — goes through the
// same only/skip filter, so filtering has one rule instead of special cases.
func Render(o Options) ([]File, error) {
	ref, err := filepath.Abs(o.From)
	if err != nil {
		return nil, err
	}
	if !fsutil.DirExists(ref) {
		return nil, fmt.Errorf("reference project not found: %s", o.From)
	}
	oldName := filepath.Base(ref)
	oldDir := projectRelDir(ref)

	candidates, err := collect(ref, o.Skip)
	if err != nil {
		return nil, err
	}
	if o.Assets != "" && fsutil.Exists(o.Assets) {
		candidates = append(candidates, candidate{rel: "assets.toml", path: o.Assets})
	}

	var files []File
	for _, c := range candidates {
		if !selected(c.rel, o.Only, o.Skip) {
			continue
		}
		content, err := os.ReadFile(c.path)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: c.rel, Content: rewrite(content, oldName, o.Name, oldDir, o.Dir)})
	}
	return files, nil
}

// candidate is one file considered for copying.
type candidate struct {
	rel  string
	path string
}

// collect walks the reference project, pruning skipped directories early.
func collect(ref string, skip []string) ([]candidate, error) {
	var out []candidate
	err := filepath.WalkDir(ref, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(ref, path)
		if err != nil || rel == "." {
			return err
		}
		if d.IsDir() {
			if skipped(rel, skip) {
				return filepath.SkipDir
			}
			return nil
		}
		out = append(out, candidate{rel: rel, path: path})
		return nil
	})
	return out, err
}

// rewrite replaces project name and directory references in text files.
func rewrite(content []byte, oldName, newName, oldDir, newDir string) []byte {
	if isBinary(content) {
		return content
	}
	s := string(content)
	if oldDir != "" && newDir != "" {
		s = strings.ReplaceAll(s, oldDir, newDir)
	}
	s = strings.ReplaceAll(s, oldName, newName)
	return []byte(s)
}

// isBinary reports whether content looks binary (NUL byte in the head).
func isBinary(content []byte) bool {
	head := content
	if len(head) > 8000 {
		head = head[:8000]
	}
	return slices.Contains(head, byte(0))
}

// projectRelDir returns "<parent>/<name>" style path from the repo root, based
// on the conventional project/<name> layout.
func projectRelDir(abs string) string {
	parent := filepath.Base(filepath.Dir(abs))
	name := filepath.Base(abs)
	if parent == "." || parent == string(filepath.Separator) {
		return name
	}
	return filepath.Join(parent, name)
}

// selected applies the only/skip filters with one rule:
// a pattern matches when it equals the path, is a parent directory, or is a
// glob (filepath.Match) against the path.
func selected(rel string, only, skip []string) bool {
	if len(only) > 0 && !matches(rel, only) {
		return false
	}
	return !matches(rel, skip)
}

// matches reports whether any pattern selects rel.
func matches(rel string, patterns []string) bool {
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if rel == p || strings.HasPrefix(rel, p+string(filepath.Separator)) {
			return true
		}
		if ok, _ := filepath.Match(p, rel); ok {
			return true
		}
	}
	return false
}

func skipped(rel string, skip []string) bool { return matches(rel, skip) }
