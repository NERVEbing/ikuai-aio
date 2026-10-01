package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/NERVEbing/ikuai-aio/v4/api"
	"github.com/NERVEbing/ikuai-aio/v4/config"
	"github.com/NERVEbing/ikuai-aio/v4/exporter"
	"github.com/NERVEbing/ikuai-aio/v4/job"
	"golang.org/x/sync/errgroup"
)

func Run(ctx context.Context, conf config.Config, logger *slog.Logger) error {
	client, err := api.NewClient(conf.Router)
	if err != nil {
		return err
	}
	defer client.Close()
	group, groupCtx := errgroup.WithContext(ctx)
	metrics := exporter.NewMetrics(groupCtx, client, conf.ScrapeTimeout, conf.IPv6, logger)
	handler, err := exporter.NewHandler(metrics, conf.MetricsEnabled)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", conf.ListenAddress)
	if err != nil {
		return err
	}
	defer listener.Close()
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: conf.ScrapeTimeout + 5*time.Second, IdleTimeout: time.Minute}
	logger.Info("service started", "listen", listener.Addr().String(), "metrics", conf.MetricsEnabled, "ip_tasks", len(conf.IPObjects), "domain_tasks", len(conf.DomainRules))
	group.Go(func() error {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})
	group.Go(func() error {
		return job.Run(groupCtx, conf, job.NewWorker(client, job.NewFetcher(conf.Router.Timeout)), logger)
	})
	group.Go(func() error {
		<-groupCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			server.Close()
			return err
		}
		return nil
	})
	err = group.Wait()
	logger.Info("service stopped")
	return err
}
