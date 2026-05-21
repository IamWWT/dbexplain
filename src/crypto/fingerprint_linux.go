//go:build linux

package crypto

import (
	"bufio"
	"os"
	"strings"
)

func collectHWInfo() map[string]string {
	info := make(map[string]string)

	// /etc/machine-id (systemd systems)
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		info["machine_id"] = strings.TrimSpace(string(data))
	}

	// /etc/machine-id fallback (dbus)
	if info["machine_id"] == "" {
		if data, err := os.ReadFile("/var/lib/dbus/machine-id"); err == nil {
			info["machine_id"] = strings.TrimSpace(string(data))
		}
	}

	// /sys/class/dmi/id/product_uuid (motherboard UUID)
	if data, err := os.ReadFile("/sys/class/dmi/id/product_uuid"); err == nil {
		info["product_uuid"] = strings.TrimSpace(string(data))
	}

	// /sys/class/dmi/id/board_serial (motherboard serial)
	if data, err := os.ReadFile("/sys/class/dmi/id/board_serial"); err == nil {
		info["board_serial"] = strings.TrimSpace(string(data))
	}

	// /proc/cpuinfo -- extract model name and flags (first CPU only)
	if f, err := os.Open("/proc/cpuinfo"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "model name") && info["cpu_model"] == "" {
				info["cpu_model"] = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
			if strings.HasPrefix(line, "flags") && info["cpu_flags"] == "" {
				info["cpu_flags"] = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
			if strings.HasPrefix(line, "CPU implementer") && info["cpu_impl"] == "" {
				info["cpu_impl"] = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
			if strings.HasPrefix(line, "CPU architecture") && info["cpu_arch"] == "" {
				info["cpu_arch"] = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
			if strings.HasPrefix(line, "CPU variant") && info["cpu_variant"] == "" {
				info["cpu_variant"] = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
		}
	}

	// hostname
	if hostname, err := os.Hostname(); err == nil {
		info["hostname"] = hostname
	}

	return info
}
