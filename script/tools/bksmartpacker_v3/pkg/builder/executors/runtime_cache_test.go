package executors

import "testing"

func TestParsePodmanImageSize(t *testing.T) {
	output := `[{"Type":"Images","RawSize":1024},{"Type":"Containers","RawSize":2048}]`
	if size, ok := parsePodmanImageSize(output); !ok || size != 1024 {
		t.Fatalf("size=%d ok=%v", size, ok)
	}
	if _, ok := parsePodmanImageSize("not-json"); ok {
		t.Fatal("invalid output should not be known")
	}
}

func TestClassifyRuntimeCache(t *testing.T) {
	cold := runtimeCacheSnapshot{Images: map[string]bool{}, ImagesKnown: true}
	warm := runtimeCacheSnapshot{Images: map[string]bool{"a": true, "b": true}, ImagesKnown: true}
	if state, _ := classifyRuntimeCache(cold, warm); state != "cold" {
		t.Fatalf("cold state = %s", state)
	}
	if state, reused := classifyRuntimeCache(warm, warm); state != "hit" || reused != 2 {
		t.Fatalf("hit state=%s reused=%d", state, reused)
	}
	if state, reused := classifyRuntimeCache(warm, runtimeCacheSnapshot{Images: map[string]bool{"a": true}, ImagesKnown: true}); state != "partial" || reused != 1 {
		t.Fatalf("partial state=%s reused=%d", state, reused)
	}
	if state, reused := classifyRuntimeCache(warm, runtimeCacheSnapshot{Images: map[string]bool{"c": true}, ImagesKnown: true}); state != "miss" || reused != 0 {
		t.Fatalf("miss state=%s reused=%d", state, reused)
	}
}

func TestClassifyRuntimeCacheUnknown(t *testing.T) {
	unknown := runtimeCacheSnapshot{Images: map[string]bool{}}
	warm := runtimeCacheSnapshot{Images: map[string]bool{"a": true}, ImagesKnown: true}
	if state, _ := classifyRuntimeCache(unknown, warm); state != "unknown" {
		t.Fatalf("unknown state = %s", state)
	}
}
