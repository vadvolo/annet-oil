// Package check provides device availability checks: TCP port reachability
// and SSH login verification for inventory devices, addressable by hostname or IP.
package check

import (
	"context"
	"fmt"
	"net"
	"sort"
	"time"

	"golang.org/x/crypto/ssh"

	"annet-oil/internal/inventory"
)

// Error types reported in Result.Error.Type.
const (
	ErrUnreachable = "unreachable" // no probed port accepted a connection
	ErrAuth        = "auth_err"    // TCP ok but SSH authentication was rejected
	ErrTimeout     = "timeout"     // SSH dial/handshake exceeded its deadline
	ErrLogin       = "login_err"   // SSH transport/handshake failed (not auth/timeout)
	ErrConfig      = "config_err"  // missing credentials / bad input
	ErrCanceled    = "canceled"    // the run was canceled before the device finished probing
)

// Login states reported in Result.Login.
const (
	LoginOK      = "ok"
	LoginFailed  = "failed"
	LoginSkipped = "skipped" // not attempted (disabled, telnet-only, or no creds)
)

// telnetPort is probed for reachability but never used for SSH login.
const telnetPort = 23

// PortResult is the outcome of probing a single TCP port.
type PortResult struct {
	Port      int    `json:"port"`
	Open      bool   `json:"open"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

// CheckError describes why a check failed.
type CheckError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Result is a single device availability report.
//
// Example (all good):
//
//	{"hostname":"r1","ip":"10.0.0.1","timestamp":"...","duration_ms":42,
//	 "reachable":true,"login":"ok","ports":[{"port":22,"open":true,"latency_ms":8}]}
//
// Example (auth failed):
//
//	{"hostname":"r1","ip":"10.0.0.1","timestamp":"...","reachable":true,
//	 "login":"failed","ports":[{"port":22,"open":true}],
//	 "error":{"type":"auth_err","message":"ssh: unable to authenticate"}}
type Result struct {
	Hostname   string       `json:"hostname"`
	IP         string       `json:"ip"`
	Vendor     string       `json:"vendor,omitempty"`
	Timestamp  time.Time    `json:"timestamp"`
	DurationMs int64        `json:"duration_ms"`
	Reachable  bool         `json:"reachable"`
	Login      string       `json:"login"`
	Ports      []PortResult `json:"ports"`
	Error      *CheckError  `json:"error,omitempty"`
}

// OK reports whether the device is reachable and, when a login was attempted,
// authenticated successfully.
func (r *Result) OK() bool {
	return r.Reachable && r.Login != LoginFailed
}

// Options controls how a device is checked.
type Options struct {
	// Ports to probe. When empty, the device's configured port is used.
	// The device's own port is always included.
	Ports []int
	// DialTimeout bounds each TCP port probe. Default 3s.
	DialTimeout time.Duration
	// LoginTimeout bounds the SSH handshake. Default 5s.
	LoginTimeout time.Duration
	// CheckLogin enables the SSH login attempt. When false, Login is "skipped".
	CheckLogin bool
}

func (o Options) dialTimeout() time.Duration {
	if o.DialTimeout <= 0 {
		return 3 * time.Second
	}
	return o.DialTimeout
}

func (o Options) loginTimeout() time.Duration {
	if o.LoginTimeout <= 0 {
		return 8 * time.Second
	}
	return o.LoginTimeout
}

// Device runs the availability check against a single inventory device.
func Device(ctx context.Context, dev *inventory.Device, opts Options) *Result {
	start := time.Now()

	target := dev.IP
	if target == "" {
		target = dev.Hostname
	}

	res := &Result{
		Hostname:  dev.Hostname,
		IP:        dev.IP,
		Vendor:    dev.Vendor,
		Timestamp: start,
		Login:     LoginSkipped,
	}

	ports := resolvePorts(dev.GetPort(), opts.Ports)
	if target == "" {
		res.Error = &CheckError{Type: ErrConfig, Message: "device has no ip or hostname"}
		res.DurationMs = time.Since(start).Milliseconds()
		return res
	}

	// Probe every port; collect which ones are open.
	var openPorts []int
	for _, p := range ports {
		pr := probePort(ctx, target, p, opts.dialTimeout())
		res.Ports = append(res.Ports, pr)
		if pr.Open {
			res.Reachable = true
			openPorts = append(openPorts, p)
		}
	}

	if !res.Reachable {
		// If the run was canceled (e.g. Ctrl+C) while this device was still
		// being probed, the ports were not truly closed — report it as
		// canceled rather than a false "unreachable".
		if ctx.Err() != nil {
			res.Error = &CheckError{
				Type:    ErrCanceled,
				Message: fmt.Sprintf("check canceled before %s was fully probed", target),
			}
		} else {
			res.Error = &CheckError{
				Type:    ErrUnreachable,
				Message: fmt.Sprintf("no open ports among %v on %s", ports, target),
			}
		}
		res.DurationMs = time.Since(start).Milliseconds()
		return res
	}

	// Login attempt (optional).
	if opts.CheckLogin {
		res.Login = attemptLogin(ctx, target, dev, openPorts, opts.dialTimeout(), opts.loginTimeout(), res)
	}

	res.DurationMs = time.Since(start).Milliseconds()
	return res
}

// attemptLogin tries an SSH login on the best available open port and updates
// res.Error on failure. It returns the resulting Login state.
func attemptLogin(ctx context.Context, target string, dev *inventory.Device, openPorts []int, dialTimeout, handshakeTimeout time.Duration, res *Result) string {
	if dev.Credentials.Login == "" {
		return LoginSkipped
	}

	loginPort := pickSSHPort(dev.GetPort(), openPorts)
	if loginPort == 0 {
		// Only telnet-style ports are open; SSH login is not supported here.
		return LoginSkipped
	}

	if err := sshLogin(ctx, net.JoinHostPort(target, fmt.Sprint(loginPort)),
		dev.Credentials.Login, dev.Credentials.Password, dialTimeout, handshakeTimeout); err != nil {
		res.Error = &CheckError{Type: classifyLoginError(err), Message: err.Error()}
		return LoginFailed
	}
	return LoginOK
}

// resolvePorts returns the sorted, de-duplicated set of ports to probe,
// always including the device's own port.
func resolvePorts(devicePort int, extra []int) []int {
	set := map[int]struct{}{}
	if devicePort > 0 {
		set[devicePort] = struct{}{}
	}
	for _, p := range extra {
		if p > 0 {
			set[p] = struct{}{}
		}
	}
	out := make([]int, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}

// pickSSHPort chooses the port to attempt SSH login on: the device's own port
// if it is open and not telnet, otherwise the lowest open non-telnet port.
// Returns 0 when no suitable port is available.
func pickSSHPort(devicePort int, openPorts []int) int {
	openSet := map[int]struct{}{}
	for _, p := range openPorts {
		openSet[p] = struct{}{}
	}
	if _, ok := openSet[devicePort]; ok && devicePort != telnetPort {
		return devicePort
	}
	best := 0
	for _, p := range openPorts {
		if p == telnetPort {
			continue
		}
		if best == 0 || p < best {
			best = p
		}
	}
	return best
}

func probePort(ctx context.Context, host string, port int, timeout time.Duration) PortResult {
	pr := PortResult{Port: port}

	// Don't misreport a port as closed when the run was already canceled.
	if err := ctx.Err(); err != nil {
		pr.Error = "canceled: " + err.Error()
		return pr
	}

	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(dialCtx, "tcp", net.JoinHostPort(host, fmt.Sprint(port)))
	if err != nil {
		pr.Error = err.Error()
		return pr
	}
	pr.Open = true
	pr.LatencyMs = time.Since(start).Milliseconds()
	_ = conn.Close()
	return pr
}

// sshLogin performs an SSH login attempt. The TCP dial and the SSH handshake
// each get their own timeout budget so a slow dial does not starve the
// handshake (which previously surfaced as "use of closed network connection").
// Both password and keyboard-interactive auth are offered, since some devices
// (notably RouterOS/MikroTik) reject bare "password" but accept the same
// credentials via keyboard-interactive.
func sshLogin(ctx context.Context, addr, user, password string, dialTimeout, handshakeTimeout time.Duration) error {
	cfg := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
			ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				// Answer every prompt with the password.
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = password
				}
				return answers, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         handshakeTimeout,
	}

	d := net.Dialer{Timeout: dialTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	// Bound the handshake with its own deadline on the raw connection. A
	// deadline surfaces as a clean i/o timeout rather than closing the fd
	// mid-handshake.
	_ = conn.SetDeadline(time.Now().Add(handshakeTimeout))

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		return err
	}
	// Handshake done: clear the deadline before tearing the session down.
	_ = conn.SetDeadline(time.Time{})
	client := ssh.NewClient(sshConn, chans, reqs)
	return client.Close()
}
