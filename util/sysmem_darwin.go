//go:build darwin

package util

import "golang.org/x/sys/unix"

// physicalMemory reads the hw.memsize sysctl.
func physicalMemory() int64 {
	n, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0
	}
	return int64(n)
}
