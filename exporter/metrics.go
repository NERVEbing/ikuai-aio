package exporter

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/NERVEbing/ikuai-aio/v4/api"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

type Reader interface {
	System(context.Context) (*api.System, error)
	Interfaces(context.Context) (*api.Interfaces, error)
	OnlineClients(context.Context, bool) ([]api.OnlineClient, error)
}

type Metrics struct {
	reader  Reader
	ctx     context.Context
	timeout time.Duration
	ipv6    bool
	logger  *slog.Logger
	desc    map[string]*prometheus.Desc
	flight  singleflight.Group
}

func NewMetrics(ctx context.Context, reader Reader, timeout time.Duration, ipv6 bool, logger *slog.Logger) *Metrics {
	m := &Metrics{ctx: ctx, reader: reader, timeout: timeout, ipv6: ipv6, logger: logger, desc: make(map[string]*prometheus.Desc)}
	define := func(name, help string, labels ...string) {
		m.desc[name] = prometheus.NewDesc("ikuai_"+name, help, labels, nil)
	}
	define("info", "Router firmware and host information.", "version", "arch", "ver_string", "hostname")
	define("up", "Whether the router or monitored interface is available.", "id")
	define("uptime_seconds", "Router or interface uptime in seconds.", "id")
	define("cpu_usage_ratio", "CPU utilization from zero to one.", "id")
	define("cpu_temperature_celsius", "CPU temperature in degrees Celsius.", "sensor")
	for _, kind := range []string{"total", "used", "available", "cached", "buffers"} {
		define("memory_"+kind+"_bytes", "Router memory in bytes.")
	}
	define("interface_info", "Network interface identity.", "id", "interface", "comment", "internet", "parent_interface", "ip_addr", "display")
	define("device_count", "Number of online terminals reported by the router.")
	define("device_info", "Online terminal identity, one series per IP address.", "id", "mac", "name", "ip_addr", "comment", "display")
	define("network_upload_total_bytes", "Cumulative uploaded bytes.", "id", "display", "ip_addr")
	define("network_download_total_bytes", "Cumulative downloaded bytes.", "id", "display", "ip_addr")
	define("network_upload_bytes_per_second", "Current upload rate in bytes per second.", "id", "display", "ip_addr")
	define("network_download_bytes_per_second", "Current download rate in bytes per second.", "id", "display", "ip_addr")
	define("network_connections", "Current network connections.", "id", "display", "ip_addr")
	define("scrape_success", "Whether every enabled collector succeeded.")
	define("collector_success", "Whether this collector succeeded.", "collector")
	define("collector_duration_seconds", "Wall time spent reading this collector, including pagination.", "collector")
	return m
}

func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	for _, descriptor := range m.desc {
		ch <- descriptor
	}
}

type result struct {
	name     string
	err      error
	duration time.Duration
}

type snapshot struct {
	system             *api.System
	interfaces         *api.Interfaces
	clients4, clients6 []api.OnlineClient
	results            []result
}

func (m *Metrics) read() *snapshot {
	ctx, cancel := context.WithTimeout(m.ctx, m.timeout)
	defer cancel()
	names := []string{"system", "interfaces", "clients_ipv4"}
	if m.ipv6 {
		names = append(names, "clients_ipv6")
	}
	s := &snapshot{results: make([]result, len(names))}
	var group errgroup.Group
	for index, name := range names {
		group.Go(func() error {
			start := time.Now()
			var err error
			switch name {
			case "system":
				s.system, err = m.reader.System(ctx)
			case "interfaces":
				s.interfaces, err = m.reader.Interfaces(ctx)
			case "clients_ipv4":
				s.clients4, err = m.reader.OnlineClients(ctx, false)
			case "clients_ipv6":
				s.clients6, err = m.reader.OnlineClients(ctx, true)
			}
			s.results[index] = result{name: name, err: err, duration: time.Since(start)}
			if err != nil {
				m.logger.Warn("collector failed", "collector", name, "error", err)
			}
			return nil // Preserve successful collectors when another endpoint fails.
		})
	}
	group.Wait()
	return s
}

