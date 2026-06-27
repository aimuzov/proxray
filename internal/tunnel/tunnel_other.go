//go:build !darwin

package tunnel

import (
	"fmt"
	"runtime"
)

// Tunnel is a no-op on platforms where TUN mode is not yet implemented.
type Tunnel struct{}

// Start reports that TUN mode is unavailable on this platform.
func Start(opts Options) (*Tunnel, error) {
	return nil, fmt.Errorf("TUN mode is not implemented on %s yet; use --mode proxy", runtime.GOOS)
}

// Close is a no-op.
func (t *Tunnel) Close() error { return nil }

// InterfaceTo is unavailable where TUN mode is not implemented.
func InterfaceTo(dest string) (string, error) {
	return "", fmt.Errorf("interface lookup is not implemented on %s yet", runtime.GOOS)
}
