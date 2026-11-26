package cli

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"bksmartpacker/pkg/smartpkg"
)

// Inspect 输出 s-mart 包的结构摘要。
func Inspect(path, format string) error {
	rep, err := smartpkg.Inspect(path)
	if err != nil {
		return errors.Wrap(err, "inspect s-mart package")
	}
	if format == "json" {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("path: %s\nsize: %d\nversion: %s\nhas_app_desc: %v\n", rep.Path, rep.Size, rep.Version, rep.HasDesc)
	fmt.Println("files:")
	for _, f := range rep.Files {
		fmt.Printf("  - %s\n", f)
	}
	fmt.Println("layers:")
	for _, l := range rep.Layers {
		fmt.Printf("  - %s (%d bytes) has_source=%v\n", l.Name, l.Size, l.HasSource)
		for _, e := range l.Entries {
			fmt.Printf("      %s\n", e)
		}
	}
	return nil
}
