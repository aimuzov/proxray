package store

import (
	"testing"
)

func TestUpsertPersistsAndReloads(t *testing.T) {
	dir := t.TempDir()

	st, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(st.Subscriptions()) != 0 {
		t.Fatalf("fresh store should be empty, got %d", len(st.Subscriptions()))
	}

	entry := SubEntry{
		Name:  "main",
		URL:   "https://sub.example.com/abc",
		Title: "My VPN",
		Links: []string{
			"vless://uuid-1@a.example.com:443?type=tcp&security=reality&pbk=k#Node A",
			"trojan://" + "pw" + "@b.example.com:443#Node B",
		},
	}
	if err := st.Upsert(entry); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	// First subscription becomes active automatically.
	if st.Active() != "main" {
		t.Errorf("Active = %q, want main", st.Active())
	}

	// Reopen from disk and verify persistence.
	st2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, ok := st2.Find("main")
	if !ok {
		t.Fatal("subscription 'main' not found after reload")
	}
	if got.Title != "My VPN" || len(got.Links) != 2 {
		t.Errorf("reloaded entry = %+v", got)
	}

	servers := got.Servers()
	if len(servers) != 2 {
		t.Fatalf("Servers() = %d, want 2", len(servers))
	}
	if servers[0].Protocol != "vless" || servers[1].Protocol != "trojan" {
		t.Errorf("protocols = %q %q", servers[0].Protocol, servers[1].Protocol)
	}
}

func TestUpsertUpdatesExisting(t *testing.T) {
	dir := t.TempDir()
	st, _ := Open(dir)
	_ = st.Upsert(SubEntry{Name: "x", URL: "u1"})
	_ = st.Upsert(SubEntry{Name: "x", URL: "u2"})
	if n := len(st.Subscriptions()); n != 1 {
		t.Fatalf("expected 1 subscription after update, got %d", n)
	}
	got, _ := st.Find("x")
	if got.URL != "u2" {
		t.Errorf("URL = %q, want u2", got.URL)
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	st, _ := Open(dir)
	_ = st.Upsert(SubEntry{Name: "a", URL: "u"})
	_ = st.Upsert(SubEntry{Name: "b", URL: "u"})
	if err := st.Remove("a"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := st.Find("a"); ok {
		t.Error("'a' still present after Remove")
	}
	// Active should fall back to a remaining subscription.
	if st.Active() != "b" {
		t.Errorf("Active = %q, want b after removing active", st.Active())
	}
}

func TestSubEntryBypassRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.Upsert(SubEntry{Name: "work", URL: "https://x", Bypass: "off"}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	sub, ok := reopened.Find("work")
	if !ok {
		t.Fatal("subscription not found after reopen")
	}
	if sub.Bypass != "off" {
		t.Errorf("Bypass = %q, want off", sub.Bypass)
	}
}
