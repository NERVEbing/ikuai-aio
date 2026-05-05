package exporter

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	ikuaiapi "github.com/zy84338719/ikuai-api"
	"github.com/zy84338719/ikuai-api/service"
	"github.com/prometheus/client_golang/prometheus"
)

const defaultTimeout = 15 * time.Second

type Metrics struct {
	mu     sync.Mutex
	client *ikuaiapi.Client

	version *prometheus.Desc
	up      *prometheus.Desc
	uptime  *prometheus.Desc

	cpuUsageRatio  *prometheus.Desc
	cpuTemperature *prometheus.Desc

	memorySizeKiloBytes    *prometheus.Desc
	memoryUsageKiloBytes   *prometheus.Desc
	memoryCachedKiloBytes  *prometheus.Desc
	memoryBuffersKiloBytes *prometheus.Desc

	interfaceInfo *prometheus.Desc

	deviceCount *prometheus.Desc
	deviceInfo  *prometheus.Desc

	networkUploadTotalBytes   *prometheus.Desc
	networkDownloadTotalBytes *prometheus.Desc
	networkUploadSpeedBytes   *prometheus.Desc
	networkDownloadSpeedBytes *prometheus.Desc
	networkConnectCount       *prometheus.Desc
}

func NewMetrics(namespace string, client *ikuaiapi.Client) *Metrics {
	lbl := func(name, help string, labels ...string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, "", name), help, labels, nil)
	}
	return &Metrics{
		client:                    client,
		version:                   lbl("version", "Router version info (always 1)", "version", "arch", "ver_string"),
		up:                        lbl("up", "Router up status", "id"),
		uptime:                    lbl("uptime", "Router uptime in seconds", "id"),
		cpuUsageRatio:             lbl("cpu_usage_ratio", "CPU usage ratio (0–1)", "id"),
		cpuTemperature:            lbl("cpu_temperature", "CPU temperature in Celsius"),
		memorySizeKiloBytes:       lbl("memory_size_kilo_bytes", "Total memory in KiB"),
		memoryUsageKiloBytes:      lbl("memory_usage_kilo_bytes", "Used memory in KiB"),
		memoryCachedKiloBytes:     lbl("memory_cached_kilo_bytes", "Cached memory in KiB"),
		memoryBuffersKiloBytes:    lbl("memory_buffers_kilo_bytes", "Buffers memory in KiB"),
		interfaceInfo:             lbl("interface_info", "Network interface info (always 1)", "id", "interface", "comment", "internet", "parent_interface", "ip_addr", "display"),
		deviceCount:               lbl("device_count", "Total number of online LAN devices"),
		deviceInfo:                lbl("device_info", "LAN device info (always 1)", "id", "mac", "hostname", "ip_addr", "comment", "display"),
		networkUploadTotalBytes:   lbl("network_upload_total_bytes", "Total bytes uploaded", "id", "display", "ip_addr"),
		networkDownloadTotalBytes: lbl("network_download_total_bytes", "Total bytes downloaded", "id", "display", "ip_addr"),
		networkUploadSpeedBytes:   lbl("network_upload_speed_bytes", "Current upload speed in bytes/s", "id", "display", "ip_addr"),
		networkDownloadSpeedBytes: lbl("network_download_speed_bytes", "Current download speed in bytes/s", "id", "display", "ip_addr"),
		networkConnectCount:       lbl("network_connect_count", "Active connection count", "id", "display", "ip_addr"),
	}
}

func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- m.version
	ch <- m.up
	ch <- m.uptime
	ch <- m.cpuUsageRatio
	ch <- m.cpuTemperature
	ch <- m.memorySizeKiloBytes
	ch <- m.memoryUsageKiloBytes
	ch <- m.memoryCachedKiloBytes
	ch <- m.memoryBuffersKiloBytes
	ch <- m.interfaceInfo
	ch <- m.deviceCount
	ch <- m.deviceInfo
	ch <- m.networkUploadTotalBytes
	ch <- m.networkDownloadTotalBytes
	ch <- m.networkUploadSpeedBytes
	ch <- m.networkDownloadSpeedBytes
	ch <- m.networkConnectCount
}

