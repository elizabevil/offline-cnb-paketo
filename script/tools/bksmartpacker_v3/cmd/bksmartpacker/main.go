// Package main 提供 bksmartpacker 命令行入口。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/pkg/errors"

	"bksmartpacker/pkg/cmd"
	"bksmartpacker/pkg/log"
	"bksmartpacker/pkg/smartpkg"
	"bksmartpacker/pkg/types"
)

func main() {
	os.Exit(run())
}

func run() int {
	runtime.GOMAXPROCS(4)
	defer func() {
		if err := recover(); err != nil {
			log.Error("panic", "error", err)
		}
	}()
	opts, err := parseCLIOptions(os.Args[1:])
	if err != nil {
		log.Error("parse args failed", "error", err)
		return 1
	}
	if err := opts.validate(); err != nil {
		log.Error("invalid args", "error", err)
		return 1
	}
	if opts.Inspect != "" {
		if err := runInspect(opts.Inspect, opts.InspectFormat); err != nil {
			log.Error("inspect failed", "error", err)
			return 1
		}
		return 0
	}
	params, err := opts.toParams()
	if err != nil {
		log.Error("invalid args", "error", err)
		return 1
	}
	if err := execute(params); err != nil {
		log.Error("build failed", "error", err)
		return 1
	}
	return 0
}

func execute(paramData *types.Params) error {
	log.Info("build params", "params", paramData)

	log.Info("start BuildSMartPackage")
	if err := cmd.BuildSMartPackage(paramData); err != nil {
		return errors.Wrap(err, "failed to build smart package")
	}
	log.Info("BuildSMartPackage done")
	return nil
}

// runInspect 输出 s-mart 包的结构摘要。
func runInspect(path, format string) error {
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
