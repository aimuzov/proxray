package store

import (
	"encoding/json"
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

	nodes := got.Nodes()
	if len(nodes) != 2 {
		t.Fatalf("Nodes() = %d, want 2", len(nodes))
	}
	if nodes[0].Protocols[0] != "vless" || nodes[1].Protocols[0] != "trojan" {
		t.Errorf("protocols = %q %q", nodes[0].Protocols, nodes[1].Protocols)
	}
}

func TestHWIDRoundTrips(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := st.HWID(); got != "" {
		t.Fatalf("fresh store HWID = %q, want empty", got)
	}
	if err := st.SetHWID("abcdef0123456789"); err != nil {
		t.Fatalf("SetHWID: %v", err)
	}
	if err := st.Upsert(SubEntry{Name: "main", URL: "u", NoHWID: true}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	st2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := st2.HWID(); got != "abcdef0123456789" {
		t.Errorf("reloaded HWID = %q", got)
	}
	if entry, _ := st2.Find("main"); !entry.NoHWID {
		t.Error("NoHWID did not survive the reload")
	}

	if err := st2.SetHWID(""); err != nil {
		t.Fatalf("SetHWID(\"\"): %v", err)
	}
	st3, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := st3.HWID(); got != "" {
		t.Errorf("HWID after clearing = %q, want empty", got)
	}
}

func TestSubEntryNodesFromConfigs(t *testing.T) {
	entry := SubEntry{
		Name: "json",
		URL:  "https://example.com/sub",
		Configs: []json.RawMessage{
			json.RawMessage(`{"remarks":"Poland","meta":{"serverDescription":"main"},
				"outbounds":[{"tag":"proxy","protocol":"vless",
				"settings":{"vnext":[{"address":"pl.example.com","port":443}]}}]}`),
			json.RawMessage(`{"broken":`),
		},
	}

	nodes := entry.Nodes()
	if len(nodes) != 1 {
		t.Fatalf("Nodes() = %d, want 1 (the malformed config is skipped)", len(nodes))
	}
	if nodes[0].Tag != "Poland" || nodes[0].Note != "main" {
		t.Errorf("node = %+v", nodes[0])
	}
	if nodes[0].Address != "pl.example.com:443" || nodes[0].Config == nil {
		t.Errorf("node = %+v", nodes[0])
	}
}

func TestSubEntryRoundTripsConfigs(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	raw := json.RawMessage(`{"remarks":"Poland","outbounds":[{"tag":"proxy","protocol":"vless",` +
		`"settings":{"vnext":[{"address":"pl.example.com","port":443}]}}]}`)
	if err := st.Upsert(SubEntry{Name: "json", URL: "u", Configs: []json.RawMessage{raw}}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	st2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, ok := st2.Find("json")
	if !ok {
		t.Fatal("subscription 'json' not found after reload")
	}
	nodes := got.Nodes()
	if len(nodes) != 1 || nodes[0].Tag != "Poland" {
		t.Fatalf("reloaded nodes = %+v", nodes)
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
