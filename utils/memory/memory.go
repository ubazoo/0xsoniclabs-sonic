// Package memory provides methods to obtain system memory information.
package memory

import "golang.org/x/sys/unix"

// TotalMemory returns the total system memory reported by the OS in bytes.
func TotalMemory() uint64 {
	var info unix.Sysinfo_t
	unix.Sysinfo(&info)
	return info.Totalram
}

// FreeMemory returns the total free system memory in bytes as reported by the OS.
func FreeMemory() uint64 {
	var info unix.Sysinfo_t
	unix.Sysinfo(&info)
	return info.Freeram
}
