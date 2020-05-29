// +build windows

package collector

import (
    "context"
    "io/ioutil"
    "github.com/bitly/go-simplejson"
    "github.com/docker/docker/client"
	"github.com/Microsoft/hcsshim"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

func init() {
	registerCollector("container", NewContainerMetricsCollector)
}

func ContainerStats(ID string) string {
    cli, err := client.NewClient("tcp://127.0.0.1:2376", "v1.39", nil, nil)
	if err != nil {
		log.Error(err)
	}
    images, err := cli.ContainerStats(context.Background(), ID[:10],false)
	if err != nil {
		log.Error(err)
	}
    s,_ := ioutil.ReadAll(images.Body)
    cnnn := string(s)
    ss_json, _ := simplejson.NewJson([]byte(cnnn))
    ss_name,err := ss_json.Get("name").String()
    //fmt.Printf(" %s\n",ss_name[1:])
    return ss_name[1:]
}
// A ContainerMetricsCollector is a Prometheus collector for containers metrics
type ContainerMetricsCollector struct {
	// Presence
	ContainerAvailable *prometheus.Desc

	// Number of containers
	ContainersCount *prometheus.Desc
	// memory
	UsageCommitBytes            *prometheus.Desc
	UsageCommitPeakBytes        *prometheus.Desc
	UsagePrivateWorkingSetBytes *prometheus.Desc

	// CPU
	RuntimeTotal  *prometheus.Desc
	RuntimeUser   *prometheus.Desc
	RuntimeKernel *prometheus.Desc

	// Network
	BytesReceived          *prometheus.Desc
	BytesSent              *prometheus.Desc
	PacketsReceived        *prometheus.Desc
	PacketsSent            *prometheus.Desc
	DroppedPacketsIncoming *prometheus.Desc
	DroppedPacketsOutgoing *prometheus.Desc
}

// NewContainerMetricsCollector constructs a new ContainerMetricsCollector
func NewContainerMetricsCollector() (Collector, error) {
	const subsystem = "container"
	return &ContainerMetricsCollector{
		ContainerAvailable: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "available"),
			"Available",
			[]string{"container_id", "container_name"},
			nil,
		),
		ContainersCount: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "count"),
			"Number of containers",
			nil,
			nil,
		),
		UsageCommitBytes: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "memory_usage_commit_bytes"),
			"Memory Usage Commit Bytes",
			[]string{"container_id", "container_name"},
			nil,
		),
		UsageCommitPeakBytes: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "memory_usage_commit_peak_bytes"),
			"Memory Usage Commit Peak Bytes",
			[]string{"container_id", "container_name"},
			nil,
		),
		UsagePrivateWorkingSetBytes: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "memory_usage_private_working_set_bytes"),
			"Memory Usage Private Working Set Bytes",
			[]string{"container_id", "container_name"},
			nil,
		),
		RuntimeTotal: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "cpu_usage_seconds_total"),
			"Total Run time in Seconds",
			[]string{"container_id", "container_name"},
			nil,
		),
		RuntimeUser: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "cpu_usage_seconds_usermode"),
			"Run Time in User mode in Seconds",
			[]string{"container_id", "container_name"},
			nil,
		),
		RuntimeKernel: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "cpu_usage_seconds_kernelmode"),
			"Run time in Kernel mode in Seconds",
			[]string{"container_id", "container_name"},
			nil,
		),
		BytesReceived: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "network_receive_bytes_total"),
			"Bytes Received on Interface",
			[]string{"container_id", "container_name", "interface"},
			nil,
		),
		BytesSent: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "network_transmit_bytes_total"),
			"Bytes Sent on Interface",
			[]string{"container_id", "container_name", "interface"},
			nil,
		),
		PacketsReceived: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "network_receive_packets_total"),
			"Packets Received on Interface",
			[]string{"container_id", "container_name", "interface"},
			nil,
		),
		PacketsSent: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "network_transmit_packets_total"),
			"Packets Sent on Interface",
			[]string{"container_id", "container_name", "interface"},
			nil,
		),
		DroppedPacketsIncoming: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "network_receive_packets_dropped_total"),
			"Dropped Incoming Packets on Interface",
			[]string{"container_id", "container_name", "interface"},
			nil,
		),
		DroppedPacketsOutgoing: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "network_transmit_packets_dropped_total"),
			"Dropped Outgoing Packets on Interface",
			[]string{"container_id", "container_name", "interface"},
			nil,
		),
	}, nil
}

// Collect sends the metric values for each metric
// to the provided prometheus Metric channel.
func (c *ContainerMetricsCollector) Collect(ctx *ScrapeContext, ch chan<- prometheus.Metric) error {
	if desc, err := c.collect(ch); err != nil {
		log.Error("failed collecting ContainerMetricsCollector metrics:", desc, err)
		return err
	}
	return nil
}

