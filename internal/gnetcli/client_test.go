package gnetcli

import (
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestClassifyExecError(t *testing.T) {
	tests := []struct {
		name        string
		errMsg      string
		wantCode    string
		wantPartial string
	}{
		{
			name:        "read timeout with partial output",
			errMsg:      `gnetcli exec failed: rpc error: code = Internal desc = read timeout error. last seen: "------ show interfaces te1/0/1 ------\r\ntengigabitethernet"`,
			wantCode:    ErrCodeReadTimeout,
			wantPartial: "------ show interfaces te1/0/1 ------\r\ntengigabitethernet",
		},
		{
			name:        "cmd timeout",
			errMsg:      `read stuff cmd timeout error. last seen: "partial"`,
			wantCode:    ErrCodeCmdTimeout,
			wantPartial: "partial",
		},
		{
			name:        "eof",
			errMsg:      `eof error. last seen: "bye"`,
			wantCode:    ErrCodeEOF,
			wantPartial: "bye",
		},
		{
			name:     "unknown device",
			errMsg:   `rpc error: code = Internal desc = unknown device eltex5300`,
			wantCode: ErrCodeUnknownDevice,
		},
		{
			name:     "unclassified",
			errMsg:   `some other failure`,
			wantCode: ErrCodeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, partial := classifyExecError(fmt.Errorf("%s", tt.errMsg))
			if code != tt.wantCode {
				t.Errorf("code = %q, want %q", code, tt.wantCode)
			}
			if partial != tt.wantPartial {
				t.Errorf("partial = %q, want %q", partial, tt.wantPartial)
			}
		})
	}
}

func TestIsDeviceExecError(t *testing.T) {
	if !isDeviceExecError(status.Error(codes.Internal, "read timeout error")) {
		t.Error("codes.Internal should be a device exec error")
	}
	if isDeviceExecError(status.Error(codes.Unavailable, "connection refused")) {
		t.Error("codes.Unavailable is a transport error, not a device exec error")
	}
	if isDeviceExecError(fmt.Errorf("plain error")) {
		t.Error("non-status error should not be treated as a device exec error")
	}
}

func TestResolveTimeouts(t *testing.T) {
	tests := []struct {
		name              string
		client            Client
		request           float64
		wantRead, wantCmd float64
	}{
		{
			name:     "no request, no defaults -> unset",
			client:   Client{},
			request:  0,
			wantRead: 0, wantCmd: 0,
		},
		{
			name:     "defaults used when no request",
			client:   Client{readTimeoutSec: 15, cmdTimeoutSec: 20},
			request:  0,
			wantRead: 15, wantCmd: 20,
		},
		{
			name:     "request overrides both",
			client:   Client{readTimeoutSec: 15, cmdTimeoutSec: 20},
			request:  45,
			wantRead: 45, wantCmd: 45,
		},
		{
			name:     "request clamped to max",
			client:   Client{maxTimeoutSec: 60},
			request:  120,
			wantRead: 60, wantCmd: 60,
		},
		{
			name:     "request under max is honored",
			client:   Client{maxTimeoutSec: 60},
			request:  30,
			wantRead: 30, wantCmd: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRead, gotCmd := tt.client.resolveTimeouts(tt.request)
			if gotRead != tt.wantRead || gotCmd != tt.wantCmd {
				t.Errorf("resolveTimeouts(%v) = (%v, %v), want (%v, %v)",
					tt.request, gotRead, gotCmd, tt.wantRead, tt.wantCmd)
			}
		})
	}
}
