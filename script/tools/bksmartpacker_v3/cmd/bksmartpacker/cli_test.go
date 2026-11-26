package main

import "testing"

func TestParseCLIOptionsRepeatedAndDefaults(t *testing.T) {
	opts, err := parseCLIOptions([]string{
		"--src", ".",
		"--module", "default",
		"--module", "pipeline",
		"--allow-partial",
		"--env", "A=1,2",
		"--env", "B=3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.Modules) != 2 || opts.Modules[0] != "default" || opts.Modules[1] != "pipeline" {
		t.Fatalf("modules = %#v", opts.Modules)
	}
	if len(opts.Envs) != 2 || opts.Envs[0] != "A=1,2" {
		t.Fatalf("envs = %#v", opts.Envs)
	}
	if opts.PackagingVersion != "v2" {
		t.Fatalf("packaging version = %q", opts.PackagingVersion)
	}
	if opts.Dest != "./s-mart.tar.gz" {
		t.Fatalf("dest = %q", opts.Dest)
	}
}

func TestCLIOptionsValidate(t *testing.T) {
	cases := []struct {
		name string
		opts cliOptions
	}{
		{name: "src required"},
		{name: "inspect and src", opts: cliOptions{Inspect: "pkg.tgz", Src: "."}},
		{name: "module requires allow partial", opts: cliOptions{Src: ".", Modules: []string{"default"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.opts.validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	valid := cliOptions{Inspect: "pkg.tgz", InspectFormat: "json"}
	if err := valid.validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCLIOptionsToParams(t *testing.T) {
	opts := cliOptions{Src: ".", Dest: "./out.tar.gz", PackagingVersion: "v2", BaseImageID: "default"}
	params, err := opts.toParams()
	if err != nil {
		t.Fatal(err)
	}
	if params.Dest == "" || params.Dest == opts.Dest {
		t.Fatalf("dest should be absolute: %q", params.Dest)
	}
}

func TestParseCLIOptionsInvalidEnum(t *testing.T) {
	if _, err := parseCLIOptions([]string{"--src", ".", "--packaging-version", "v3"}); err == nil {
		t.Fatal("expected enum error")
	}
}