func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	// Concurrent Prometheus scrapes share the same in-flight router snapshot.
	value, _, _ := m.flight.Do("snapshot", func() (any, error) { return m.read(), nil })
	s := value.(*snapshot)
	emit := func(name string, kind prometheus.ValueType, value float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(m.desc[name], kind, value, labels...)
	}
	gauge := func(name string, value float64, labels ...string) {
		emit(name, prometheus.GaugeValue, value, labels...)
	}
	success := true
	for _, r := range s.results {
		ok := r.err == nil
		success = success && ok
		gauge("collector_success", boolean(ok), r.name)
		gauge("collector_duration_seconds", r.duration.Seconds(), r.name)
	}
	gauge("scrape_success", boolean(success))
	gauge("up", boolean(s.system != nil && s.results[0].err == nil), "host")
	traffic := func(id, display, address string, t api.Traffic) {
		emit("network_upload_total_bytes", prometheus.CounterValue, float64(t.TotalUp), id, display, address)
		emit("network_download_total_bytes", prometheus.CounterValue, float64(t.TotalDown), id, display, address)
		gauge("network_upload_bytes_per_second", float64(t.Upload), id, display, address)
		gauge("network_download_bytes_per_second", float64(t.Download), id, display, address)
		gauge("network_connections", float64(t.Connections), id, display, address)
	}
	if sys := s.system; sys != nil && s.results[0].err == nil {
		gauge("info", 1, sys.Version.Version, sys.Version.Arch, sys.Version.Description, sys.Hostname)
		gauge("uptime_seconds", float64(sys.Uptime), "host")
		traffic("host", "host", "host", sys.Stream)
		for index, value := range sys.CPU {
			usage, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "%")), 64)
			if err != nil || math.IsNaN(usage) || math.IsInf(usage, 0) || usage < 0 || usage > 100 {
				continue
			}
			id := "all"
			if index > 0 {
				id = fmt.Sprintf("core/%d", index-1)
			}
			gauge("cpu_usage_ratio", usage/100, id)
		}
		for sensor, temperature := range sys.CPUTemp {
			gauge("cpu_temperature_celsius", float64(temperature), strconv.Itoa(sensor))
		}
		gauge("memory_total_bytes", float64(sys.Memory.Total)*1024)
		gauge("memory_used_bytes", max(float64(sys.Memory.Total-sys.Memory.Available), 0)*1024)
		gauge("memory_available_bytes", float64(sys.Memory.Available)*1024)
		gauge("memory_cached_bytes", float64(sys.Memory.Cached)*1024)
		gauge("memory_buffers_bytes", float64(sys.Memory.Buffers)*1024)
		gauge("device_count", float64(sys.OnlineUser.Count))
	}
	if interfaces := s.interfaces; interfaces != nil && s.results[1].err == nil {
		checks := make(map[string]api.InterfaceCheck, len(interfaces.Checks))
		for _, check := range interfaces.Checks {
			checks[check.Interface] = check
		}
		seen := map[string]bool{}
		for _, stream := range interfaces.Streams {
			if stream.Interface == "" || seen[stream.Interface] {
				continue
			}
			seen[stream.Interface] = true
			id, display := "interface/"+stream.Interface, displayName(stream.Comment, stream.Interface)
			check, found := checks[stream.Interface]
			gauge("interface_info", 1, id, stream.Interface, stream.Comment, check.Internet, check.Parent, stream.Address, display)
			if found {
				gauge("up", boolean(check.Result == "success"), id)
				if updated, err := strconv.ParseInt(check.Updated, 10, 64); err == nil && updated > 0 && updated <= time.Now().Unix() && check.Result == "success" {
					gauge("uptime_seconds", float64(time.Now().Unix()-updated), id)
				}
			}
			connections, err := strconv.ParseFloat(stream.Connections, 64)
			trafficData := api.Traffic{Upload: stream.Upload, Download: stream.Download, TotalUp: stream.TotalUp, TotalDown: stream.TotalDown}
			// "--" means the firmware does not expose a connection count.
			emit("network_upload_total_bytes", prometheus.CounterValue, float64(trafficData.TotalUp), id, display, stream.Address)
			emit("network_download_total_bytes", prometheus.CounterValue, float64(trafficData.TotalDown), id, display, stream.Address)
			gauge("network_upload_bytes_per_second", float64(trafficData.Upload), id, display, stream.Address)
			gauge("network_download_bytes_per_second", float64(trafficData.Download), id, display, stream.Address)
			if err == nil && !math.IsNaN(connections) && !math.IsInf(connections, 0) {
				gauge("network_connections", connections, id, display, stream.Address)
			}
		}
	}
	seen := map[string]bool{}
	for index, clients := range [][]api.OnlineClient{s.clients4, s.clients6} {
		if index+2 >= len(s.results) || s.results[index+2].err != nil {
			continue
		}
		for _, client := range clients {
			if client.Address == "" || seen[client.Address] {
				continue
			}
			seen[client.Address] = true
			id := "device/" + client.Address
			display := displayName(client.Comment, client.Name, client.Address, client.MAC)
			gauge("device_info", 1, id, client.MAC, client.Name, client.Address, client.Comment, display)
			traffic(id, display, client.Address, client.Traffic)
		}
	}
}

func boolean(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func displayName(names ...string) string {
	for _, name := range names {
		if name != "" {
			return name
		}
	}
	return "unknown"
}
