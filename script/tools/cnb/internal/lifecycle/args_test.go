package lifecycle

import (
	"slices"
	"testing"
)

func baseConfig() Config {
	return Config{
		Paths: Paths{Layers: "/layers", App: "/app", Platform: "/platform"},
		Build: BuildInput{Order: "/cnb/order.toml"},
		Bins:  Bins{Detector: "detector", Builder: "builder", Analyzer: "analyzer", Exporter: "exporter"},
		Export: ExportTarget{
			OutputImage: "example/app:1",
			Mode:        ExportLayout,
		},
	}
}

func TestDetectArgsUsesGeneratedOrder(t *testing.T) {
	c := baseConfig()
	c.Build.Required = "paketo-buildpacks/go"
	got := c.detectArgs()
	want := []string{"-layers=/layers", "-app=/app", "-platform=/platform", "-order=/layers/order.toml"}
	if !slices.Equal(got, want) {
		t.Errorf("detectArgs = %v, want %v", got, want)
	}
}

func TestAnalyzeArgsLayoutMode(t *testing.T) {
	c := baseConfig()
	got := c.analyzeArgs()
	want := []string{"-layers=/layers", "-layout", "-layout-dir=/layers/oci", "example/app:1"}
	if !slices.Equal(got, want) {
		t.Errorf("analyzeArgs = %v, want %v", got, want)
	}
}

func TestAnalyzeArgsDaemonMode(t *testing.T) {
	c := baseConfig()
	c.Export.Mode = ExportDaemon
	c.Export.RunImage = "example/run:1"
	got := c.analyzeArgs()
	want := []string{"-layers=/layers", "-run-image=example/run:1", "-daemon", "example/app:1"}
	if !slices.Equal(got, want) {
		t.Errorf("analyzeArgs = %v, want %v", got, want)
	}
}

func TestExportArgsPerMode(t *testing.T) {
	cases := []struct {
		mode ExportMode
		want []string
	}{
		{ExportLayout, []string{"-layers=/layers", "-app=/app", "-layout", "-layout-dir=/layers/oci", "example/app:1"}},
		{ExportDaemon, []string{"-layers=/layers", "-app=/app", "-daemon", "-run-image=example/run:1", "example/app:1"}},
	}
	for _, c := range cases {
		cfg := baseConfig()
		cfg.Export.Mode = c.mode
		cfg.Export.RunImage = "example/run:1"
		if got := cfg.exportArgs(); !slices.Equal(got, c.want) {
			t.Errorf("mode %s: exportArgs = %v, want %v", c.mode, got, c.want)
		}
	}
}

func TestExportNoneAddsNoMode(t *testing.T) {
	c := baseConfig()
	c.Export.Mode = ExportNone
	got := c.exportArgs()
	want := []string{"-layers=/layers", "-app=/app", "example/app:1"}
	if !slices.Equal(got, want) {
		t.Errorf("exportArgs = %v, want %v", got, want)
	}
}
