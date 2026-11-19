package cli

// CLI-scoped constants.
const (
	// DefaultLockName matches the workspace package.
	DefaultManifestName = "assets.toml"
	// DefaultConfigDir is the default pipeline config directory.
	DefaultConfigDir = "config/projects"

	// ExitOK means success.
	ExitOK = 0
	// ExitFailure means execution failed.
	ExitFailure = 1
	// ExitUsage means invalid arguments or usage.
	ExitUsage = 2

	// BundleContext is the named build context for material references.
	BundleContext = "bundle"
	// BuildpacksContext is the named build context for buildpacks.
	BuildpacksContext = "buildpacks"
)
