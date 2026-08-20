//go:build !darwin && !linux

package device

import (
	"os"
	"runtime"
)

// machineID has no portable source here, so the caller falls back to a stored
// random id.
func machineID() string { return "" }

func osInfo() Info {
	host, _ := os.Hostname()
	return Info{OS: runtime.GOOS, Model: host}
}
