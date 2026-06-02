//go:build windows

package crypto

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

func collectHWInfo() map[string]string {
	info := make(map[string]string)

	// HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid
	// This GUID is generated during Windows installation and persists
	// across reboots. It is the standard machine identifier on Windows.
	k, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Cryptography`,
		registry.QUERY_VALUE,
	)
	if err == nil {
		if guid, _, err := k.GetStringValue("MachineGuid"); err == nil {
			info["machine_guid"] = guid
		}
		k.Close()
	}

	// hostname
	if hostname, err := os.Hostname(); err == nil {
		info["hostname"] = hostname
	}

	return info
}
