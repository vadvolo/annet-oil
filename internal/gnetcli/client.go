package gnetcli

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	proto "github.com/annetutil/gnetcli/pkg/server/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"annet-oil/internal/audit"
	"annet-oil/internal/config"
	"annet-oil/internal/logging"
)

// Machine-readable error codes surfaced to API clients alongside the free-text
// message, so callers do not have to regex the error string.
const (
	ErrCodeReadTimeout   = "READ_TIMEOUT"
	ErrCodeCmdTimeout    = "CMD_TIMEOUT"
	ErrCodeEOF           = "EOF"
	ErrCodeUnknownDevice = "UNKNOWN_DEVICE"
	ErrCodeUnknown       = "UNKNOWN"
)

type Client struct {
	conn      *grpc.ClientConn
	client    proto.GnetcliClient
	authToken string
	login     string
	pass      string
	recorder  audit.Recorder

	// Per-command timeout defaults (seconds); 0 means inherit the gnetcli
	// server default. maxTimeoutSec caps a per-request override (0 = no cap).
	readTimeoutSec float64
	cmdTimeoutSec  float64
	maxTimeoutSec  float64
}

type ExecResult struct {
	Output string
	Error  string
	// ErrorCode is a stable, machine-readable classification of Error (one of
	// the ErrCode* constants). Empty when the command succeeded.
	ErrorCode string
	Status    int32
}

// New builds a gnetcli client. recorder may be nil to disable auditing; pass a
// NopRecorder to keep call sites nil-check-free. Use NewWithRecorder to inject.
func New(cfg *config.GnetcliConfig) (*Client, error) {
	return NewWithRecorder(cfg, nil)
}

// NewWithRecorder builds a gnetcli client that records execute actions to the
// given audit recorder.
func NewWithRecorder(cfg *config.GnetcliConfig, recorder audit.Recorder) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	log.Printf("[gnetcli] Connecting to %s", addr)
	log.Printf("[gnetcli] Config: AuthToken=%v, Login=%s", cfg.AuthToken != "", cfg.Login)

	var opts []grpc.DialOption
	if cfg.TLS {
		return nil, fmt.Errorf("TLS for gnetcli not yet supported")
	}
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gnetcli at %s: %w", addr, err)
	}

	return &Client{
		conn:           conn,
		client:         proto.NewGnetcliClient(conn),
		authToken:      cfg.AuthToken,
		login:          cfg.Login,
		pass:           cfg.Password,
		recorder:       recorder,
		readTimeoutSec: cfg.ReadTimeoutSec,
		cmdTimeoutSec:  cfg.CmdTimeoutSec,
		maxTimeoutSec:  cfg.MaxTimeoutSec,
	}, nil
}

// resolveTimeouts computes the effective read/cmd timeouts (seconds) for a
// command. A non-zero requestSec overrides both defaults and is clamped to
// maxTimeoutSec when a cap is configured. A zero result means "leave unset" so
// the gnetcli server keeps its own default.
func (c *Client) resolveTimeouts(requestSec float64) (readSec, cmdSec float64) {
	readSec, cmdSec = c.readTimeoutSec, c.cmdTimeoutSec
	if requestSec > 0 {
		if c.maxTimeoutSec > 0 && requestSec > c.maxTimeoutSec {
			requestSec = c.maxTimeoutSec
		}
		// A stalled read (pager) and overall command are the two ways a big
		// diagnostic command hangs; raise both to the requested budget.
		readSec, cmdSec = requestSec, requestSec
	}
	return readSec, cmdSec
}

// classifyExecError maps a gnetcli gRPC error to a stable code and, when the
// device streamed output before the failure, the partial output embedded in
// the error message (gnetcli formats it as: last seen: %q). Returns partial ==
// "" when nothing recoverable is present.
func classifyExecError(err error) (code, partial string) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "read timeout error"):
		code = ErrCodeReadTimeout
	case strings.Contains(msg, "cmd timeout error"):
		code = ErrCodeCmdTimeout
	case strings.Contains(msg, "eof error"):
		code = ErrCodeEOF
	case strings.Contains(msg, "unknown device"):
		code = ErrCodeUnknownDevice
	default:
		code = ErrCodeUnknown
	}
	if i := strings.Index(msg, "last seen: "); i >= 0 {
		// The remainder is a Go-quoted literal (%q); unquote to recover the
		// real bytes the device sent (newlines, escapes, ...).
		if s, uerr := strconv.Unquote(msg[i+len("last seen: "):]); uerr == nil {
			partial = s
		}
	}
	return code, partial
}

// isDeviceExecError reports whether err is a command-level failure from the
// device (timeout, unknown device, ...) as opposed to a transport failure
// reaching the gnetcli server. gnetcli returns device-level failures as
// codes.Internal; connection problems surface as Unavailable/DeadlineExceeded.
func isDeviceExecError(err error) bool {
	return status.Code(err) == codes.Internal
}

