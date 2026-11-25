//go:build testhooks

package util_test

import (
	"csp2d/util"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// UT-UTIL-MEM-01: container memory limits from cgroup v2 and v1 files; "max"
// and v1's huge "unlimited" value mean no limit.
func TestCgroupMemoryLimit(t *testing.T) {
	cases := []struct {
		name, file, content string
		want                int64
	}{
		{"v2 limit", "memory.max", "1073741824\n", 1 << 30},
		{"v2 unlimited", "memory.max", "max\n", 0},
		{"v1 limit", "memory/memory.limit_in_bytes", "536870912\n", 512 << 20},
		{"v1 unlimited", "memory/memory.limit_in_bytes", "9223372036854771712\n", 0},
		{"garbage", "memory.max", "lots\n", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, c.file), c.content)
			if got := util.CgroupMemoryLimit(root); got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
	if got := util.CgroupMemoryLimit(t.TempDir()); got != 0 {
		t.Fatalf("no cgroup files: got %d", got)
	}
}

// UT-UTIL-MEM-02: the physical memory is known on the supported platforms
// (Linux, Windows, macOS) and plausible.
func TestPhysicalMemory(t *testing.T) {
	got := util.PhysicalMemory()
	if got < 256<<20 || got > 1<<50 {
		t.Fatalf("physical memory %d bytes is implausible", got)
	}
}
