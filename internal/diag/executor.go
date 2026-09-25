package diag

import (
	"context"
	"regexp"
	"strings"
)

// ExecOutcome is the result of running one command on a device. It mirrors the
// fields diag needs without coupling to any specific gnetcli client signature.
type ExecOutcome struct {
	Output    string
	Error     string
	ErrorCode string
	Status    int32
}

// DeviceExecutor runs a single read-only command against a target. The handler
// provides a gnetcli-backed adapter; tests use a fake. Decoupling here keeps the
// diag package independent of gnetcli's evolving ExecWithDevice signature
// (ssh/telnet selection, timeouts, etc. live in the adapter).
type DeviceExecutor interface {
	Exec(ctx context.Context, target Target, cmd string) (ExecOutcome, error)
}

// readOnlyGuard is a defense-in-depth allowlist. Every command in the registry is
// read-only by construction; this rejects anything that would ever mutate state,
// so a bad registry edit (or a placeholder that expands oddly) can never write.
var readOnlyGuard = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^show\s+.*$`),
	regexp.MustCompile(`(?i)^ping\s+\S+.*$`),
	regexp.MustCompile(`(?i)^traceroute\s+\S+.*$`),
	// RouterOS: paths ending in print/monitor/export, plus /ping and /tool traceroute.
	regexp.MustCompile(`(?i)^/?[a-z0-9 /\[\]~"=.-]+\s+print(\s+.*)?$`),
	regexp.MustCompile(`(?i)^/?[a-z0-9 /\[\]~"=.-]+\s+monitor(\s+.*)?$`),
	regexp.MustCompile(`(?i)^/?([a-z0-9 /-]+\s+)?export(\s+.*)?$`),
	regexp.MustCompile(`(?i)^/?ping\s+\S+.*$`),
	regexp.MustCompile(`(?i)^/?tool\s+traceroute\s+\S+.*$`),
	regexp.MustCompile(`(?i)^/?system\s+identity\s+print$`),
	regexp.MustCompile(`(?i)^/?tool\s+profile(\s+.*)?$`),
}

// isReadOnly reports whether cmd is a safe read-only command.
func isReadOnly(cmd string) bool {
	c := strings.TrimSpace(cmd)
	for _, re := range readOnlyGuard {
		if re.MatchString(c) {
			return true
		}
	}
	return false
}
