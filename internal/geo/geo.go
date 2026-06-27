// Package geo downloads and caches the geoip.dat / geosite.dat databases that
// xray-core needs to resolve geoip:/geosite: routing rules.
package geo

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ReleaseBaseURL is the directory the .dat assets are fetched from. It is a var
// so tests can point it at a local server.
var ReleaseBaseURL = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download"

// assets are the database files required for geoip:/geosite: rules.
var assets = []string{"geoip.dat", "geosite.dat"}

// EnsureAssets makes sure every asset exists in dir and is newer than maxAge,
// downloading any that are missing or stale. Existing fresh files are left
// untouched.
func EnsureAssets(dir string, maxAge time.Duration) error {
	for _, name := range assets {
		path := filepath.Join(dir, name)
		if fresh(path, maxAge) {
			continue
		}
		if err := download(name, path); err != nil {
			return err
		}
	}
	return nil
}

// Update re-downloads every asset regardless of age.
func Update(dir string) error {
	for _, name := range assets {
		if err := download(name, filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

func fresh(path string, maxAge time.Duration) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < maxAge
}

// download fetches one asset and writes it atomically (temp file + rename).
func download(name, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return fmt.Errorf("geo: mkdir: %w", err)
	}
	url := ReleaseBaseURL + "/" + name
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("geo: download %s: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("geo: download %s: unexpected status %s", name, resp.Status)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), name+".*.tmp")
	if err != nil {
		return fmt.Errorf("geo: temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("geo: write %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("geo: close %s: %w", name, err)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("geo: rename %s: %w", name, err)
	}
	return nil
}