// containerClose closes the container resource
func containerClose(c hcsshim.Container) {
	err := c.Close()
	if err != nil {
		log.Error(err)
	}
}

func (c *ContainerMetricsCollector) collect(ch chan<- prometheus.Metric) (*prometheus.Desc, error) {

	// Types Container is passed to get the containers compute systems only
	containers, err := hcsshim.GetContainers(hcsshim.ComputeSystemQuery{Types: []string{"Container"}})
	if err != nil {
		log.Error("Err in Getting containers:", err)
		return nil, err
	}

	count := len(containers)

	ch <- prometheus.MustNewConstMetric(
		c.ContainersCount,
		prometheus.GaugeValue,
		float64(count),
	)
	if count == 0 {
		return nil, nil
	}

	for _, containerDetails := range containers {
		containerId := containerDetails.ID
		containerName := ContainerStats(containerId)

		container, err := hcsshim.OpenContainer(containerId)
		if container != nil {
			defer containerClose(container)
		}
		if err != nil {
			log.Error("err in opening container: ", containerId, err)
			continue
		}

		cstats, err := container.Statistics()
		if err != nil {
			log.Error("err in fetching container Statistics: ", containerId, err)
			continue
		}
		// HCS V1 is for docker runtime. Add the docker:// prefix on container_id
		containerId = "docker://" + containerId

		ch <- prometheus.MustNewConstMetric(
			c.ContainerAvailable,
			prometheus.CounterValue,
			1,
			containerId, containerName,
		)
		ch <- prometheus.MustNewConstMetric(
			c.UsageCommitBytes,
			prometheus.GaugeValue,
			float64(cstats.Memory.UsageCommitBytes),
			containerId, containerName,
		)
		ch <- prometheus.MustNewConstMetric(
			c.UsageCommitPeakBytes,
			prometheus.GaugeValue,
			float64(cstats.Memory.UsageCommitPeakBytes),
			containerId, containerName,
		)
		ch <- prometheus.MustNewConstMetric(
			c.UsagePrivateWorkingSetBytes,
			prometheus.GaugeValue,
			float64(cstats.Memory.UsagePrivateWorkingSetBytes),
			containerId, containerName,
		)
		ch <- prometheus.MustNewConstMetric(
			c.RuntimeTotal,
			prometheus.CounterValue,
			float64(cstats.Processor.TotalRuntime100ns)*ticksToSecondsScaleFactor,
			containerId, containerName,
		)
		ch <- prometheus.MustNewConstMetric(
			c.RuntimeUser,
			prometheus.CounterValue,
			float64(cstats.Processor.RuntimeUser100ns)*ticksToSecondsScaleFactor,
			containerId, containerName,
		)
		ch <- prometheus.MustNewConstMetric(
			c.RuntimeKernel,
			prometheus.CounterValue,
			float64(cstats.Processor.RuntimeKernel100ns)*ticksToSecondsScaleFactor,
			containerId, containerName,
		)

		if len(cstats.Network) == 0 {
			log.Info("No Network Stats for container: ", containerId)
			continue
		}

		networkStats := cstats.Network

		for _, networkInterface := range networkStats {
			ch <- prometheus.MustNewConstMetric(
				c.BytesReceived,
				prometheus.CounterValue,
				float64(networkInterface.BytesReceived),
				containerId, containerName, networkInterface.EndpointId,
			)
			ch <- prometheus.MustNewConstMetric(
				c.BytesSent,
				prometheus.CounterValue,
				float64(networkInterface.BytesSent),
				containerId, containerName, networkInterface.EndpointId,
			)
			ch <- prometheus.MustNewConstMetric(
				c.PacketsReceived,
				prometheus.CounterValue,
				float64(networkInterface.PacketsReceived),
				containerId, containerName, networkInterface.EndpointId,
			)
			ch <- prometheus.MustNewConstMetric(
				c.PacketsSent,
				prometheus.CounterValue,
				float64(networkInterface.PacketsSent),
				containerId, containerName, networkInterface.EndpointId,
			)
			ch <- prometheus.MustNewConstMetric(
				c.DroppedPacketsIncoming,
				prometheus.CounterValue,
				float64(networkInterface.DroppedPacketsIncoming),
				containerId, containerName, networkInterface.EndpointId,
			)
			ch <- prometheus.MustNewConstMetric(
				c.DroppedPacketsOutgoing,
				prometheus.CounterValue,
				float64(networkInterface.DroppedPacketsOutgoing),
				containerId, containerName, networkInterface.EndpointId,
			)
			break
		}
	}

	return nil, nil
}
