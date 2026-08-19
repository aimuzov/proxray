package cli

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aimuzov/proxray/internal/link"
	"github.com/aimuzov/proxray/internal/profile"
	"github.com/aimuzov/proxray/internal/store"
)

func TestNodeTagsCarryTagAndProtocol(t *testing.T) {
	list := []profile.Node{
		profile.NodeFromServer(&link.Server{Tag: "Netherlands #1", Protocol: "vless"}),
		profile.NodeFromServer(&link.Server{Tag: "Germany", Protocol: "trojan"}),
	}
	got := nodeTags(list)
	want := []cobra.Completion{
		cobra.CompletionWithDesc("Netherlands #1", "vless"),
		cobra.CompletionWithDesc("Germany", "trojan"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("nodeTags() = %#v, want %#v", got, want)
	}
}

func TestNodeTagsEmpty(t *testing.T) {
	if got := nodeTags(nil); len(got) != 0 {
		t.Errorf("nodeTags(nil) = %#v, want empty", got)
	}
}

func TestSubNamesMarksActiveAndUsesTitle(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	// The first subscription added becomes the active one.
	if err := st.Upsert(store.SubEntry{Name: "home", Title: "Home VPN"}); err != nil {
		t.Fatalf("upsert home: %v", err)
	}
	if err := st.Upsert(store.SubEntry{Name: "work"}); err != nil {
		t.Fatalf("upsert work: %v", err)
	}

	got := subNames(st)
	want := []cobra.Completion{
		cobra.CompletionWithDesc("home", "Home VPN (active)"),
		cobra.Completion("work"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("subNames() = %#v, want %#v", got, want)
	}
}
