package collectors

import (
	"fmt"
	"net"

	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "free5gc_upf"

// GTP5GCollector implements prometheus.Collector.
type GTP5GCollector struct {
	ifindex uint32
	ifname  string

	ulPktCnt  *prometheus.Desc
	dlPktCnt  *prometheus.Desc
	ulByteCnt *prometheus.Desc
	dlByteCnt *prometheus.Desc
	ulDropCnt *prometheus.Desc
	dlDropCnt *prometheus.Desc

	activePDRs *prometheus.Desc

	ifRxBytes   *prometheus.Desc
	ifTxBytes   *prometheus.Desc
	ifRxPackets *prometheus.Desc
	ifTxPackets *prometheus.Desc
}

var labelNames = []string{"pdr_id", "seid", "ue_addr", "teid"}

func NewGTP5GCollector(ifname string) (*GTP5GCollector, error) {
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		return nil, fmt.Errorf("interface %q not found: %w", ifname, err)
	}

	d := func(name, help string, labels []string) *prometheus.Desc {
		return prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "gtp", name),
			help, labels, nil,
		)
	}
	dIf := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "interface", name),
			help, []string{"interface"}, nil,
		)
	}

	return &GTP5GCollector{
		ifindex: uint32(iface.Index),
		ifname:  ifname,

		ulPktCnt:  d("ul_packets_total", "Uplink packets processed by PDR", labelNames),
		dlPktCnt:  d("dl_packets_total", "Downlink packets processed by PDR", labelNames),
		ulByteCnt: d("ul_bytes_total", "Uplink bytes processed by PDR", labelNames),
		dlByteCnt: d("dl_bytes_total", "Downlink bytes processed by PDR", labelNames),
		ulDropCnt: d("ul_dropped_total", "Uplink packets dropped by PDR", labelNames),
		dlDropCnt: d("dl_dropped_total", "Downlink packets dropped by PDR", labelNames),

		activePDRs: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "gtp", "active_pdrs"),
			"Number of PDRs currently installed in gtp5g", nil, nil,
		),

		ifRxBytes:   dIf("rx_bytes_total", "Total bytes received on GTP interface"),
		ifTxBytes:   dIf("tx_bytes_total", "Total bytes transmitted on GTP interface"),
		ifRxPackets: dIf("rx_packets_total", "Total packets received on GTP interface"),
		ifTxPackets: dIf("tx_packets_total", "Total packets transmitted on GTP interface"),
	}, nil
}

func (c *GTP5GCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.ulPktCnt
	ch <- c.dlPktCnt
	ch <- c.ulByteCnt
	ch <- c.dlByteCnt
	ch <- c.ulDropCnt
	ch <- c.dlDropCnt
	ch <- c.activePDRs
	ch <- c.ifRxBytes
	ch <- c.ifTxBytes
	ch <- c.ifRxPackets
	ch <- c.ifTxPackets
}

func (c *GTP5GCollector) Collect(ch chan<- prometheus.Metric) {
	// Interface-level stats — always available via /proc/net/dev
	if stats, err := ReadInterfaceStats(c.ifname); err == nil {
		ch <- prometheus.MustNewConstMetric(c.ifRxBytes, prometheus.CounterValue,
			float64(stats.RxBytes), c.ifname)
		ch <- prometheus.MustNewConstMetric(c.ifTxBytes, prometheus.CounterValue,
			float64(stats.TxBytes), c.ifname)
		ch <- prometheus.MustNewConstMetric(c.ifRxPackets, prometheus.CounterValue,
			float64(stats.RxPackets), c.ifname)
		ch <- prometheus.MustNewConstMetric(c.ifTxPackets, prometheus.CounterValue,
			float64(stats.TxPackets), c.ifname)
	}

	// Phase 1: genl dump — enumerate active PDRs (id, seid, ue_ip, teid)
	pdrs, err := DumpPDRs(c.ifindex)
	if err != nil {
		return
	}

	ch <- prometheus.MustNewConstMetric(c.activePDRs, prometheus.GaugeValue,
		float64(len(pdrs)))

	// Phase 2: proc write→read — get counters for each PDR
	for _, pdr := range pdrs {
		counters, err := ReadPDRCounters(c.ifname, pdr.SEID, pdr.ID)
		if err != nil {
			continue
		}
		labels := pdrLabels(pdr)

		ch <- prometheus.MustNewConstMetric(c.ulPktCnt, prometheus.CounterValue,
			float64(counters.ULPktCnt), labels...)
		ch <- prometheus.MustNewConstMetric(c.dlPktCnt, prometheus.CounterValue,
			float64(counters.DLPktCnt), labels...)
		ch <- prometheus.MustNewConstMetric(c.ulByteCnt, prometheus.CounterValue,
			float64(counters.ULByteCnt), labels...)
		ch <- prometheus.MustNewConstMetric(c.dlByteCnt, prometheus.CounterValue,
			float64(counters.DLByteCnt), labels...)
		ch <- prometheus.MustNewConstMetric(c.ulDropCnt, prometheus.CounterValue,
			float64(counters.ULDropCnt), labels...)
		ch <- prometheus.MustNewConstMetric(c.dlDropCnt, prometheus.CounterValue,
			float64(counters.DLDropCnt), labels...)
	}
}

func pdrLabels(p PDR) []string {
	ueAddr := "<none>"
	if p.UEIPV4 != nil {
		ueAddr = p.UEIPV4.String()
	}
	return []string{
		fmt.Sprintf("%d", p.ID),
		fmt.Sprintf("%d", p.SEID),
		ueAddr,
		fmt.Sprintf("0x%08x", p.TEID),
	}
}
