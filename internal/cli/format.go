package cli

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aimuzov/proxray/internal/profile"
)

// formatBytes renders a byte count in human-readable units.
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// formatTraffic renders subscription traffic as "used / total", or just "used"
// when the panel did not report a total (Total == 0), or "-" when absent.
func formatTraffic(ui *profile.UserInfo) string {
	if ui == nil {
		return "-"
	}
	used := formatBytes(ui.Upload + ui.Download)
	if ui.Total > 0 {
		return used + " / " + formatBytes(ui.Total)
	}
	return used
}

// defaultName derives a subscription name from its title or URL host.
func defaultName(title, rawURL string) string {
	if s := slug(title); s != "" {
		return s
	}
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		return u.Hostname()
	}
	return "sub"
}

// slug lowercases and replaces spaces/odd characters with dashes.
func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// expiryString renders the expiry time, or "never" / "" when absent.
func expiryString(t time.Time) string {
	if t.IsZero() {
		return "∞"
	}
	return t.Format("2006-01-02")
}
