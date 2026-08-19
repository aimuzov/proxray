package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aimuzov/proxray/internal/geo"
	"github.com/aimuzov/proxray/internal/profile"
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
// They are needed when we route by region ourselves, and when a JSON
// subscription config routes by geoip:/geosite: rules of its own. On failure it
// returns an actionable error (the connect command aborts rather than silently
// falling back).
func prepareGeo(bypass string, node profile.Node) error {
	reason := fmt.Sprintf("--bypass %s", bypass)
	switch {
	case bypass != "off":
	case node.Config != nil && node.Config.UsesGeoAssets():
		reason = "the subscription's routing rules"
	default:
		return nil
	}

	dir, err := geoDir()
	if err != nil {
		return err
	}
	if err := geo.EnsureAssets(dir, geoMaxAge); err != nil {
		return fmt.Errorf("%w\ngeo databases are required for %s; retry with a network connection or run with --bypass off", err, reason)
	}
	return os.Setenv("XRAY_LOCATION_ASSET", dir)
}

// geoUpdate force-refreshes the geo databases in dir.
func geoUpdate(dir string) error {
	return geo.Update(dir)
}
