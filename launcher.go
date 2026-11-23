package main

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

// getFreeSocket returns a listener on the first free port of [startPort,
// startPort+100) on the loopback interface only: the desktop app's API has no
// authentication, so it must not be reachable from the network (SEC-01). A
// ":port" listener took every interface, and on Windows it could share a port
// another process already listened on at 127.0.0.1.
func getFreeSocket(startPort int) (int, net.Listener, error) {
	for port := startPort; port < startPort+100; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return port, ln, nil
		}
	}
	return 0, nil, fmt.Errorf("no free port found in range %d-%d", startPort, startPort+100)
}

func waitForServer(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/api/service/v1/healthz", port)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