// recordExec emits an audit event for a device command execution.
func (c *Client) recordExec(ctx context.Context, host, cmd string, res *ExecResult, err error, start time.Time) {
	if c.recorder == nil {
		return
	}
	e := audit.Event{
		Action:     audit.ActionExecute,
		Devices:    []string{host},
		Command:    cmd,
		Success:    err == nil && res != nil && res.Status == 0,
		DurationMs: time.Since(start).Milliseconds(),
	}
	if reqID, ok := ctx.Value(logging.RequestIDKey).(string); ok {
		e.RequestID = reqID
	}
	if err != nil {
		e.Error = &audit.Error{Type: "exec_err", Message: err.Error()}
	} else if res != nil && res.Status != 0 {
		msg := res.Error
		if msg == "" {
			msg = fmt.Sprintf("exit status %d", res.Status)
		}
		e.Error = &audit.Error{Type: "exec_failed", Message: msg}
	}
	c.recorder.Record(ctx, e)
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) getAuthHeader() string {
	// If auth token is provided, use it directly (it's already base64 encoded)
	if c.authToken != "" {
		return "Basic " + c.authToken
	}
	// Otherwise use login/password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(c.login+":"+c.pass))
}

func (c *Client) Exec(ctx context.Context, host, cmd string) (*ExecResult, error) {
	log.Printf("[gnetcli] Executing command on host=%s, cmd=%s", host, cmd)
	start := time.Now()

	authCtx := metadata.AppendToOutgoingContext(ctx, "authorization", c.getAuthHeader())

	res, err := c.client.Exec(authCtx, &proto.CMD{
		Host: host,
		Cmd:  cmd,
	})
	if err != nil {
		log.Printf("[gnetcli] Exec failed: %v", err)
		wrapped := fmt.Errorf("gnetcli exec failed: %w", err)
		c.recordExec(ctx, host, cmd, nil, wrapped, start)
		return nil, wrapped
	}

	log.Printf("[gnetcli] Exec success, status=%d", res.Status)

	out := res.OutStr
	if out == "" {
		out = string(res.Out)
	}
	errStr := res.ErrorStr
	if errStr == "" {
		errStr = string(res.Error)
	}

	result := &ExecResult{
		Output: out,
		Error:  errStr,
		Status: res.Status,
	}
	c.recordExec(ctx, host, cmd, result, nil, start)
	return result, nil
}

// ExecWithDevice executes command with device-specific parameters.
//
// timeoutSec is an optional per-request timeout (seconds); 0 uses the
// configured defaults. Device-level failures (read/cmd timeout, unknown
// device) are NOT returned as an error: they come back in the *ExecResult with
// a non-zero Status, an ErrorCode, and any partial output the device streamed
// before the failure. A non-nil error means the gnetcli server was
// unreachable.
func (c *Client) ExecWithDevice(ctx context.Context, host, cmd, vendor, login, password string, port int, timeoutSec float64) (*ExecResult, error) {
	log.Printf("[gnetcli] Executing command with device params: host=%s, port=%d, cmd=%s, vendor=%s", host, port, cmd, vendor)
	start := time.Now()

	authCtx := metadata.AppendToOutgoingContext(ctx, "authorization", c.getAuthHeader())

	hostParams := &proto.HostParams{
		Device: vendor,
		Credentials: &proto.Credentials{
			Login:    login,
			Password: password,
		},
	}

	if port > 0 && port != 22 {
		hostParams.Port = int32(port)
	}

	readSec, cmdSec := c.resolveTimeouts(timeoutSec)

	res, err := c.client.Exec(authCtx, &proto.CMD{
		Host:         host,
		Cmd:          cmd,
		HostParams:   hostParams,
		StringResult: true,
		ReadTimeout:  readSec,
		CmdTimeout:   cmdSec,
	})
	if err != nil {
		log.Printf("[gnetcli] ExecWithDevice failed: %v", err)
		wrapped := fmt.Errorf("gnetcli exec failed: %w", err)
		// Transport-level failure: the server was unreachable, nothing to
		// salvage. Propagate as an error.
		if !isDeviceExecError(err) {
			c.recordExec(ctx, host, cmd, nil, wrapped, start)
			return nil, wrapped
		}
		// Device-level failure: recover the machine-readable code and any
		// partial output the device sent before the cut-off, and return them
		// as a failed result rather than discarding them.
		code, partial := classifyExecError(err)
		result := &ExecResult{
			Output:    partial,
			Error:     wrapped.Error(),
			ErrorCode: code,
			Status:    1,
		}
		c.recordExec(ctx, host, cmd, result, nil, start)
		return result, nil
	}

	log.Printf("[gnetcli] ExecWithDevice success, status=%d", res.Status)

	out := res.OutStr
	if out == "" {
		out = string(res.Out)
	}
	errStr := res.ErrorStr
	if errStr == "" {
		errStr = string(res.Error)
	}

	result := &ExecResult{
		Output: out,
		Error:  errStr,
		Status: res.Status,
	}
	c.recordExec(ctx, host, cmd, result, nil, start)
	return result, nil
}
