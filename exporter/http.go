package exporter

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewHandler(metrics *Metrics, enabled bool) (http.Handler, error) {
	mux := http.NewServeMux()
	if enabled {
		registry := prometheus.NewRegistry()
		if err := registry.Register(metrics); err != nil {
			return nil, err
		}
		mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{MaxRequestsInFlight: 8}))
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<!doctype html><html lang=\"en\"><title>iKuai AIO v4</title><h1>iKuai AIO v4</h1><p>iKuai 4.x REST API exporter</p><a href=\"/metrics\">Metrics</a></html>"))
	})
	return mux, nil
}
