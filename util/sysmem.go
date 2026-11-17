package util

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// unlimitedCgroup: cgroup v1 reports "no limit" as a huge page-aligned number
// (9223372036854771712); anything at or above 2^60 bytes is treated as none.
const unlimitedCgroup = int64(1) << 60

// CgroupMemoryLimit returns the memory limit, in bytes, of the cgroup the
// process runs in (a container limit), or 0 if there is none or it cannot be
// read. root is the cgroup filesystem mount point, normally "/sys/fs/cgroup".
//
// cgroup v2: <root>/memory.max ("max" means unlimited). With a cgroup namespace
// (the default for Docker and Kubernetes on cgroup v2) this is the container's
// own cgroup. cgroup v1: <root>/memory/memory.limit_in_bytes.
func CgroupMemoryLimit(root string) int64 {
	for _, file := range []string{
		filepath.Join(root, "memory.max"),
		filepath.Join(root, "memory", "memory.limit_in_bytes"),
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		value := strings.TrimSpace(string(data))
		if value == "max" {
			return 0
		}
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n <= 0 || n >= unlimitedCgroup {
			return 0
		}
		return n
	}
	return 0
}

// PhysicalMemory returns the total physical memory in bytes, or 0 if unknown.
func PhysicalMemory() int64 { return physicalMemory() }
