// Package xraylog renders embedded xray-core runtime logs through proxray's own
// logger (internal/log), so engine output matches the "<time> <LEVEL> message"
// format instead of xray's raw, microsecond-stamped lines.
package xraylog

import (
	"fmt"
	"strings"

	xlog "github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/common/serial"

	"github.com/aimuzov/proxray/internal/log"
)

// Handler implements xray-core's common/log.Handler. It routes each message to
// the matching internal/log level. Access logs (per-connection) are only shown
// when verbose is set; general Info/Debug likewise.
type Handler struct{ verbose bool }

// Install registers the handler as xray-core's global log handler. It MUST be
// called AFTER xray.Start(), because starting an instance registers xray's own
// handler; calling Install afterwards replaces it.
func Install(verbose bool) {
	xlog.RegisterHandler(Handler{verbose: verbose})
}

// Handle implements common/log.Handler.
func (h Handler) Handle(msg xlog.Message) {
	switch m := msg.(type) {
	case *xlog.AccessMessage:
		if !h.verbose {
			return
		}
		to := cleanHost(serial.ToString(m.To))
		if d := shortDetour(m.Detour); d != "" {
			log.Info("→ " + to + "  " + d)
		} else {
			log.Info("→ " + to)
		}
	case *xlog.GeneralMessage:
		content := fmt.Sprint(m.Content)
		switch m.Severity {
		case xlog.Severity_Error:
			log.Error(content)
		case xlog.Severity_Warning:
			log.Warn(content)
		case xlog.Severity_Info:
			if h.verbose {
				log.Info(content)
			}
		case xlog.Severity_Debug:
			if h.verbose {
				log.Debug(content)
			}
		default:
			log.Info(content)
		}
	default:
		log.Info(msg.String())
	}
}

// cleanHost strips xray's leading "//" that http inbounds prepend to the
// destination (e.g. "//apple.com:443" -> "apple.com:443").
func cleanHost(s string) string {
	return strings.TrimLeft(s, "/")
}

// shortDetour reduces xray's "inbound >> outbound" routing tag to just the
// outbound (e.g. "http-in >> proxy" -> "proxy"). A tag without ">>" is returned
// trimmed as-is.
func shortDetour(d string) string {
	if d == "" {
		return ""
	}
	if i := strings.LastIndex(d, ">>"); i >= 0 {
		return strings.TrimSpace(d[i+2:])
	}
	return strings.TrimSpace(d)
}
