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
	exportermetrics "github.com/zy84338719/ikuai_exporter/metrics"
)

func Run(c *config.Config) error {
	if c.IKuaiExporterDisable {
		logger("Run", "ikuai exporter is disabled, skipping")
		return nil
	}

	var collector prometheus.Collector

	if c.IKuaiToken != "" {
		httpsAddr := exportermetrics.ToHTTPS(c.IKuaiAddr)
		v4client := ikuaiapi.NewV4RESTClient(httpsAddr, c.IKuaiToken,
			ikuaiapi.WithV4RawMode(false),
		)
		collector = exportermetrics.NewV4Collector("ikuai", v4client)
		logger("Run", "v4 REST token mode  addr=%s", httpsAddr)
	} else {
		opts := []ikuaiapi.ClientOption{
			ikuaiapi.WithTimeout(c.HttpTimeout),
			ikuaiapi.WithInsecureSkipVerify(c.HttpInsecureSkipVerify),
		}
		client, err := ikuaiapi.NewClientWithLoginContext(context.Background(), c.IKuaiAddr, c.IKuaiUsername, c.IKuaiPassword, opts...)
		if err != nil {
			return fmt.Errorf("initial login failed: %w", err)
		}
		collector = exportermetrics.NewCollector("ikuai", client)
		logger("Run", "session mode  addr=%s  username=%s", c.IKuaiAddr, c.IKuaiUsername)
	}

	listenAddr := c.IKuaiExporterListenAddr
	metricsPath := "/metrics"

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

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
