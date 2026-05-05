package exporter

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/NERVEbing/ikuai-aio/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	ikuaiapi "github.com/zy84338719/ikuai-api"
)

func Run(c *config.Config) error {
	if c.IKuaiExporterDisable {
		logger("Run", "ikuai exporter is disable, skip running")
		return nil
	}

	opts := []ikuaiapi.ClientOption{
		ikuaiapi.WithTimeout(c.HttpTimeout),
		ikuaiapi.WithInsecureSkipVerify(c.HttpInsecureSkipVerify),
	}
	if c.IKuaiToken != "" {
		opts = append(opts, ikuaiapi.WithToken(c.IKuaiToken))
	}

	client, err := ikuaiapi.NewClientWithLoginContext(context.Background(), c.IKuaiAddr, c.IKuaiUsername, c.IKuaiPassword, opts...)
	if err != nil {
		return fmt.Errorf("initial login failed: %w", err)
	}

	listenAddr := c.IKuaiExporterListenAddr
	metricsPath := "/metrics"
	metrics := NewMetrics("ikuai", client)
	registry := prometheus.NewRegistry()
	registry.MustRegister(metrics)

	http.Handle(metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`<html>
			<head><title>iKuai Prometheus Exporter</title></head>
			<body>
			<h1>iKuai Prometheus Exporter</h1>
			<p><a href='` + metricsPath + `'>Metrics</a></p>
			</body>
			</html>`))
		if err != nil {
			log.Println(err)
		}
	})

	logger("Run", "listen addr: %s, path: %s", listenAddr, metricsPath)

	return http.ListenAndServe(listenAddr, nil)
}

func logger(tag string, format string, v ...any) {
	s := fmt.Sprintf("[exporter] tag: [%s], %s", tag, fmt.Sprintf(format, v...))
	log.Printf("%s", s)
}
