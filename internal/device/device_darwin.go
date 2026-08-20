//go:build darwin

package device

import (
	"os/exec"
	"strings"
)

// machineID returns the IOPlatformUUID, the id macOS keeps stable across
// reinstalls of everything above the firmware.
func machineID() string {
	out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "IOPlatformUUID") {
			continue
		}
		// The line looks like `  "IOPlatformUUID" = "1234-..."`.
		_, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"`)
	}
	return ""
}

func osInfo() Info {
	return Info{
		OS:      "macOS",
		Version: run("sw_vers", "-productVersion"),
		Model:   run("sysctl", "-n", "hw.model"),
	}
}

// run returns the trimmed stdout of a command, or "" if it fails: every field
// it fills is optional.
func run(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
