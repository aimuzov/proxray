package profile

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Subscription is the result of fetching and parsing a subscription URL,
// including the metadata HAPP clients display.
type Subscription struct {
	Title          string
	UpdateInterval int // hours; 0 if not provided
	SupportURL     string
	Announcement   string
	UserInfo       *UserInfo
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

// Fetch downloads a subscription URL and parses its body and metadata headers.
func Fetch(ctx context.Context, subURL, userAgent string) (*Subscription, error) {
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, subURL, nil)
	if err != nil {
		return nil, fmt.Errorf("subscription request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
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