func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	if !m.client.IsLoggedIn() {
		log.Println("session expired, re-logging in")
		if err := m.client.Login(ctx); err != nil {
			log.Printf("login failed: %v", err)
			ch <- gauge(m.up, 0, "host")
			return
		}
	}

	sys := service.NewSystemService(m.client)
	stat, err := sys.GetHomepage(ctx)
	if err != nil {
		log.Printf("GetHomepage: %v", err)
		ch <- gauge(m.up, 0, "host")
		return
	}
	ch <- gauge(m.up, 1, "host")

	if stat.VerInfo.VerString != "" {
		ch <- gaugeV(m.version, 1, stat.VerInfo.Version, stat.VerInfo.Arch, stat.VerInfo.VerString)
	}
	ch <- gauge(m.uptime, float64(stat.Uptime), "host")

	for i, cpuStr := range stat.CPU {
		if v, err := parseCPU(cpuStr); err == nil {
			ch <- gauge(m.cpuUsageRatio, v, fmt.Sprintf("core/%d", i))
		}
	}
	if len(stat.CPUTemp) > 0 {
		ch <- gaugeV(m.cpuTemperature, float64(stat.CPUTemp[0]))
	}
	if stat.Memory.Total > 0 {
		used := stat.Memory.Total - stat.Memory.Available
		ch <- gaugeV(m.memorySizeKiloBytes, float64(stat.Memory.Total))
		ch <- gaugeV(m.memoryUsageKiloBytes, float64(used))
		ch <- gaugeV(m.memoryCachedKiloBytes, float64(stat.Memory.Cached))
		ch <- gaugeV(m.memoryBuffersKiloBytes, float64(stat.Memory.Buffers))
	}

	mon := service.NewMonitorService(m.client)

	ifaces, err := mon.GetInterfaces(ctx)
	if err != nil {
		log.Printf("GetInterfaces: %v", err)
	} else {
		for _, chk := range ifaces.GetIFaceCheck() {
			iface := chk.Interface
			display := chk.Comment
			if display == "" {
				display = iface
			}
			id := "interface/" + iface
			ch <- gaugeV(m.interfaceInfo, 1, id, iface, chk.Comment, chk.Internet, chk.ParentInterface, chk.IPAddr, display)
		}
		for _, s := range ifaces.GetIFaceStream() {
			iface := s.Interface
			display := s.Comment
			if display == "" {
				display = iface
			}
			id := "interface/" + iface
			connectNum := parseConnectNum(s.ConnectNum)
			ch <- gauge(m.up, 1, id)
			ch <- gauge(m.uptime, 0, id)
			ch <- counterV(m.networkUploadTotalBytes, float64(s.TotalUp), id, display, s.IPAddr)
			ch <- counterV(m.networkDownloadTotalBytes, float64(s.TotalDown), id, display, s.IPAddr)
			ch <- gauge(m.networkUploadSpeedBytes, float64(s.Upload), id, display, s.IPAddr)
			ch <- gauge(m.networkDownloadSpeedBytes, float64(s.Download), id, display, s.IPAddr)
			ch <- gauge(m.networkConnectCount, float64(connectNum), id, display, s.IPAddr)
		}
	}

	devices, err := mon.GetLanIP(ctx)
	if err != nil {
		log.Printf("GetLanIP: %v", err)
	}
	ch <- gaugeV(m.deviceCount, float64(len(devices)))
	for _, d := range devices {
		display := d.Hostname
		if display == "" {
			display = d.IPAddr
		}
		id := fmt.Sprintf("device/%d", d.ID)
		ch <- gaugeV(m.deviceInfo, 1, id, d.Mac, d.Hostname, d.IPAddr, d.Comment, display)
		ch <- counterV(m.networkUploadTotalBytes, float64(d.TotalUp), id, display, d.IPAddr)
		ch <- counterV(m.networkDownloadTotalBytes, float64(d.TotalDown), id, display, d.IPAddr)
		ch <- gauge(m.networkUploadSpeedBytes, float64(d.Upload), id, display, d.IPAddr)
		ch <- gauge(m.networkDownloadSpeedBytes, float64(d.Download), id, display, d.IPAddr)
		ch <- gauge(m.networkConnectCount, float64(d.ConnectNum), id, display, d.IPAddr)
	}
}

func gauge(desc *prometheus.Desc, val float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, val, labels...)
}

func gaugeV(desc *prometheus.Desc, val float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, val, labels...)
}

func counterV(desc *prometheus.Desc, val float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(desc, prometheus.CounterValue, val, labels...)
}

func parseCPU(s string) (float64, error) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return v / 100, nil
}

func parseConnectNum(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}
