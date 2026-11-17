//go:build !linux && !windows && !darwin

package util

// physicalMemory is unknown on this platform.
func physicalMemory() int64 { return 0 }
