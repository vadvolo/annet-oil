package gnetcli

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	proto "github.com/annetutil/gnetcli/pkg/server/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"annet-oil/internal/config"
)

type Client struct {
	conn      *grpc.ClientConn
	client    proto.GnetcliClient
	authToken string
	login     string
	pass      string
}

type ExecResult struct {
	Output string
	Error  string
	Status int32
}

func New(cfg *config.GnetcliConfig) (*Client, error) {
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
		conn:      conn,
		client:    proto.NewGnetcliClient(conn),
		authToken: cfg.AuthToken,
		login:     cfg.Login,
		pass:      cfg.Password,
	}, nil
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

	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", c.getAuthHeader())

	res, err := c.client.Exec(ctx, &proto.CMD{
		Host: host,
		Cmd:  cmd,
	})
	if err != nil {
		log.Printf("[gnetcli] Exec failed: %v", err)
		return nil, fmt.Errorf("gnetcli exec failed: %w", err)
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

	return &ExecResult{
		Output: out,
		Error:  errStr,
		Status: res.Status,
	}, nil
}

// ExecWithDevice executes command with device-specific parameters
func (c *Client) ExecWithDevice(ctx context.Context, host, cmd, vendor, login, password string) (*ExecResult, error) {
	log.Printf("[gnetcli] Executing command with device params: host=%s, cmd=%s, vendor=%s, login=%s, password=%s", host, cmd, vendor, login, password)

	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", c.getAuthHeader())

	// Build host_params
	hostParams := &proto.HostParams{
		Device: vendor,
		Credentials: &proto.Credentials{
			Login:    login,
			Password: password,
		},
	}

	res, err := c.client.Exec(ctx, &proto.CMD{
		Host:         host,
		Cmd:          cmd,
		HostParams:   hostParams,
		StringResult: true,
	})
	if err != nil {
		log.Printf("[gnetcli] ExecWithDevice failed: %v", err)
		return nil, fmt.Errorf("gnetcli exec failed: %w", err)
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

	return &ExecResult{
		Output: out,
		Error:  errStr,
		Status: res.Status,
	}, nil
}
