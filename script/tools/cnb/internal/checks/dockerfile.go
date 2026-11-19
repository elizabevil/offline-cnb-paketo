package checks

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
)

// DockerfileSourceFinding reports one missing COPY/ADD source.
type DockerfileSourceFinding struct {
	Stage    string
	Source   string
	Resolved string
}

// Describe renders a finding for CLI output.
func (f DockerfileSourceFinding) Describe() string {
	return fmt.Sprintf("COPY source not found: %s (resolved: %s)", f.Source, f.Resolved)
}

// CheckDockerfiles verifies that every literal COPY/ADD source of the given
// Dockerfiles exists on disk. Targets come from spec.checks, never from argv,
// so stage command formatting cannot break the check.
//
// Sources containing shell variables are skipped: they are expanded at build
// time and cannot be checked statically.
func CheckDockerfiles(targets []DockerfileTarget, bundleDir, buildpacksDir string) []DockerfileSourceFinding {
	var out []DockerfileSourceFinding
	for _, t := range targets {
		vars := dockerfileVars(t.Dockerfile, t.Args)
		for _, src := range copySources(t.Dockerfile) {
			src.path = expandVars(src.path, vars)
			resolved, ok := resolveSource(src, t.Context, t.Contexts, bundleDir, buildpacksDir)
			if !ok {
				continue
			}
			if !fsutil.Exists(resolved) {
				out = append(out, DockerfileSourceFinding{Stage: t.Name, Source: src.path, Resolved: resolved})
			}
		}
	}
	return out
}

// DockerfileTarget is one Dockerfile plus the contexts and variables its COPY
// sources use.
type DockerfileTarget struct {
	Name       string
	Dockerfile string
	Context    string
	Contexts   map[string]string
	// Args are the build args (same meaning as --build-arg) used to expand
	// variables in COPY sources.
	Args map[string]string
}

// dockerfileVars collects ARG/ENV defaults from the Dockerfile, then overlays
// explicitly configured build args. Later values win, mirroring Docker.
func dockerfileVars(dockerfile string, args map[string]string) map[string]string {
	vars := map[string]string{}
	data, err := os.ReadFile(dockerfile)
	if err == nil {
		for line := range strings.SplitSeq(string(data), "\n") {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) < 2 {
				continue
			}
			switch strings.ToUpper(fields[0]) {
			case "ARG", "ENV":
				name, value, ok := strings.Cut(fields[1], "=")
				if !ok || value == "" {
					continue // ARG without default: resolved by --build-arg or skipped
				}
				vars[name] = value
			}
		}
	}
	maps.Copy(vars, args)
	return vars
}

// expandVars substitutes ${NAME} and $NAME using known values. Unknown
// variables are left untouched so resolveSource can skip the source.
func expandVars(s string, vars map[string]string) string {
	for name, value := range vars {
		s = strings.ReplaceAll(s, "${"+name+"}", value)
		s = strings.ReplaceAll(s, "$"+name, value)
	}
	return s
}

// copySource is one COPY/ADD source with its optional --from value.
type copySource struct {
	from string
	path string
}

// copySources parses COPY and ADD instructions from a Dockerfile.
func copySources(dockerfile string) []copySource {
	data, err := os.ReadFile(dockerfile)
	if err != nil {
		return nil
	}
	var out []copySource
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 {
			continue
		}
		if !strings.EqualFold(fields[0], "COPY") && !strings.EqualFold(fields[0], "ADD") {
			continue
		}
		var from string
		var paths []string
		for _, f := range fields[1:] {
			switch {
			case strings.HasPrefix(f, "--from="):
				from = strings.TrimPrefix(f, "--from=")
			case strings.HasPrefix(f, "--"):
				// other flags (--chown, --chmod) carry no path
			default:
				paths = append(paths, f)
			}
		}
		if len(paths) < 2 {
			continue
		}
		for _, path := range paths[:len(paths)-1] {
			out = append(out, copySource{from: from, path: path})
		}
	}
	return out
}

// resolveSource maps a COPY source to an absolute path.
//
// The second return value is false when the source cannot be resolved
// statically (variables, multi-stage images, missing context).
func resolveSource(src copySource, contextDir string, contexts map[string]string, bundleDir, buildpacksDir string) (string, bool) {
	if strings.Contains(src.path, "$") {
		return "", false
	}
	if ctx, ok := contexts[src.from]; ok {
		return filepath.Join(ctx, src.path), true
	}
	switch src.from {
	case "":
		return filepath.Join(contextDir, src.path), true
	case "bundle":
		return filepath.Join(bundleDir, src.path), true
	case "buildpacks":
		return filepath.Join(buildpacksDir, src.path), true
	default:
		return "", false // named stage or image: resolved by the builder
	}
}
