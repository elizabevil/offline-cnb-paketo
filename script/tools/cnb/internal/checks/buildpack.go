// Package checks runs static checks for buildpack layout, Dockerfile sources,
// environment health, and builder metadata.
//
// Rule text lives next to the check that enforces it; default_rules.toml can
// override level, enabled, and message without a rebuild.
package checks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	specchecks "learn-cnb-paketo/tools/cnb/internal/spec/checksrules"
	speclifecycle "learn-cnb-paketo/tools/cnb/internal/spec/lifecycle"
)

// File and layout conventions.
const (
	ManifestName = "buildpack.toml"
	DetectScript = "detect"
	BuildScript  = "build"
	BinDir       = "bin"

	IDPrefix        = "paketo-buildpacks_"
	IDPrefixSlash   = "paketo-buildpacks/"
	FileModeExec    = 0o111
	SBOMMarker      = ".sbom."
	defaultLevelErr = specchecks.LevelError
)

// Rule scopes.
const (
	scopeAll       = "all"
	scopeMeta      = "meta"
	scopeComponent = "component"
)

// Finding is one check result.
type Finding struct {
	Name    string
	Message string
	IsError bool
}

// BuildpackReport is the result for one buildpack directory.
type BuildpackReport struct {
	Findings []Finding
}

// Errors returns the error count.
func (r BuildpackReport) Errors() int {
	n := 0
	for _, f := range r.Findings {
		if f.IsError {
			n++
		}
	}
	return n
}

// Warnings returns the warning count.
func (r BuildpackReport) Warnings() int { return len(r.Findings) - r.Errors() }

// OK reports whether there are no errors.
func (r BuildpackReport) OK() bool { return r.Errors() == 0 }

// buildpack is the parsed view used by the checks.
type buildpack struct {
	leaf    string
	dir     string
	readErr error
	parsed  *speclifecycle.Buildpack
	parseOK bool
}

// isMeta reports whether the buildpack is an order-only meta buildpack.
func (b *buildpack) isMeta() bool {
	return b.parsed != nil && len(b.parsed.Order) > 0
}

// rule is one enforceable check plus its default presentation.
type rule struct {
	id      string
	scope   string
	level   string
	message string
	run     func(*buildpack, *Ruleset) []string // reasons; empty means pass
}

// ruleTable is the single place where checks are declared.
func ruleTable() []rule {
	return []rule{
		{"manifest-exists", scopeAll, defaultLevelErr, "missing buildpack.toml", checkManifestExists},
		{"manifest-parses", scopeAll, defaultLevelErr, "invalid buildpack.toml", checkManifestParses},
		{"api-declared", scopeAll, defaultLevelErr, "missing api", requireField(func(b *speclifecycle.Buildpack) string { return b.API })},
		{"id-declared", scopeAll, defaultLevelErr, "missing [buildpack] id", requireField(func(b *speclifecycle.Buildpack) string { return b.BuildpackInfo.ID })},
		{"version-declared", scopeAll, defaultLevelErr, "missing [buildpack] version", requireField(func(b *speclifecycle.Buildpack) string { return b.BuildpackInfo.Version })},
		{"name-declared", scopeAll, defaultLevelErr, "missing [buildpack] name", requireField(func(b *speclifecycle.Buildpack) string { return b.BuildpackInfo.Name })},
		{"dir-id-match", scopeAll, defaultLevelErr, "directory name does not match buildpack id", checkDirIDMatch},
		{"meta-no-scripts", scopeMeta, defaultLevelErr, "meta buildpack must not ship bin scripts", forbidScripts},
		{"order-refs-exist", scopeMeta, defaultLevelErr, "order references missing buildpack", checkOrderRefs},
		{"detect-present", scopeComponent, defaultLevelErr, "missing bin/detect", requireScript(DetectScript)},
		{"build-present", scopeComponent, defaultLevelErr, "missing bin/build", requireScript(BuildScript)},
		{"scripts-executable", scopeComponent, defaultLevelErr, "script is not executable", checkScriptsExecutable},
		{"sbom-formats-declared", scopeComponent, specchecks.LevelWarn, "writes SBOM but declares no sbom-formats", checkSBOMDeclared},
		{"dependencies-declared", scopeComponent, specchecks.LevelWarn, "provider buildpack without [[metadata.dependencies]]", checkProviderDeps},
	}
}

