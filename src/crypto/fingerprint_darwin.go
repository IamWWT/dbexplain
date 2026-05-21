//go:build darwin

package crypto

import (
	"os"

	"golang.org/x/sys/unix"
)

func collectHWInfo() map[string]string {
	info := make(map[string]string)

	// Sysctl keys stable across macOS 10.12+.
	sysctlKeys := map[string]string{
		"hw.uuid":      "hw_uuid",      // Hardware UUID (persistent across reboots)
		"hw.model":     "hw_model",     // e.g. "MacBookPro18,1"
		"hw.machine":   "hw_machine",   // e.g. "arm64" or "x86_64"
		"hw.memsize":   "hw_memsize",   // Total physical memory in bytes
		"hw.ncpu":      "hw_ncpu",      // Logical CPU count
		"hw.cpufamily": "hw_cpufamily", // CPU family identifier
	}
	for sysctlName, key := range sysctlKeys {
		if val, err := unix.Sysctl(sysctlName); err == nil && val != "" {
			info[key] = val
		}
	}

	// hostname
	if hostname, err := os.Hostname(); err == nil {
		info["hostname"] = hostname
	}

	return info
}
