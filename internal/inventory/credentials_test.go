package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

func loadInv(t *testing.T, body string) *Inventory {
	t.Helper()
	path := filepath.Join(t.TempDir(), "inv.yaml")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	inv, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return inv
}

func logins(cs []DeviceCredentials) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Login
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

const credYAML = `
credentials:
  default:
    login: "def"
    password: "defpass"
  infra-router:
    login: "infra"
    password: "infrapass"
  edge:
    login: "edge"
    password: "edgepass"
devices:
  - hostname: r-role
    ip: 10.0.0.1
    role: infra-router
  - hostname: r-norole
    ip: 10.0.0.2
  - hostname: r-explicit
    ip: 10.0.0.3
    role: infra-router
    credentials:
      login: "special"
      password: "specialpass"
  - hostname: r-badrole
    ip: 10.0.0.4
    role: does-not-exist
`

func TestCredentialsFor_RoleThenDefault(t *testing.T) {
	inv := loadInv(t, credYAML)

	dev, _ := GetDevice("r-role")
	if got := logins(inv.CredentialsFor(dev)); !eq(got, []string{"infra", "def"}) {
		t.Errorf("role device: got %v, want [infra def]", got)
	}
}

func TestCredentialsFor_NoRoleDefaultOnly(t *testing.T) {
	inv := loadInv(t, credYAML)

	dev, _ := GetDevice("r-norole")
	if got := logins(inv.CredentialsFor(dev)); !eq(got, []string{"def"}) {
		t.Errorf("no-role device: got %v, want [def]", got)
	}
}

func TestCredentialsFor_ExplicitFirst(t *testing.T) {
	inv := loadInv(t, credYAML)

	dev, _ := GetDevice("r-explicit")
	if got := logins(inv.CredentialsFor(dev)); !eq(got, []string{"special", "infra", "def"}) {
		t.Errorf("explicit device: got %v, want [special infra def]", got)
	}
}

func TestCredentialsFor_UnknownRoleFallsBackToDefault(t *testing.T) {
	inv := loadInv(t, credYAML)

	dev, _ := GetDevice("r-badrole")
	if got := logins(inv.CredentialsFor(dev)); !eq(got, []string{"def"}) {
		t.Errorf("unknown-role device: got %v, want [def]", got)
	}
}

func TestPrimaryCredentials(t *testing.T) {
	loadInv(t, credYAML)

	dev, _ := GetDevice("r-role")
	if got := PrimaryCredentials(dev).Login; got != "infra" {
		t.Errorf("primary for role device: got %q, want infra", got)
	}
}

// Legacy default_credentials must still work as the "default" group.
func TestLegacyDefaultCredentials(t *testing.T) {
	os.Setenv("DEVICE_USERNAME", "envuser")
	os.Setenv("DEVICE_PASSWORD", "envpass")
	inv := loadInv(t, `
default_credentials:
  login: "${DEVICE_USERNAME}"
  password: "${DEVICE_PASSWORD}"
devices:
  - hostname: legacy
    ip: 10.0.0.9
`)
	dev, _ := GetDevice("legacy")
	if got := logins(inv.CredentialsFor(dev)); !eq(got, []string{"envuser"}) {
		t.Errorf("legacy default: got %v, want [envuser]", got)
	}
}
