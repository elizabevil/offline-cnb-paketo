package materials

// 物料布局与标记文件约定（与 script/lib/common.sh、fetch.sh 保持一致）。
const (
	// DirArchives 存放下载归档。
	DirArchives = "archives"
	// DirBinaries 存放解包后的产物。
	DirBinaries = "binaries"
	// DirMeta 存放解压标记。
	DirMeta = ".meta"

	// MarkerSuffix 是解压标记文件名后缀。
	MarkerSuffix = ".sha256"

	// SyncSuffix 是同步标记（JSON）的后缀，与 legacy MarkerSuffix 区分。
	SyncSuffix = ".sync.json"

	// MarkerUnpinned 表示物料未固定 sha256，但已解压。
	MarkerUnpinned = "unpinned"
)
