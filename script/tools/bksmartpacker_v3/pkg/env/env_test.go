package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidKey(t *testing.T) {
	for _, key := range []string{"A", "_A", "A1", "A_B"} {
		if !ValidKey(key) {
			t.Fatalf("%q should be valid", key)
		}
	}
	for _, key := range []string{"", "1A", "A-B", "A.B"} {
		if ValidKey(key) {
			t.Fatalf("%q should be invalid", key)
		}
	}
}

func TestWritePlatform(t *testing.T) {
	dir := t.TempDir()
	if err := WritePlatform(dir, map[string]string{"A": "1", "B": "two"}); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"A": "1", "B": "two"} {
		data, err := os.ReadFile(filepath.Join(dir, "env", key))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("%s=%q, want %q", key, data, want)
		}
	}
	if err := WritePlatform(dir, map[string]string{"A": "bad\nvalue"}); err == nil {
		t.Fatal("expected newline error")
	}
}
