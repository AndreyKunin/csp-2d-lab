package main

// Tests of the desktop launcher (launcher.go). They live here, not under
// test/, because getFreeSocket is in package main, which no other package can
// import.

import (
	"net"
	"strconv"
	"testing"
	"time"
)

// lanAddresses returns the host's non-loopback IPv4 addresses.
func lanAddresses(t *testing.T) []net.IP {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	var ips []net.IP
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && !n.IP.IsLoopback() && !n.IP.IsLinkLocalUnicast() {
			ips = append(ips, n.IP)
		}
	}
	return ips
}

// UT-DSK-01: the desktop server listens on the loopback interface only
// (SEC-01): reachable at 127.0.0.1, not at the host's LAN addresses.
func TestGetFreeSocketListensOnLoopbackOnly(t *testing.T) {
	port, ln, err := getFreeSocket(18180)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || !addr.IP.IsLoopback() || addr.Port != port {
		t.Fatalf("listening on %s, want 127.0.0.1:%d", ln.Addr(), port)
	}
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 2*time.Second)
	if err != nil {
		t.Fatalf("not reachable at 127.0.0.1:%d: %v", port, err)
	}
	_ = c.Close()

	lan := lanAddresses(t)
	if len(lan) == 0 {
		t.Log("no LAN address on this host: the LAN check is skipped")
	}
	for _, ip := range lan {
		if c, err := net.DialTimeout("tcp", net.JoinHostPort(ip.String(), strconv.Itoa(port)), 2*time.Second); err == nil {
			_ = c.Close()
			t.Errorf("reachable from the LAN at %s:%d", ip, port)
		}
	}
}

// UT-DSK-02: a busy port is skipped; the next free one is used.
func TestGetFreeSocketSkipsABusyPort(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:18190")
	if err != nil {
		t.Skipf("port 18190 unavailable: %v", err)
	}
	defer busy.Close()
	port, ln, err := getFreeSocket(18190)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if port == 18190 {
		t.Fatalf("got the busy port %d", port)
	}
	if port < 18190 || port >= 18290 {
		t.Fatalf("port %d outside [18190, 18290)", port)
	}
}
