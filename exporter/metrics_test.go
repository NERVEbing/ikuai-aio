package exporter

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/NERVEbing/ikuai-aio/v4/api"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type fakeReader struct {
	fail       string
	system     api.System
	interfaces api.Interfaces
	clients    []api.OnlineClient
}

func (r fakeReader) System(context.Context) (*api.System, error) {
	if r.fail == "all" {
		return nil, errors.New("offline")
	}
	s := r.system
	return &s, nil
}
func (r fakeReader) Interfaces(context.Context) (*api.Interfaces, error) {
	if r.fail == "all" {
		return nil, errors.New("offline")
	}
	s := r.interfaces
	return &s, nil
}
func (r fakeReader) OnlineClients(_ context.Context, ipv6 bool) ([]api.OnlineClient, error) {
	if r.fail == "all" || ipv6 && r.fail == "ipv6" {
		return nil, errors.New("offline")
	}
	return r.clients, nil
}

func fixtureReader() fakeReader {
	r := fakeReader{clients: []api.OnlineClient{{Address: "192.168.1.2", Name: "Laptop", Traffic: api.Traffic{TotalUp: 100, TotalDown: 200}}, {Address: "192.168.1.2", Name: "Duplicate"}}}
	r.system.CPU = []string{"12%", "6%", "", "NaN", "bad"}
	r.system.CPUTemp = []api.Number{42.6}
	r.system.Memory.Total = 1024
	r.system.Memory.Available = 512
	r.system.OnlineUser.Count = 1
	r.interfaces.Streams = []api.InterfaceStream{{Interface: "lan1", Address: "192.168.1.1", Connections: "--"}, {Interface: "lan1", Address: "192.168.1.1"}}
	return r
}

func metricRegistry(t *testing.T, r fakeReader, ipv6 bool) (*prometheus.Registry, *Metrics) {
	t.Helper()
	m := NewMetrics(context.Background(), r, time.Second, ipv6, slog.New(slog.NewTextHandler(io.Discard, nil)))
	registry := prometheus.NewPedanticRegistry()
	if err := registry.Register(m); err != nil {
		t.Fatal(err)
	}
	return registry, m
}

func gather(t *testing.T, registry *prometheus.Registry) map[string]*dto.MetricFamily {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*dto.MetricFamily{}
	for _, f := range families {
		out[f.GetName()] = f
	}
	return out
}

func TestMetricsUseBytesCountersAndDeduplicateAddresses(t *testing.T) {
	registry, _ := metricRegistry(t, fixtureReader(), true)
	families := gather(t, registry)
	if got := families["ikuai_memory_used_bytes"].Metric[0].GetGauge().GetValue(); got != 512*1024 {
		t.Fatalf("memory units wrong: %v", got)
	}
	if families["ikuai_network_upload_total_bytes"].GetType() != dto.MetricType_COUNTER {
		t.Fatal("cumulative traffic must be counters")
	}
	if got := len(families["ikuai_device_info"].Metric); got != 1 {
		t.Fatalf("duplicate devices emitted: %d", got)
	}
	if got := len(families["ikuai_interface_info"].Metric); got != 1 {
		t.Fatalf("duplicate interfaces emitted: %d", got)
	}
	if got := len(families["ikuai_cpu_usage_ratio"].Metric); got != 2 {
		t.Fatalf("malformed CPU values emitted: %d", got)
	}
	if got := len(families["ikuai_collector_success"].Metric); got != 4 {
		t.Fatalf("missing collectors: %d", got)
	}
}

func TestPartialFailureKeepsHealthyCollectors(t *testing.T) {
	r := fixtureReader()
	r.fail = "ipv6"
	registry, _ := metricRegistry(t, r, true)
	families := gather(t, registry)
	if families["ikuai_scrape_success"].Metric[0].GetGauge().GetValue() != 0 {
		t.Fatal("partial scrape reported as success")
	}
	if families["ikuai_up"].Metric[0].GetGauge().GetValue() != 1 || families["ikuai_device_info"] == nil {
		t.Fatal("healthy collectors discarded")
	}
}

func TestRouterFailureAlwaysReportsDown(t *testing.T) {
	r := fixtureReader()
	r.fail = "all"
	registry, _ := metricRegistry(t, r, true)
	families := gather(t, registry)
	if families["ikuai_up"].Metric[0].GetGauge().GetValue() != 0 || families["ikuai_scrape_success"].Metric[0].GetGauge().GetValue() != 0 {
		t.Fatal("router failure hidden")
	}
	if families["ikuai_memory_total_bytes"] != nil || families["ikuai_device_info"] != nil {
		t.Fatal("fabricated measurements emitted")
	}
}

func TestDisabledIPv6AndConcurrentScrapes(t *testing.T) {
	registry, _ := metricRegistry(t, fixtureReader(), false)
	if got := len(gather(t, registry)["ikuai_collector_success"].Metric); got != 3 {
		t.Fatal("disabled collector still active")
	}
	var group sync.WaitGroup
	for i := 0; i < 12; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := registry.Gather(); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
}

func TestHealthAndDisabledMetricsDoNotContactRouter(t *testing.T) {
	handler, err := NewHandler(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path string
		code int
	}{{"/healthz", 200}, {"/metrics", 404}, {"/unknown", 404}} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.code {
			t.Fatalf("%s returned %d", tc.path, w.Code)
		}
	}
}
