package geo

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fakeReleases(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/geoip.dat", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("IPDATA")) })
	mux.HandleFunc("/geosite.dat", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("SITEDATA")) })
	return httptest.NewServer(mux)
}

func withBaseURL(t *testing.T, url string) {
	t.Helper()
	old := ReleaseBaseURL
	ReleaseBaseURL = url
	t.Cleanup(func() { ReleaseBaseURL = old })
}

func TestEnsureAssetsDownloadsWhenMissing(t *testing.T) {
	ts := fakeReleases(t)
	defer ts.Close()
	withBaseURL(t, ts.URL)
	dir := t.TempDir()

	if err := EnsureAssets(dir, time.Hour); err != nil {
		t.Fatalf("EnsureAssets: %v", err)
	}
	for name, want := range map[string]string{"geoip.dat": "IPDATA", "geosite.dat": "SITEDATA"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestEnsureAssetsSkipsFresh(t *testing.T) {
	ts := fakeReleases(t)
	defer ts.Close()
	withBaseURL(t, ts.URL)
	dir := t.TempDir()

	for _, name := range []string{"geoip.dat", "geosite.dat"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("OLD"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := EnsureAssets(dir, time.Hour); err != nil {
		t.Fatalf("EnsureAssets: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "geoip.dat"))
	if string(got) != "OLD" {
		t.Errorf("fresh file was overwritten: %q", got)
	}
}

func TestEnsureAssetsRefreshesStale(t *testing.T) {
	ts := fakeReleases(t)
	defer ts.Close()
	withBaseURL(t, ts.URL)
	dir := t.TempDir()

	p := filepath.Join(dir, "geoip.dat")
	if err := os.WriteFile(p, []byte("OLD"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(p, stale, stale); err != nil {
		t.Fatal(err)
	}
	if err := EnsureAssets(dir, 24*time.Hour); err != nil {
		t.Fatalf("EnsureAssets: %v", err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "IPDATA" {
		t.Errorf("stale file not refreshed: %q", got)
	}
}

func TestUpdateForcesDownload(t *testing.T) {
	ts := fakeReleases(t)
	defer ts.Close()
	withBaseURL(t, ts.URL)
	dir := t.TempDir()

	p := filepath.Join(dir, "geosite.dat")
	if err := os.WriteFile(p, []byte("OLD"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Update(dir); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "SITEDATA" {
		t.Errorf("Update did not overwrite: %q", got)
	}
}
