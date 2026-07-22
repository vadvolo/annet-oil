package check

import (
	"context"
	"net"
	"testing"
	"time"

	"annet-oil/internal/inventory"
)

// listenTCP starts a throwaway TCP listener and returns its port.
func listenTCP(t *testing.T) (int, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	return port, func() { ln.Close() }
}

func TestDevice_ReachableNoLogin(t *testing.T) {
	port, closeFn := listenTCP(t)
	defer closeFn()

	dev := &inventory.Device{Hostname: "test", IP: "127.0.0.1", Port: port}
	res := Device(context.Background(), dev, Options{CheckLogin: false})

	if !res.Reachable {
		t.Fatalf("expected reachable, got %+v", res)
	}
	if res.Login != LoginSkipped {
		t.Errorf("expected login skipped, got %q", res.Login)
	}
	if res.Error != nil {
		t.Errorf("expected no error, got %+v", res.Error)
	}
	if len(res.Ports) != 1 || !res.Ports[0].Open {
		t.Errorf("expected one open port, got %+v", res.Ports)
	}
}

func TestDevice_Unreachable(t *testing.T) {
	// Reserve a port then close it so nothing listens.
	port, closeFn := listenTCP(t)
	closeFn()
	time.Sleep(50 * time.Millisecond)

	dev := &inventory.Device{Hostname: "dead", IP: "127.0.0.1", Port: port}
	res := Device(context.Background(), dev, Options{CheckLogin: true, DialTimeout: 500 * time.Millisecond})

	if res.Reachable {
		t.Fatalf("expected unreachable, got %+v", res)
	}
	if res.Error == nil || res.Error.Type != ErrUnreachable {
		t.Errorf("expected unreachable error, got %+v", res.Error)
	}
	if res.Login != LoginSkipped {
		t.Errorf("expected login skipped when unreachable, got %q", res.Login)
	}
}

func TestDevice_MultiPort(t *testing.T) {
	openPort, closeFn := listenTCP(t)
	defer closeFn()
	closedPort, closeFn2 := listenTCP(t)
	closeFn2()

	dev := &inventory.Device{Hostname: "multi", IP: "127.0.0.1", Port: openPort}
	res := Device(context.Background(), dev, Options{
		Ports:       []int{closedPort},
		CheckLogin:  false,
		DialTimeout: 500 * time.Millisecond,
	})

	if !res.Reachable {
		t.Fatalf("expected reachable via open port, got %+v", res)
	}
	if len(res.Ports) != 2 {
		t.Fatalf("expected 2 probed ports, got %d", len(res.Ports))
	}
}

func TestDevice_LoginAuthError(t *testing.T) {
	// A plain TCP listener that closes immediately is not an SSH server, so
	// the handshake fails — exercising the login-failure path.
	port, closeFn := listenTCP(t)
	defer closeFn()

	dev := &inventory.Device{
		Hostname:    "auth",
		IP:          "127.0.0.1",
		Port:        port,
		Credentials: inventory.DeviceCredentials{Login: "admin", Password: "secret"},
	}
	res := Device(context.Background(), dev, Options{CheckLogin: true, LoginTimeout: time.Second})

	if !res.Reachable {
		t.Fatalf("expected reachable, got %+v", res)
	}
	if res.Login != LoginFailed {
		t.Errorf("expected login failed, got %q", res.Login)
	}
	if res.Error == nil {
		t.Errorf("expected an error for failed login")
	}
}

func TestDevices_Batch(t *testing.T) {
	port, closeFn := listenTCP(t)
	defer closeFn()

	devices := []inventory.Device{
		{Hostname: "a", IP: "127.0.0.1", Port: port},
		{Hostname: "b", IP: "127.0.0.1", Port: port},
		{Hostname: "z-dead", IP: "127.0.0.1", Port: 1},
	}
	report := Devices(context.Background(), devices, Options{DialTimeout: 500 * time.Millisecond}, 2)

	if report.Total != 3 {
		t.Errorf("expected total 3, got %d", report.Total)
	}
	if report.Reachable != 2 {
		t.Errorf("expected 2 reachable, got %d", report.Reachable)
	}
	if report.Unreachable != 1 {
		t.Errorf("expected 1 unreachable, got %d", report.Unreachable)
	}
	// Sorted by hostname.
	if report.Results[0].Hostname != "a" {
		t.Errorf("expected results sorted by hostname, got %q first", report.Results[0].Hostname)
	}
}

func TestPickSSHPort(t *testing.T) {
	if p := pickSSHPort(22, []int{22, 23}); p != 22 {
		t.Errorf("expected 22, got %d", p)
	}
	if p := pickSSHPort(23, []int{23, 10022}); p != 10022 {
		t.Errorf("expected 10022 (telnet skipped), got %d", p)
	}
	if p := pickSSHPort(23, []int{23}); p != 0 {
		t.Errorf("expected 0 when only telnet open, got %d", p)
	}
}
