package materials

// Bundle layout and marker conventions.
const (
	// DirArchives holds downloaded archives.
	DirArchives = "archives"
	// DirBinaries holds unpacked artifacts.
	DirBinaries = "binaries"
	// DirMeta holds unpack markers.
	DirMeta = ".meta"

	// MarkerSuffix is the legacy unpack marker suffix.
	MarkerSuffix = ".sha256"

	// SyncSuffix is the sync marker (JSON) suffix, distinct from MarkerSuffix.
	SyncSuffix = ".sync.json"

	// MarkerUnpinned marks an unpacked material without a pinned sha256.
	MarkerUnpinned = "unpinned"
)
