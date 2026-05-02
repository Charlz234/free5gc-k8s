package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/Charlz234/free5gc-k8s/exporter/collectors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	listenAddr := flag.String("listen-address", ":9090", "Address to expose metrics on")
	metricsPath := flag.String("metrics-path", "/metrics", "Path under which to expose metrics")
	ifname := flag.String("interface", "upfgtp", "GTP5G network interface name")
	flag.Parse()

	gtp5gCollector, err := collectors.NewGTP5GCollector(*ifname)
	if err != nil {
		log.Fatalf("Failed to create GTP5G collector: %v", err)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(gtp5gCollector)

	http.Handle(*metricsPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	log.Printf("gtp5g-exporter listening on %s%s (interface: %s)",
		*listenAddr, *metricsPath, *ifname)
	if err := http.ListenAndServe(*listenAddr, nil); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
