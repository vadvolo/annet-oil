package gnetcli

import (
	"context"
	"encoding/base64"
	"fmt"

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
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", c.getAuthHeader())

	res, err := c.client.Exec(ctx, &proto.CMD{
		Host: host,
		Cmd:  cmd,
	})
	if err != nil {
		return nil, fmt.Errorf("gnetcli exec failed: %w", err)
	}

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
