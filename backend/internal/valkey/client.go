package valkey

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

var ErrCommandFailed = errors.New("valkey command failed")

type Client struct {
	addr    string
	timeout time.Duration
}

func NewClient(addr string) *Client {
	return &Client{
		addr:    addr,
		timeout: 2 * time.Second,
	}
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.do(ctx, "PING")
	return err
}

func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.do(ctx, "INCR", key)
}

func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	seconds := int64((ttl + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	_, err := c.do(ctx, "EXPIRE", key, strconv.FormatInt(seconds, 10))
	return err
}

func (c *Client) do(ctx context.Context, args ...string) (int64, error) {
	if c.addr == "" {
		return 0, errors.New("valkey address is required")
	}

	dialer := net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	deadline := time.Now().Add(c.timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)

	if _, err := conn.Write(encodeCommand(args)); err != nil {
		return 0, err
	}
	return parseResponse(bufio.NewReader(conn))
}

func encodeCommand(args []string) []byte {
	var builder strings.Builder
	builder.WriteString("*")
	builder.WriteString(strconv.Itoa(len(args)))
	builder.WriteString("\r\n")
	for _, arg := range args {
		builder.WriteString("$")
		builder.WriteString(strconv.Itoa(len(arg)))
		builder.WriteString("\r\n")
		builder.WriteString(arg)
		builder.WriteString("\r\n")
	}
	return []byte(builder.String())
}

func parseResponse(reader *bufio.Reader) (int64, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return 0, err
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")

	switch prefix {
	case '+':
		return 0, nil
	case ':':
		value, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return 0, err
		}
		return value, nil
	case '-':
		return 0, fmt.Errorf("%w: %s", ErrCommandFailed, line)
	default:
		return 0, fmt.Errorf("unexpected valkey response prefix %q", prefix)
	}
}
