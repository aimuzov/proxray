package cli

import (
	"testing"

	"github.com/aimuzov/proxray/internal/profile"
)

func TestFormatTrafficWithTotal(t *testing.T) {
	ui := &profile.UserInfo{Upload: 1 << 30, Download: 1 << 30, Total: 10 << 30}
	got := formatTraffic(ui)
	want := "2.0 GB / 10.0 GB"
	if got != want {
		t.Errorf("formatTraffic = %q, want %q", got, want)
	}
}

func TestFormatTrafficNoTotal(t *testing.T) {
	// Panel did not report a total (total=0): show only what's used.
	ui := &profile.UserInfo{Upload: 1 << 30, Download: 1 << 30, Total: 0}
	got := formatTraffic(ui)
	want := "2.0 GB"
	if got != want {
		t.Errorf("formatTraffic = %q, want %q", got, want)
	}
}

func TestFormatTrafficNil(t *testing.T) {
	if got := formatTraffic(nil); got != "-" {
		t.Errorf("formatTraffic(nil) = %q, want -", got)
	}
}
