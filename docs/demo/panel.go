//go:build ignore

// Command panel is the fake HAPP panel the README recording runs against.
//
//	go run docs/demo/panel.go   # serves http://127.0.0.1:8099/sub/demo
//
// It answers like a real subscription endpoint — a base64 list of share links plus
// the metadata headers internal/profile reads — so every command in the recording
// is the real one, against invented servers instead of someone's actual panel.
//
// The build tag keeps it out of `go build ./...` and `go vet ./...`.
package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const addr = "127.0.0.1:8099"

const (
	mb = 1 << 20
	gb = 1 << 30
)

func main() {
	links, err := loadLinks()
	if err != nil {
		log.Fatal(err)
	}
	body := base64.StdEncoding.EncodeToString([]byte(strings.Join(links, "\n")))

	http.HandleFunc("/sub/", func(w http.ResponseWriter, r *http.Request) {
		// An expiry three months out rather than a fixed date: a hardcoded one would
		// read as an expired subscription the next time the demo is recorded.
		expire := time.Now().AddDate(0, 3, 0).Unix()
		w.Header().Set("profile-title", "Demo VPN")
		w.Header().Set("profile-update-interval", "12")
		w.Header().Set("support-url", "https://example.com/support")
		w.Header().Set("subscription-userinfo",
			fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d",
				2*gb, 10*gb+400*mb, 200*gb, expire))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, body)
	})

	log.Printf("demo panel on http://%s/sub/demo (%d servers)", addr, len(links))
	log.Fatal(http.ListenAndServe(addr, nil))
}

// loadLinks reads subscription.txt next to this file, dropping comments and blanks.
// The path is derived from the source location, so the panel can be started from
// the repo root or from docs/ while vhs records.
func loadLinks() ([]string, error) {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("cannot locate panel.go")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(self), "subscription.txt"))
	if err != nil {
		return nil, fmt.Errorf("read fixture: %w", err)
	}

	var links []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		links = append(links, line)
	}
	if len(links) == 0 {
		return nil, fmt.Errorf("fixture holds no links")
	}
	return links, nil
}
