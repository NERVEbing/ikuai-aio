package exporter

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/NERVEbing/ikuai-aio/api"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	client *api.Client

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

func NewMetrics(namespace string) *Metrics {
	client := api.NewClient()
	if err := client.Login(); err != nil {
		log.Fatalln(err)
	}

	return &Metrics{
		client:                    client,
		version:                   newDesc(namespace, "version", "", []string{"version", "arch", "ver_string"}),
		up:                        newDesc(namespace, "up", "", []string{"id"}),
		uptime:                    newDesc(namespace, "uptime", "", []string{"id"}),
		cpuUsageRatio:             newDesc(namespace, "cpu_usage_ratio", "", []string{"id"}),
		cpuTemperature:            newDesc(namespace, "cpu_temperature", "", nil),
		memorySizeKiloBytes:       newDesc(namespace, "memory_size_kilo_bytes", "", nil),
		memoryUsageKiloBytes:      newDesc(namespace, "memory_usage_kilo_bytes", "", nil),
		memoryCachedKiloBytes:     newDesc(namespace, "memory_cached_kilo_bytes", "", nil),
		memoryBuffersKiloBytes:    newDesc(namespace, "memory_buffers_kilo_bytes", "", nil),
		interfaceInfo:             newDesc(namespace, "interface_info", "", []string{"id", "interface", "comment", "internet", "parent_interface", "ip_addr", "display"}),
		deviceCount:               newDesc(namespace, "device_count", "", nil),
		deviceInfo:                newDesc(namespace, "device_info", "", []string{"id", "mac", "hostname", "ip_addr", "comment", "display"}),
		networkUploadTotalBytes:   newDesc(namespace, "network_upload_total_bytes", "", []string{"id", "display"}),
		networkDownloadTotalBytes: newDesc(namespace, "network_download_total_bytes", "", []string{"id", "display"}),
		networkUploadSpeedBytes:   newDesc(namespace, "network_upload_speed_bytes", "", []string{"id", "display"}),
		networkDownloadSpeedBytes: newDesc(namespace, "network_download_speed_bytes", "", []string{"id", "display"}),
		networkConnectCount:       newDesc(namespace, "network_connect_count", "", []string{"id", "display"}),
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
	defer func() {
		if err := recover(); err != nil {
			logger("recover", "error: %s", err)
			ch <- prometheus.MustNewConstMetric(
				m.up,
				prometheus.GaugeValue,
				0,
				"host",
			)
		}
	}()

	homepageShowSysStatResp, err := m.client.HomepageShowSysStat()
	if err != nil {
		logger("HomepageShowSysStat", "error: %s", err)
		return
	}
	monitorLanIPShowResp, err := m.client.MonitorLanIPShow()
	if err != nil {
		logger("MonitorLanIPShow", "error: %s", err)
		return
	}
	monitorIFaceShowResp, err := m.client.MonitorIFaceShow()
	if err != nil {
		logger("MonitorIFaceShow", "error: %s", err)
		return
	}

	sysStat := homepageShowSysStatResp.Data.SysStat
	iFaceStream := monitorIFaceShowResp.Data.IFaceStream
	iFaceCheck := monitorIFaceShowResp.Data.IFaceCheck
	lanDevices := monitorLanIPShowResp.Data.Data

	ch <- prometheus.MustNewConstMetric(
		m.version,
		prometheus.GaugeValue,
		1,
		sysStat.VerInfo.Version, sysStat.VerInfo.Arch, sysStat.VerInfo.VerString,
	)

	{
		ch <- prometheus.MustNewConstMetric(
			m.up,
			prometheus.GaugeValue,
			1,
			"host",
		)
		ch <- prometheus.MustNewConstMetric(
			m.uptime,
			prometheus.GaugeValue,
			float64(sysStat.Uptime),
			"host",
		)
		ch <- prometheus.MustNewConstMetric(
			m.networkUploadTotalBytes,
			prometheus.GaugeValue,
			float64(sysStat.Stream.TotalUp),
			"host", "host",
		)
		ch <- prometheus.MustNewConstMetric(
			m.networkDownloadTotalBytes,
			prometheus.GaugeValue,
			float64(sysStat.Stream.TotalDown),
			"host", "host",
		)
		ch <- prometheus.MustNewConstMetric(
			m.networkUploadSpeedBytes,
			prometheus.GaugeValue,
			float64(sysStat.Stream.Upload),
			"host", "host",
		)
		ch <- prometheus.MustNewConstMetric(
			m.networkDownloadSpeedBytes,
			prometheus.GaugeValue,
			float64(sysStat.Stream.Download),
			"host", "host",
		)
		ch <- prometheus.MustNewConstMetric(
			m.networkConnectCount,
			prometheus.GaugeValue,
			float64(sysStat.Stream.ConnectNum),
			"host", "host",
		)
	}

	if len(sysStat.Cpu) > 1 {
		sysStat.Cpu = sysStat.Cpu[1:]
	}
	for k, v := range sysStat.Cpu {
		s := v[:len(v)-1]
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			logger("cpuUsageRatio", "error: %s", err)
		}
		ch <- prometheus.MustNewConstMetric(
			m.cpuUsageRatio,
			prometheus.GaugeValue,
			f/100,
			fmt.Sprintf("core/%v", k),
		)
	}

	cpuTemp := 0.0
	if len(sysStat.CpuTemp) > 0 {
		cpuTemp = float64(sysStat.CpuTemp[0])
	}
	ch <- prometheus.MustNewConstMetric(
		m.cpuTemperature,
		prometheus.GaugeValue,
		cpuTemp,
	)

	ch <- prometheus.MustNewConstMetric(
		m.memorySizeKiloBytes,
		prometheus.GaugeValue,
		float64(sysStat.Memory.Total),
	)

	ch <- prometheus.MustNewConstMetric(
		m.memoryUsageKiloBytes,
		prometheus.GaugeValue,
		float64(sysStat.Memory.Total-sysStat.Memory.Available),
	)

	ch <- prometheus.MustNewConstMetric(
		m.memoryCachedKiloBytes,
		prometheus.GaugeValue,
		float64(sysStat.Memory.Cached),
	)

	ch <- prometheus.MustNewConstMetric(
		m.memoryBuffersKiloBytes,
		prometheus.GaugeValue,
		float64(sysStat.Memory.Buffers),
	)

	for _, i := range iFaceStream {
		internet := ""
		parentInterface := ""
		interfaceUp := 1
		interfaceID := fmt.Sprintf("interface/%s", i.Interface)
		interfaceUptime := int64(0)
		display := displayName(i.Interface)

		for _, n := range iFaceCheck {
			if n.Interface == i.Interface {
				internet = n.Internet
				parentInterface = n.ParentInterface
				if n.Result != "success" {
					interfaceUp = 0
				} else {
					if updateTime, err := strconv.Atoi(n.UpdateTime); err == nil {
						interfaceUptime = time.Now().Unix() - int64(updateTime)
					}
				}
			}
		}

		ch <- prometheus.MustNewConstMetric(
			m.interfaceInfo,
			prometheus.GaugeValue,
			1,
			interfaceID, i.Interface, i.Comment, internet, parentInterface, i.IpAddr, display,
		)

		ch <- prometheus.MustNewConstMetric(
			m.up,
			prometheus.GaugeValue,
			float64(interfaceUp),
			interfaceID,
		)

		ch <- prometheus.MustNewConstMetric(
			m.uptime,
			prometheus.GaugeValue,
			float64(interfaceUptime),
			interfaceID,
		)

		ch <- prometheus.MustNewConstMetric(
			m.networkUploadTotalBytes,
			prometheus.GaugeValue,
			float64(i.TotalUp),
			interfaceID, display,
		)

		ch <- prometheus.MustNewConstMetric(
			m.networkDownloadTotalBytes,
			prometheus.GaugeValue,
			float64(i.TotalDown),
			interfaceID, display,
		)

		ch <- prometheus.MustNewConstMetric(
			m.networkUploadSpeedBytes,
			prometheus.GaugeValue,
			float64(i.Upload),
			interfaceID, display,
		)

		ch <- prometheus.MustNewConstMetric(
			m.networkDownloadSpeedBytes,
			prometheus.GaugeValue,
			float64(i.Download),
			interfaceID, display,
		)

		if interfaceConnectCount, err := strconv.Atoi(i.ConnectNum); err == nil {
			ch <- prometheus.MustNewConstMetric(
				m.networkConnectCount,
				prometheus.GaugeValue,
				float64(interfaceConnectCount),
				interfaceID, display,
			)
		}
	}

	ch <- prometheus.MustNewConstMetric(
		m.deviceCount,
		prometheus.GaugeValue,
		float64(sysStat.OnlineUser.Count),
	)

	for _, i := range lanDevices {
		deviceID := fmt.Sprintf("device/%s", i.IpAddr)
		display := displayName(i.Comment, i.Hostname, i.IpAddr, i.Mac)

		ch <- prometheus.MustNewConstMetric(
			m.deviceInfo,
			prometheus.GaugeValue,
			1,
			deviceID, i.Mac, i.Hostname, i.IpAddr, i.Comment, display,
		)

		ch <- prometheus.MustNewConstMetric(
			m.networkUploadTotalBytes,
			prometheus.GaugeValue,
			float64(i.TotalUp),
			deviceID, display,
		)

		ch <- prometheus.MustNewConstMetric(
			m.networkUploadSpeedBytes,
			prometheus.GaugeValue,
			float64(i.Upload),
			deviceID, display,
		)

		ch <- prometheus.MustNewConstMetric(
			m.networkDownloadTotalBytes,
			prometheus.GaugeValue,
			float64(i.TotalDown),
			deviceID, display,
		)

		ch <- prometheus.MustNewConstMetric(
			m.networkDownloadSpeedBytes,
			prometheus.GaugeValue,
			float64(i.Download),
			deviceID, display,
		)

		ch <- prometheus.MustNewConstMetric(
			m.networkConnectCount,
			prometheus.GaugeValue,
			float64(i.ConnectNum),
			deviceID, display,
		)
	}
}

func newDesc(namespace string, metricName string, help string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(namespace+"_"+metricName, help, labels, nil)
}

func displayName(args ...string) string {
	for _, i := range args {
		if len(i) > 0 {
			return i
		}
	}
	return "unknown"
}
