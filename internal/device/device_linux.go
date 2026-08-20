//go:build linux

package device

import (
	"os"
	"strings"
)

// machineID reads the systemd machine id, falling back to the D-Bus one that
// predates it.
func machineID() string {
	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if id := strings.TrimSpace(readFile(path)); id != "" {
			return id
		}
	}
	return ""
}

func osInfo() Info {
	info := Info{OS: "Linux", Version: osRelease("VERSION_ID")}
	if name := osRelease("NAME"); name != "" {
		info.OS = name
	}
	info.Model = strings.TrimSpace(readFile("/sys/devices/virtual/dmi/id/product_name"))
	if info.Model == "" {
		info.Model, _ = os.Hostname()
	}
	return info
}

// osRelease pulls one key out of /etc/os-release, whose values may be quoted.
func osRelease(key string) string {
	for _, line := range strings.Split(readFile("/etc/os-release"), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if ok && k == key {
			return strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return ""
}

// readFile returns a file's contents, or "" if it cannot be read: every caller
// treats the value as optional.
func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
