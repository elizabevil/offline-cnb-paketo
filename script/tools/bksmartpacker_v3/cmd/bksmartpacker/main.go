// Package main 提供 bksmartpacker 命令行入口。
package main

import (
	"os"
	"runtime"

	"github.com/pkg/errors"

	"bksmartpacker/pkg/cli"
	"bksmartpacker/pkg/cmd"
	"bksmartpacker/pkg/log"
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
	opts, err := cli.Parse(os.Args[1:])
	if err != nil {
		log.Error("parse args failed", "error", err)
		return 1
	}
	if err := opts.Validate(); err != nil {
		log.Error("invalid args", "error", err)
		return 1
	}
	if opts.Inspect != "" {
		if err := cli.Inspect(opts.Inspect, opts.InspectFormat); err != nil {
			log.Error("inspect failed", "error", err)
			return 1
		}
		return 0
	}
	params, err := opts.Params()
	if err != nil {
		log.Error("invalid args", "error", err)
		return 1
	}
	log.Info("build params", "params", params)
	log.Info("start BuildSMartPackage")
	if err := cmd.BuildSMartPackage(params); err != nil {
		log.Error("build failed", "error", errors.Wrap(err, "failed to build smart package"))
		return 1
	}
	log.Info("BuildSMartPackage done")
	return 0
}
