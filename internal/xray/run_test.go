package xray

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/aimuzov/proxray/internal/link"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// TestStartAndListen builds a config from a server and starts the embedded
// xray-core, verifying it accepts our generated JSON and binds the SOCKS port.
func TestStartAndListen(t *testing.T) {
	port := freePort(t)
	s := &link.Server{
		Protocol: "shadowsocks", Address: "127.0.0.1", Port: 9, // discard port; never dialed
		Method: "aes-256-gcm", Password: "testpass", Network: "tcp",
	}
	cfg, err := BuildConfig(s, Options{SocksPort: port})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}
	raw, err := cfg.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}

	inst, err := Start(raw)
	if err != nil {
		t.Fatalf("Start: %v\nconfig:\n%s", err, raw)
	}
	defer inst.Close()

	// Give the inbound a moment to bind, then confirm it's listening.
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	var conn net.Conn
	for i := 0; i < 20; i++ {
		conn, err = net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("SOCKS inbound not listening on %s: %v", addr, err)
	}
	conn.Close()
}
