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
	"sync"
	"testing"
	"time"
)

func TestLinuxTUNCarriesTCP(t *testing.T) {
	if os.Getenv("PROXRAY_TUN_INTEGRATION") == "" {
		t.Skip("set PROXRAY_TUN_INTEGRATION=1 to exercise a real tun device (needs root)")
	}
	if os.Geteuid() != 0 {
		t.Fatal("TUN integration test requires root")
	}

	var mu sync.Mutex
	var dsts []string
	ln := socksSink(t, &mu, &dsts)

	tun, err := Start(Options{
		SocksAddr: ln.Addr().String(),
		ServerIPs: []string{"9.9.9.9", "9.9.9.9"},
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
	for _, want := range []string{"0.0.0.0/1 dev proxray0", "128.0.0.0/1 dev proxray0"} {
		if !strings.Contains(routes, want) {
			t.Fatalf("routing table missing %q:\n%s", want, routes)
		}
	}
	ip6, err := exec.Command("ip", "-6", "route", "show").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"blackhole ::/1", "blackhole 8000::/1"} {
		if !strings.Contains(string(ip6), want) {
			t.Fatalf("IPv6 table missing %q:\n%s", want, ip6)
		}
	}
	server, err := exec.Command("ip", "route", "show", "9.9.9.9").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(server), "proxray0") || !strings.Contains(string(server), "9.9.9.9") {
		t.Fatalf("server route = %q", server)
	}

	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}}
	resp, err := client.Get("http://1.2.3.4/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		t.Fatalf("body = %q", body)
	}
	time.Sleep(200 * time.Millisecond)
	conn, _ := net.DialTimeout("tcp", "9.9.9.9:853", 300*time.Millisecond)
	if conn != nil {
		conn.Close()
	}
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	seen := append([]string(nil), dsts...)
	mu.Unlock()
	if len(seen) != 1 || seen[0] != "1.2.3.4:80" {
		t.Fatalf("socks destinations = %#v", seen)
	}
}

func TestLinuxTUNReclaimsOwnRoutes(t *testing.T) {
	if os.Getenv("PROXRAY_TUN_INTEGRATION") == "" {
		t.Skip("set PROXRAY_TUN_INTEGRATION=1 to exercise a real tun device (needs root)")
	}
	ln := socksSink(t, nil, nil)
	tun, err := Start(Options{SocksAddr: ln.Addr().String(), ServerIPs: []string{"9.9.9.9"}})
	if err != nil {
		t.Fatal(err)
	}
	tun.teardown = []func(){tun.teardown[0]}
	tun.Close()
	tun, err = Start(Options{SocksAddr: ln.Addr().String(), ServerIPs: []string{"9.9.9.9"}})
	if err != nil {
		t.Fatal(err)
	}
	tun.Close()
	out, _ := exec.Command("ip", "route", "show", "0.0.0.0/1").CombinedOutput()
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("0.0.0.0/1 left behind: %s", out)
	}
}

func TestLinuxTUNLeavesForeignHalfRoute(t *testing.T) {
	if os.Getenv("PROXRAY_TUN_INTEGRATION") == "" {
		t.Skip("set PROXRAY_TUN_INTEGRATION=1 to exercise a real tun device (needs root)")
	}
	if err := run("ip", "route", "add", "blackhole", "0.0.0.0/1"); err != nil {
		t.Fatal(err)
	}
	defer run("ip", "route", "del", "blackhole", "0.0.0.0/1")
	ln := socksSink(t, nil, nil)
	if _, err := Start(Options{SocksAddr: ln.Addr().String(), ServerIPs: []string{"9.9.9.9"}}); err == nil {
		t.Fatal("Start replaced a foreign 0.0.0.0/1")
	}
	out, _ := exec.Command("ip", "route", "show", "0.0.0.0/1").CombinedOutput()
	if !strings.Contains(string(out), "blackhole") {
		t.Fatalf("foreign route = %q", out)
	}
}

func socksSink(t *testing.T, mu *sync.Mutex, dsts *[]string) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			dst, err := readSOCKS5Connect(c)
			if err == nil && mu != nil && dsts != nil {
				mu.Lock()
				*dsts = append(*dsts, dst)
				mu.Unlock()
			}
			_, _ = io.WriteString(c, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nOK")
			c.Close()
		}
	}()
	return ln
}

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
