package main

import "github.com/prometheus/client_golang/prometheus"

var scrapeDurations = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "thanosfederateproxy_scrape_duration",
		Help:    "Duration of scrape requests with response code",
		Buckets: []float64{0.01, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 20, 30, 50},
	},
	[]string{"status_code", "match_query"},
)

func init() {
	prometheus.MustRegister(scrapeDurations)
}
