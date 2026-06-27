package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aimuzov/happ-cli/internal/geo"
)

// geoMaxAge is how long cached .dat databases stay valid before connect
// refreshes them.
const geoMaxAge = 24 * time.Hour

// normalizeBypass validates a bypass value and applies the default. An empty
// string means "use the default" (ru). Returns "ru" or "off".
func normalizeBypass(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "":
		return "ru", nil
	case "ru":
		return "ru", nil
	case "off":
		return "off", nil
	default:
		return "", fmt.Errorf("invalid bypass %q (use 'ru' or 'off')", v)
	}
}

// geoDir returns the directory holding the geoip/geosite databases.
func geoDir() (string, error) {
	dir, err := storeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "geo"), nil
}

// prepareGeo ensures the geo databases are present and points xray-core at them.
// It is a no-op when bypass is "off". On failure it returns an actionable error
// (the connect command aborts rather than silently falling back).
func prepareGeo(bypass string) error {
	if bypass == "off" {
		return nil
	}
	dir, err := geoDir()
	if err != nil {
		return err
	}
	if err := geo.EnsureAssets(dir, geoMaxAge); err != nil {
		return fmt.Errorf("%w\ngeo databases are required for --bypass %s; retry with a network connection or run with --bypass off", err, bypass)
	}
	return os.Setenv("XRAY_LOCATION_ASSET", dir)
}

// geoUpdate force-refreshes the geo databases in dir.
func geoUpdate(dir string) error {
	return geo.Update(dir)
}
