// Package store persists subscriptions and their cached share links on disk.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aimuzov/proxray/internal/link"
	"github.com/aimuzov/proxray/internal/profile"
	"github.com/aimuzov/proxray/internal/rawconf"
)

// SubEntry is one stored subscription and its cached contents: share links for
// a link subscription, whole xray configs for a JSON one.
type SubEntry struct {
	Name           string            `json:"name"`
	URL            string            `json:"url"`
	UserAgent      string            `json:"userAgent,omitempty"`
	Title          string            `json:"title,omitempty"`
	SupportURL     string            `json:"supportUrl,omitempty"`
	UpdateInterval int               `json:"updateInterval,omitempty"`
	UserInfo       *profile.UserInfo `json:"userInfo,omitempty"`
	UpdatedAt      string            `json:"updatedAt,omitempty"`
	Links          []string          `json:"links,omitempty"`
	Configs        []json.RawMessage `json:"configs,omitempty"`
	Bypass         string            `json:"bypass,omitempty"`
	NoHWID         bool              `json:"noHwid,omitempty"`
}

// Nodes re-parses the cached entries into selectable nodes, skipping any that
// no longer parse.
func (e SubEntry) Nodes() []profile.Node {
	var out []profile.Node
	for _, raw := range e.Links {
		if s, err := link.Parse(raw); err == nil {
			out = append(out, profile.NodeFromServer(s))
		}
	}
	for _, raw := range e.Configs {
		cfgs, err := rawconf.Parse(raw)
		if err != nil {
			continue
		}
		out = append(out, profile.NodeFromConfig(cfgs[0]))
	}
	return out
}

type state struct {
	Active        string     `json:"active,omitempty"`
	HWID          string     `json:"hwid,omitempty"`
	Subscriptions []SubEntry `json:"subscriptions,omitempty"`
}

// Store is an on-disk collection of subscriptions.
type Store struct {
	dir   string
	state state
}

// DefaultDir returns the per-user config directory for proxray. If that
// directory does not yet exist but a legacy "happ-cli" one does, the legacy
// directory is migrated in place (the project was renamed from happ-cli).
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "proxray")
	migrateLegacyDir(base, dir)
	return dir, nil
}

// migrateLegacyDir renames the old happ-cli config directory to dir when dir
// does not exist yet. Any error is ignored: a fresh config is created instead.
func migrateLegacyDir(base, dir string) {
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		return
	}
	legacy := filepath.Join(base, "happ-cli")
	if fi, err := os.Stat(legacy); err == nil && fi.IsDir() {
		_ = os.Rename(legacy, dir)
	}
}

// Open loads the store rooted at dir, creating an empty one if none exists.
func Open(dir string) (*Store, error) {
	s := &Store{dir: dir}
	data, err := os.ReadFile(s.path())
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("store: read: %w", err)
	}
	if err := json.Unmarshal(data, &s.state); err != nil {
		return nil, fmt.Errorf("store: parse %s: %w", s.path(), err)
	}
	return s, nil
}

func (s *Store) path() string { return filepath.Join(s.dir, "state.json") }

func (s *Store) save() error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("store: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("store: marshal: %w", err)
	}
	tmp := s.path() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("store: write: %w", err)
	}
	return os.Rename(tmp, s.path())
}

// Subscriptions returns all stored subscriptions.
func (s *Store) Subscriptions() []SubEntry { return s.state.Subscriptions }

// Find returns the subscription with the given name.
func (s *Store) Find(name string) (SubEntry, bool) {
	for _, e := range s.state.Subscriptions {
		if e.Name == name {
			return e, true
		}
	}
	return SubEntry{}, false
}

// Upsert inserts or replaces a subscription by name and persists the store. The
// first subscription added becomes the active one.
func (s *Store) Upsert(entry SubEntry) error {
	for i, e := range s.state.Subscriptions {
		if e.Name == entry.Name {
			s.state.Subscriptions[i] = entry
			return s.save()
		}
	}
	s.state.Subscriptions = append(s.state.Subscriptions, entry)
	if s.state.Active == "" {
		s.state.Active = entry.Name
	}
	return s.save()
}

// Remove deletes a subscription by name. If it was active, the active pointer
// falls back to the first remaining subscription.
func (s *Store) Remove(name string) error {
	out := s.state.Subscriptions[:0]
	removed := false
	for _, e := range s.state.Subscriptions {
		if e.Name == name {
			removed = true
			continue
		}
		out = append(out, e)
	}
	if !removed {
		return fmt.Errorf("store: subscription %q not found", name)
	}
	s.state.Subscriptions = out
	if s.state.Active == name {
		s.state.Active = ""
		if len(s.state.Subscriptions) > 0 {
			s.state.Active = s.state.Subscriptions[0].Name
		}
	}
	return s.save()
}

// Active returns the name of the active subscription, or "".
func (s *Store) Active() string { return s.state.Active }

// HWID returns the stored device id sent to panels that limit devices, or "".
func (s *Store) HWID() string { return s.state.HWID }

// SetHWID stores the device id; "" clears it so the next fetch derives a new
// one from the machine.
func (s *Store) SetHWID(id string) error {
	s.state.HWID = id
	return s.save()
}

// SetActive marks a subscription as active.
func (s *Store) SetActive(name string) error {
	if _, ok := s.Find(name); !ok {
		return fmt.Errorf("store: subscription %q not found", name)
	}
	s.state.Active = name
	return s.save()
}
