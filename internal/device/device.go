// Package device identifies the machine proxray runs on for panels that
// enforce a device limit (Remnawave's "HWID Device Limit" and compatible
// implementations). Such panels count devices by the x-hwid header and answer
// 404 to clients that omit it.
package device

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
)

// Info describes the machine as reported to the panel.
type Info struct {
	HWID    string // stable device id; empty when the machine id is undetectable
	OS      string // "macOS", "Linux", ...
	Version string // OS version, e.g. "15.3"
	Model   string // hardware model, e.g. "MacBookPro18,3"
}

// hwidPattern is the format Remnawave accepts since panel v3.0.0; anything else
// is ignored by the panel as if no header had been sent at all.
var hwidPattern = regexp.MustCompile(`^[a-zA-Z0-9=-]{10,64}$`)

// Valid reports whether id is acceptable as an x-hwid value.
func Valid(id string) bool { return hwidPattern.MatchString(id) }

// Detect gathers what this platform can tell us about itself. Every lookup is
// best-effort: an unavailable one leaves its field empty rather than failing.
func Detect() Info {
	info := osInfo()
	if id := machineID(); id != "" {
		info.HWID = hwidFrom(id)
	}
	return info
}

// hwidFrom derives a panel-safe id from a platform machine id. The value is
// hashed so the raw platform UUID never leaves the machine, and truncated to 32
// characters to stay comfortably inside the panel's length limit.
func hwidFrom(machineID string) string {
	sum := sha256.Sum256([]byte("proxray:" + machineID))
	return hex.EncodeToString(sum[:])[:32]
}

// Random returns a fresh 32-character id, used when the machine id cannot be
// read and when the user explicitly asks for a new one.
func Random() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
