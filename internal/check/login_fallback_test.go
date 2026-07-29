package check

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"annet-oil/internal/inventory"
)

// startSSHServer starts a minimal SSH server that accepts only the given
// user/password. It returns the listening port and a stop function.
func startSSHServer(t *testing.T, user, pass string) (int, func()) {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	signer, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if c.User() == user && string(password) == pass {
				return &ssh.Permissions{}, nil
			}
			return nil, errPermissionDenied
		},
	}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				sc, chans, reqs, err := ssh.NewServerConn(c, cfg)
				if err != nil {
					c.Close()
					return
				}
				go ssh.DiscardRequests(reqs)
				go func() {
					for ch := range chans {
						ch.Reject(ssh.Prohibited, "no sessions")
					}
				}()
				_ = sc
			}(conn)
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	return port, func() { ln.Close() }
}

// errPermissionDenied mimics a real SSH auth rejection so the client reports an
// auth error and advances to the next credential.
var errPermissionDenied = &sshAuthError{}

type sshAuthError struct{}

func (e *sshAuthError) Error() string { return "ssh: permission denied" }

func TestLoginFallback_RoleFailsDefaultSucceeds(t *testing.T) {
	// Server accepts only the default credential.
	port, stop := startSSHServer(t, "def", "defpass")
	defer stop()

	// Load an inventory where the device's role creds are wrong and default is right.
	path := filepath.Join(t.TempDir(), "inv.yaml")
	os.WriteFile(path, []byte(`
credentials:
  default:
    login: "def"
    password: "defpass"
  infra-router:
    login: "infra"
    password: "wrongpass"
devices:
  - hostname: r1
    ip: 127.0.0.1
    role: infra-router
`), 0644)
	if _, err := inventory.Load(path); err != nil {
		t.Fatal(err)
	}
	defer inventory.Load(writeEmptyInventory(t)) // reset global to avoid leaking to other tests

	dev, err := inventory.GetDevice("r1")
	if err != nil {
		t.Fatal(err)
	}
	dev.Port = port // point at the test server

	res := Device(context.Background(), dev, Options{
		CheckLogin:   true,
		DialTimeout:  2 * time.Second,
		LoginTimeout: 2 * time.Second,
	})

	if res.Login != LoginOK {
		t.Fatalf("expected login ok via default fallback, got %q (err %+v)", res.Login, res.Error)
	}
	if res.LoginUser != "def" {
		t.Errorf("expected login user 'def' (default group), got %q", res.LoginUser)
	}
}

func TestLoginFallback_RoleSucceedsFirst(t *testing.T) {
	// Server accepts the role credential.
	port, stop := startSSHServer(t, "infra", "infrapass")
	defer stop()

	path := filepath.Join(t.TempDir(), "inv.yaml")
	os.WriteFile(path, []byte(`
credentials:
  default:
    login: "def"
    password: "defpass"
  infra-router:
    login: "infra"
    password: "infrapass"
devices:
  - hostname: r1
    ip: 127.0.0.1
    role: infra-router
`), 0644)
	if _, err := inventory.Load(path); err != nil {
		t.Fatal(err)
	}
	defer inventory.Load(writeEmptyInventory(t))

	dev, _ := inventory.GetDevice("r1")
	dev.Port = port

	res := Device(context.Background(), dev, Options{CheckLogin: true, DialTimeout: 2 * time.Second, LoginTimeout: 2 * time.Second})
	if res.Login != LoginOK || res.LoginUser != "infra" {
		t.Fatalf("expected login ok via role creds, got login=%q user=%q err=%+v", res.Login, res.LoginUser, res.Error)
	}
}

func writeEmptyInventory(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "empty.yaml")
	os.WriteFile(path, []byte("devices: []\n"), 0644)
	return path
}
