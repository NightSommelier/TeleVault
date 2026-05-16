package valkey

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestEncodeCommandRESP(t *testing.T) {
	got := string(encodeCommand([]string{"INCR", "rate:key"}))
	want := "*2\r\n$4\r\nINCR\r\n$8\r\nrate:key\r\n"
	if got != want {
		t.Fatalf("encodeCommand() = %q, want %q", got, want)
	}
}

func TestClientAgainstValkey(t *testing.T) {
	addr := os.Getenv("TEST_VALKEY_ADDR")
	if addr == "" {
		t.Skip("set TEST_VALKEY_ADDR to run Valkey integration test")
	}

	client := NewClient(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	key := "t2d:test:valkey-client:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	first, err := client.Incr(ctx, key)
	if err != nil {
		t.Fatalf("first Incr() error = %v", err)
	}
	if first != 1 {
		t.Fatalf("first Incr() = %d, want 1", first)
	}
	second, err := client.Incr(ctx, key)
	if err != nil {
		t.Fatalf("second Incr() error = %v", err)
	}
	if second != 2 {
		t.Fatalf("second Incr() = %d, want 2", second)
	}
	if err := client.Expire(ctx, key, time.Second); err != nil {
		t.Fatalf("Expire() error = %v", err)
	}
}
