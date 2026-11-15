//go:build linux

package tunnel

import (
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestLinuxTUNCarriesTCP starts a real tun device and checks that a TCP
// connection opened by this process is delivered to the local SOCKS5 proxy
// with the original destination. Gated so ordinary `go test` stays offline
// and unprivileged.
func TestLinuxTUNCarriesTCP(t *testing.T) {
	if os.Getenv("PROXRAY_TUN_INTEGRATION") == "" {
		t.Skip("set PROXRAY_TUN_INTEGRATION=1 to exercise a real tun device (needs root)")
	}
	if os.Geteuid() != 0 {
		t.Fatal("TUN integration test requires root")
	}

	got := make(chan string, 1)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		dst, err := readSOCKS5Connect(c)
		if err != nil {
			t.Errorf("socks: %v", err)
			return
		}
		got <- dst
		_, _ = io.WriteString(c, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nOK")
	}()

	tun, err := Start(Options{
		SocksAddr: ln.Addr().String(),
		ServerIPs: []string{"9.9.9.9"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tun.Close()

	out, err := exec.Command("ip", "route", "show").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	routes := string(out)
	for _, want := range []string{"0.0.0.0/1 dev proxray0", "128.0.0.0/1 dev proxray0", "9.9.9.9"} {
		if !strings.Contains(routes, want) {
			t.Fatalf("routing table missing %q:\n%s", want, routes)
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://1.2.3.4/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		t.Fatalf("body = %q", body)
	}
	select {
	case dst := <-got:
		if dst != "1.2.3.4:80" {
			t.Fatalf("socks destination = %q, want 1.2.3.4:80", dst)
		}
	default:
		t.Fatal("socks server did not observe a connect")
	}
}

// readSOCKS5Connect accepts one unauthenticated CONNECT and returns host:port.
func readSOCKS5Connect(c net.Conn) (string, error) {
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	head := make([]byte, 2)
	if _, err := io.ReadFull(c, head); err != nil {
		return "", err
	}
	methods := make([]byte, int(head[1]))
	if _, err := io.ReadFull(c, methods); err != nil {
		return "", err
	}
	if _, err := c.Write([]byte{0x05, 0x00}); err != nil {
		return "", err
	}
	req := make([]byte, 4)
	if _, err := io.ReadFull(c, req); err != nil {
		return "", err
	}
	var host string
	switch req[3] {
	case 0x01:
		ip := make([]byte, 4)
		if _, err := io.ReadFull(c, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	case 0x03:
		var n [1]byte
		if _, err := io.ReadFull(c, n[:]); err != nil {
			return "", err
		}
		name := make([]byte, int(n[0]))
		if _, err := io.ReadFull(c, name); err != nil {
			return "", err
		}
		host = string(name)
	default:
		return "", io.ErrUnexpectedEOF
	}
	var portb [2]byte
	if _, err := io.ReadFull(c, portb[:]); err != nil {
		return "", err
	}
	port := binary.BigEndian.Uint16(portb[:])
	// Reply success, bound 0.0.0.0:0.
	if _, err := c.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return "", err
	}
	return net.JoinHostPort(host, itoa(int(port))), nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