// CheckBuildpack checks one directory with the embedded default ruleset.
func CheckBuildpack(dir string) BuildpackReport {
	rs, err := DefaultRuleset()
	if err != nil {
		return BuildpackReport{Findings: []Finding{{Name: filepath.Base(dir), Message: err.Error(), IsError: true}}}
	}
	return CheckBuildpackWith(dir, rs)
}

// CheckBuildpackWith checks one directory against the given ruleset.
func CheckBuildpackWith(dir string, rs *Ruleset) BuildpackReport {
	bp := loadBuildpack(dir)
	var rep BuildpackReport
	for _, r := range ruleTable() {
		if !inScope(r.scope, bp) {
			continue
		}
		level, message, enabled := r.level, r.message, true
		if o, ok := rs.Lookup(r.id); ok {
			if o.Level != "" {
				level = o.Level
			}
			if o.Message != "" {
				message = o.Message
			}
			if o.Enabled != nil {
				enabled = *o.Enabled
			}
		}
		if !enabled {
			continue
		}
		for _, reason := range r.run(bp, rs) {
			rep.Findings = append(rep.Findings, Finding{
				Name: bp.leaf, Message: withReason(message, reason), IsError: level == specchecks.LevelError,
			})
		}
	}
	return rep
}

// loadBuildpack locates and parses buildpack.toml, tolerating <id>/<version>.
func loadBuildpack(dir string) *buildpack {
	bp := &buildpack{leaf: filepath.Base(dir), dir: dir}
	if !fsutil.Exists(filepath.Join(dir, ManifestName)) {
		if versionDir := firstVersionDir(dir); versionDir != "" {
			bp.dir = versionDir
		}
	}
	data, err := os.ReadFile(filepath.Join(bp.dir, ManifestName))
	if err != nil {
		bp.readErr = err
		return bp
	}
	parsed, _, err := speclifecycle.ParseBuildpack(data)
	if err == nil && parsed != nil {
		bp.parsed, bp.parseOK = parsed, true
	}
	return bp
}

// firstVersionDir returns the first subdirectory holding a buildpack.toml.
func firstVersionDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		cand := filepath.Join(dir, e.Name())
		if e.IsDir() && fsutil.Exists(filepath.Join(cand, ManifestName)) {
			return cand
		}
	}
	return ""
}

func inScope(scope string, bp *buildpack) bool {
	switch scope {
	case scopeMeta:
		return bp.isMeta()
	case scopeComponent:
		return bp.parsed != nil && !bp.isMeta()
	default:
		return true
	}
}

// withReason appends a concrete reason to the rule message.
func withReason(message, reason string) string {
	if reason == "" {
		return message
	}
	return fmt.Sprintf("%s: %s", message, reason)
}

// ── individual checks ──────────────────────────────────────────────────────

func checkManifestExists(bp *buildpack, _ *Ruleset) []string {
	if bp.readErr != nil {
		return []string{""}
	}
	return nil
}

func checkManifestParses(bp *buildpack, _ *Ruleset) []string {
	if bp.readErr != nil || bp.parseOK {
		return nil
	}
	return []string{"unparsable"}
}

// requireField reports a missing manifest field.
func requireField(get func(*speclifecycle.Buildpack) string) func(*buildpack, *Ruleset) []string {
	return func(bp *buildpack, _ *Ruleset) []string {
		if bp.parsed != nil && get(bp.parsed) != "" {
			return nil
		}
		return []string{""}
	}
}

