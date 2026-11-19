package checks

import (
	_ "embed"
	"os"

	specchecks "learn-cnb-paketo/tools/cnb/internal/spec/checksrules"
)

//go:embed default_rules.toml
var defaultRules []byte

// Ruleset is a set of rule overrides plus the provider list.
type Ruleset = specchecks.Ruleset

// DefaultRuleset returns the embedded ruleset.
func DefaultRuleset() (*Ruleset, error) { return LoadRuleset("") }

// LoadRuleset reads overrides from path; an empty path uses the embedded set.
func LoadRuleset(path string) (*Ruleset, error) {
	data := defaultRules
	if path != "" {
		read, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		data = read
	}
	return specchecks.Parse(data)
}
