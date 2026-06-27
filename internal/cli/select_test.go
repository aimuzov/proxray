package cli

import (
	"testing"

	"github.com/aimuzov/proxray/internal/link"
)

func servers() []*link.Server {
	return []*link.Server{
		{Tag: "Netherlands #1", Protocol: "vless"},
		{Tag: "Germany Frankfurt", Protocol: "trojan"},
		{Tag: "USA West", Protocol: "vmess"},
	}
}

func TestSelectServerDefaultsToFirst(t *testing.T) {
	s, idx, err := selectServer(servers(), "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 0 || s.Tag != "Netherlands #1" {
		t.Errorf("got idx=%d tag=%q", idx, s.Tag)
	}
}

func TestSelectServerByIndex(t *testing.T) {
	s, idx, err := selectServer(servers(), "2")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1 || s.Tag != "Germany Frankfurt" {
		t.Errorf("got idx=%d tag=%q", idx, s.Tag)
	}
}

func TestSelectServerByTagSubstringCaseInsensitive(t *testing.T) {
	s, _, err := selectServer(servers(), "frankfurt")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Tag != "Germany Frankfurt" {
		t.Errorf("got tag=%q", s.Tag)
	}
}

func TestSelectServerIndexOutOfRange(t *testing.T) {
	if _, _, err := selectServer(servers(), "9"); err == nil {
		t.Error("expected error for out-of-range index")
	}
}

func TestSelectServerNoMatch(t *testing.T) {
	if _, _, err := selectServer(servers(), "antarctica"); err == nil {
		t.Error("expected error for no tag match")
	}
}

func TestSelectServerEmptyList(t *testing.T) {
	if _, _, err := selectServer(nil, ""); err == nil {
		t.Error("expected error for empty server list")
	}
}