// forbidScripts reports a meta buildpack that ships build scripts.
func forbidScripts(bp *buildpack, _ *Ruleset) []string {
	if fsutil.Exists(filepath.Join(bp.dir, BinDir, DetectScript)) ||
		fsutil.Exists(filepath.Join(bp.dir, BinDir, BuildScript)) {
		return []string{"bin/ contains scripts"}
	}
	return nil
}

func checkDirIDMatch(bp *buildpack, _ *Ruleset) []string {
	if bp.parsed == nil || bp.parsed.BuildpackInfo.ID == "" || idMatchesDir(bp.parsed.BuildpackInfo.ID, bp.leaf) {
		return nil
	}
	return []string{fmt.Sprintf("dir %s vs id %s", bp.leaf, bp.parsed.BuildpackInfo.ID)}
}

func checkOrderRefs(bp *buildpack, _ *Ruleset) []string {
	if bp.parsed == nil {
		return nil
	}
	root := filepath.Dir(bp.dir)
	var missing []string
	for _, g := range bp.parsed.Order {
		if g.ID == "" || orderTargetExists(root, g.ID) {
			continue
		}
		missing = append(missing, g.ID)
	}
	if len(missing) == 0 {
		return nil
	}
	return []string{strings.Join(missing, ", ")}
}

func orderTargetExists(root, id string) bool {
	leaf := strings.TrimPrefix(id, IDPrefixSlash)
	return fsutil.Exists(filepath.Join(root, IDPrefix+leaf)) || fsutil.Exists(filepath.Join(root, leaf))
}

// requireScript reports a missing bin script.
func requireScript(name string) func(*buildpack, *Ruleset) []string {
	return func(bp *buildpack, _ *Ruleset) []string {
		if fsutil.Exists(filepath.Join(bp.dir, BinDir, name)) {
			return nil
		}
		return []string{"not found"}
	}
}

func checkScriptsExecutable(bp *buildpack, _ *Ruleset) []string {
	var notExec []string
	for _, name := range []string{DetectScript, BuildScript} {
		info, err := os.Stat(filepath.Join(bp.dir, BinDir, name))
		if err != nil {
			continue
		}
		if info.Mode()&FileModeExec == 0 {
			notExec = append(notExec, name)
		}
	}
	if len(notExec) == 0 {
		return nil
	}
	return []string{strings.Join(notExec, ", ")}
}

func checkSBOMDeclared(bp *buildpack, _ *Ruleset) []string {
	if !hasSBOMMarker(bp.dir) || hasSBOMFormats(bp.parsed) {
		return nil
	}
	return []string{"sbom-formats is empty"}
}

func hasSBOMFormats(bp *speclifecycle.Buildpack) bool {
	return bp != nil && len(bp.BuildpackInfo.SBOMFormats) > 0
}

func checkProviderDeps(bp *buildpack, rs *Ruleset) []string {
	if !rs.IsProvider(bp.leaf) || hasDependencies(bp.parsed) {
		return nil
	}
	return []string{"dependencies is empty"}
}

func hasDependencies(bp *speclifecycle.Buildpack) bool {
	return bp != nil && len(bp.MetadataSection.Dependencies) > 0
}

// hasSBOMMarker reports whether the build scripts mention an SBOM file name.
func hasSBOMMarker(dir string) bool {
	for _, name := range []string{DetectScript, BuildScript} {
		data, err := os.ReadFile(filepath.Join(dir, BinDir, name))
		if err == nil && strings.Contains(string(data), SBOMMarker) {
			return true
		}
	}
	return false
}

// idMatchesDir reports whether a directory name matches a buildpack id.
func idMatchesDir(id, leaf string) bool {
	if id == "" {
		return true
	}
	last := id[strings.LastIndex(id, "/")+1:]
	return leaf == IDPrefix+strings.TrimPrefix(id, IDPrefixSlash) || leaf == last
}
