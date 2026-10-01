package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/NERVEbing/ikuai-aio/v4/config"
)

func TestGracefulCancellationWithNoScheduledTasks(t *testing.T) {
	conf, err := config.Read([]string{"IKUAI_TOKEN=test-token", "IKUAI_EXPORTER_LISTEN_ADDR=127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err = Run(ctx, conf, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("service did not stop")
	}
}

func TestBindFailureReturnsWithoutLeavingScheduler(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	conf, err := config.Read([]string{"IKUAI_TOKEN=test-token", "IKUAI_EXPORTER_LISTEN_ADDR=" + listener.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	if err = Run(context.Background(), conf, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
		t.Fatal("occupied listen address accepted")
	}
}
