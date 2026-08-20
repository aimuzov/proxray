package profile

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aimuzov/proxray/internal/log"
)

// Subscription is the result of fetching and parsing a subscription URL,
// including the metadata HAPP clients display.
type Subscription struct {
	Title          string
	UpdateInterval int // hours; 0 if not provided
	SupportURL     string
	Announcement   string
	UserInfo       *UserInfo
	HWIDActive     bool // the panel counts devices against a limit
	Body           Body
}

// Nodes lists the subscription's entries, whichever format it arrived in.
func (s *Subscription) Nodes() []Node {
	out := make([]Node, 0, len(s.Body.Servers)+len(s.Body.Configs))
	for _, srv := range s.Body.Servers {
		out = append(out, NodeFromServer(srv))
	}
	for _, cfg := range s.Body.Configs {
		out = append(out, NodeFromConfig(cfg))
	}
	return out
}

// DefaultUserAgent is sent when fetching a subscription unless overridden. Many
// panels vary their response format by User-Agent; identifying as Happ yields
// the base64 share-link list this tool understands.
const DefaultUserAgent = "Happ/1.0"

// FetchOptions carries what the panel learns about the client. Everything but
// UserAgent belongs to the HWID device-limit protocol; empty fields are simply
// not sent. The device is described by plain strings rather than a device.Info
// so this package stays free of platform-specific code.
type FetchOptions struct {
	UserAgent   string
	HWID        string
	DeviceOS    string
	OSVersion   string
	DeviceModel string
}

// Fetch downloads a subscription URL and parses its body and metadata headers.
func Fetch(ctx context.Context, subURL string, opts FetchOptions) (*Subscription, error) {
	if opts.UserAgent == "" {
		opts.UserAgent = DefaultUserAgent
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, subURL, nil)
	if err != nil {
		return nil, fmt.Errorf("subscription request: %w", err)
	}
	req.Header.Set("User-Agent", opts.UserAgent)
	for _, h := range [...]struct{ name, value string }{
		{"x-hwid", opts.HWID},
		{"x-device-os", opts.DeviceOS},
		{"x-ver-os", opts.OSVersion},
		{"x-device-model", opts.DeviceModel},
	} {
		if h.value != "" {
			req.Header.Set(h.name, h.value)
		}
	}
	// Host only: the path carries the subscription token.
	log.Debug("subscription request", "host", req.URL.Host, "user-agent", opts.UserAgent,
		"x-hwid", orNone(opts.HWID), "x-device-os", orNone(opts.DeviceOS),
		"x-ver-os", orNone(opts.OSVersion), "x-device-model", orNone(opts.DeviceModel))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	log.Debug("subscription response", "status", resp.StatusCode, "hwid-headers", hwidVerdict(resp.Header))

	// The device-limit verdict comes before the status check: a panel that
	// rejects the device answers 404, which on its own says nothing useful.
	if err := hwidError(resp.Header); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusNotFound && opts.HWID == "" {
			return nil, fmt.Errorf("subscription returned HTTP 404; if the panel limits devices, " +
				"drop --no-hwid so proxray identifies this machine")
		}
		return nil, fmt.Errorf("subscription returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read subscription body: %w", err)
	}

	parsed, err := ParseBody(body)
	if err != nil {
		return nil, err
	}

	sub := &Subscription{
		Title:        decodeHeaderText(resp.Header.Get("profile-title")),
		SupportURL:   resp.Header.Get("support-url"),
		Announcement: decodeHeaderText(resp.Header.Get("announce")),
		HWIDActive:   resp.Header.Get("x-hwid-active") != "",
		Body:         parsed,
	}
	if iv, err := strconv.Atoi(resp.Header.Get("profile-update-interval")); err == nil {
		sub.UpdateInterval = iv
	}
	if ui, ok := ParseUserInfo(resp.Header.Get("subscription-userinfo")); ok {
		sub.UserInfo = &ui
	}
	return sub, nil
}

// hwidVerdict collects the panel's x-hwid-* headers for the debug log. Panels
// that only read the device id answer with none of them, and that silence is
// exactly what the log has to make visible: it means the id went nowhere the
// user can see.
func hwidVerdict(h http.Header) string {
	var found []string
	for name, values := range h {
		if strings.HasPrefix(strings.ToLower(name), "x-hwid") {
			found = append(found, strings.ToLower(name)+"="+strings.Join(values, ","))
		}
	}
	if len(found) == 0 {
		return "none (panel ignores the device id)"
	}
	slices.Sort(found)
	return strings.Join(found, " ")
}

func orNone(v string) string {
	if v == "" {
		return "(not sent)"
	}
	return v
}

// hwidError turns the panel's device-limit refusals into advice. The panel
// answers x-hwid-not-supported when it got no usable x-hwid at all, which for
// us also covers an id it rejected as malformed.
func hwidError(h http.Header) error {
	switch {
	case h.Get("x-hwid-max-devices-reached") != "":
		return fmt.Errorf("subscription device limit reached; free a slot in the panel, " +
			"or point proxray at a different device with 'proxray hwid set'")
	case h.Get("x-hwid-not-supported") != "":
		return fmt.Errorf("panel requires a device id; check 'proxray hwid' and re-run without --no-hwid")
	}
	return nil
}

// decodeHeaderText decodes a header value that may carry a "base64:" prefix or
// be raw base64, returning it as plain UTF-8 text.
func decodeHeaderText(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if rest, ok := strings.CutPrefix(v, "base64:"); ok {
		if dec, err := decodeBase64(rest); err == nil {
			return string(dec)
		}
		return rest
	}
	return v
}
