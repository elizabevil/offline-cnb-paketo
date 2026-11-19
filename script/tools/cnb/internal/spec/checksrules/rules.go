// Package checksrules parses lint rule overrides (TOML protocol boundary).
package checksrules

import "github.com/pelletier/go-toml/v2"

// Level values understood by the lint engine.
const (
	LevelError = "error"
	LevelWarn  = "warn"
)

// Override adjusts one built-in rule. Rules themselves live in code
// (internal/checks); this file only carries what may change without a rebuild.
type Override struct {
	ID      string `toml:"id"`
	Level   string `toml:"level,omitempty"`
	Enabled *bool  `toml:"enabled,omitempty"`
	Message string `toml:"message,omitempty"`
}

// Ruleset is a set of overrides plus the provider buildpack list.
type Ruleset struct {
	Schema    int        `toml:"schema"`
	Providers []string   `toml:"providers"`
	Overrides []Override `toml:"rule"`
}

// IsProvider reports whether a component must declare external dependencies.
func (r *Ruleset) IsProvider(name string) bool {
	for _, p := range r.Providers {
		if p == name {
			return true
		}
	}
	return false
}

// Lookup returns the override for a rule id, if any.
func (r *Ruleset) Lookup(id string) (Override, bool) {
	for _, o := range r.Overrides {
		if o.ID == id {
			return o, true
		}
	}
	return Override{}, false
}

// Parse decodes a ruleset from TOML.
func Parse(data []byte) (*Ruleset, error) {
	var rs Ruleset
	if err := toml.Unmarshal(data, &rs); err != nil {
		return nil, err
	}
	return &rs, nil
}
