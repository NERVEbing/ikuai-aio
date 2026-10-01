package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NERVEbing/ikuai-aio/v4/config"
	"github.com/NERVEbing/ikuai-aio/v4/internal/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger, os.Args[1:]); err != nil {
		logger.Error("service failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, args []string) error {
	if len(args) == 1 && args[0] == "healthcheck" {
		return healthcheck()
	}
	if len(args) > 1 || (len(args) == 1 && args[0] != "check-config") {
		return fmt.Errorf("usage: ikuai-aio [check-config|healthcheck]")
	}
	conf, err := config.Load()
	if err != nil {
		return err
	}
	if len(args) == 1 {
		logger.Info("configuration valid", "ip_tasks", len(conf.IPObjects), "domain_tasks", len(conf.DomainRules))
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return app.Run(ctx, conf, logger)
}

func healthcheck() error {
	address := os.Getenv("IKUAI_EXPORTER_LISTEN_ADDR")
	if address == "" {
		address = "0.0.0.0:8000"
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	if host == "::" {
		host = "::1"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck returned HTTP %d", resp.StatusCode)
	}
	return nil
}
