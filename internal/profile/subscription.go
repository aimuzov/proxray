// Package profile handles HAPP-style subscriptions: fetching a subscription
// URL, decoding its base64 list of share links, and reading the metadata
// headers HAPP clients understand (subscription-userinfo, profile-title, ...).
package profile

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/aimuzov/proxray/internal/link"
)

// UserInfo is the parsed subscription-userinfo header: traffic counters and the
// expiration time of the subscription.
type UserInfo struct {
	Upload   int64
	Download int64
	Total    int64
	Expire   time.Time
}

// Remaining returns the remaining traffic in bytes (Total - used).
func (u UserInfo) Remaining() int64 { return u.Total - u.Upload - u.Download }

// ParseBody turns a subscription response body into a list of servers. The body
// may be a base64-encoded newline-separated list of share links, or the plain
// list itself. Lines that fail to parse are skipped.
func ParseBody(body []byte) ([]*link.Server, error) {
	text := strings.TrimSpace(string(body))

	// A base64-encoded list has no scheme markers; decode it first.
	if !strings.Contains(text, "://") {
		if decoded, err := decodeBase64(text); err == nil {
			text = string(decoded)
		}
	}

	var servers []*link.Server
	for _, line := range strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r'
	}) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		s, err := link.Parse(line)
		if err != nil {
			continue // skip unsupported / malformed entries
		}
		servers = append(servers, s)
	}
	return servers, nil
}

// ParseUserInfo parses a subscription-userinfo header value of the form
// "upload=..; download=..; total=..; expire=..".
func ParseUserInfo(header string) (UserInfo, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return UserInfo{}, false
	}

	var ui UserInfo
	found := false
	for _, part := range strings.Split(header, ";") {
		key, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		if err != nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "upload":
			ui.Upload, found = n, true
		case "download":
			ui.Download, found = n, true
		case "total":
			ui.Total, found = n, true
		case "expire":
			if n > 0 {
				ui.Expire = time.Unix(n, 0)
			}
			found = true
		}
	}
	return ui, found
}

// decodeBase64 tries the common base64 variants used by subscription servers.
func decodeBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, enc := range encodings {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		} else {
			lastErr = err
		}
	}
	return nil, lastErr
}
