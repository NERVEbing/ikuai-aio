package exporter

import (
	"fmt"
	"log"
	"net/http"

	"github.com/NERVEbing/ikuai-aio/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Run(c *config.Config) error {
	if c.IKuaiExporterDisable {
		logger("Run", "ikuai exporter is disable, skip running")
		return nil
	}
	listenAddr := c.IKuaiExporterListenAddr
	metricsPath := "/metrics"
	metrics := NewMetrics("ikuai")
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
			log.Fatalln(err)
		}
	})

	logger("Run", "listen addr: %s, path: %s", listenAddr, metricsPath)

	return http.ListenAndServe(listenAddr, nil)
}

func logger(tag string, format string, v ...any) {
	s := fmt.Sprintf("[exporter] tag: [%s], %s", tag, fmt.Sprintf(format, v...))
	log.Printf("%s", s)
}
