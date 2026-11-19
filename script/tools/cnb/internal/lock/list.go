package lock

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
)

// WriteList 以 TSV 输出清单（id、version、kind、平台）。
func WriteList(w io.Writer, f *File) {
	for _, a := range f.Assets {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.ID, a.Version, a.Unpack.Kind, strings.Join(slices.Sorted(maps.Keys(a.Platform)), ","))
	}
}

// Get 输出 shell 赋值，供脚本 eval。字段名是契约，勿改。
