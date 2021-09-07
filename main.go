package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"k8s.io/klog/v2"
)

var (
	insecureListenAddress string
	upstream              string
	tlsSkipVerify         bool
)

func parseFlag() {
	flag.StringVar(&insecureListenAddress, "insecure-listen-address", "127.0.0.1:9099", "The address which proxy listens on")
	flag.StringVar(&upstream, "upstream", "http://127.0.0.1:9090", "The upstream thanos URL")
	flag.BoolVar(&tlsSkipVerify, "tlsSkipVerify", false, "Skip TLS Verfication")
	flag.Parse()
}

func main() {
	parseFlag()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// DefaultRoundTripper is used if no RoundTripper is set in Config.
	var roundTripper http.RoundTripper = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: tlsSkipVerify,
		},
	}
	// Create a new client.
	c, err := api.NewClient(api.Config{
		Address:      upstream,
		RoundTripper: roundTripper,
	})
	if err != nil {
		klog.Fatalf("error creating API client:", err)
	}
	apiClient := v1.NewAPI(c)

	// server mux
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/federate", func(w http.ResponseWriter, r *http.Request) {
		federate(ctx, w, r, apiClient)
	})
	startServer(insecureListenAddress, mux, cancel)
}

func federate(ctx context.Context, w http.ResponseWriter, r *http.Request, apiClient v1.API) {
	// TODO: extend to support multiple match queries
	// This will only act based on first match query
	matchQuery := r.URL.Query().Get("match[]")

	nctx, ncancel := context.WithTimeout(r.Context(), 2*time.Minute)
	start := time.Now()
	val, _, err := apiClient.Query(nctx, matchQuery, time.Now()) // Ignoring warnings for now.
	responseTime := time.Since(start).Seconds()
	ncancel()

	if err != nil {
		klog.Errorf("query failed:", err)
		w.WriteHeader(http.StatusInternalServerError)
		scrapeDurations.With(prometheus.Labels{
			"match_query": matchQuery,
			"status_code": "500",
		}).Observe(responseTime)
		return
	}
	if val.Type() != model.ValVector {
		klog.Errorf("query result is not a vector: %v", val.Type())
		w.WriteHeader(http.StatusInternalServerError)
		scrapeDurations.With(prometheus.Labels{
			"match_query": matchQuery,
			"status_code": "502",
		}).Observe(responseTime)
		return
	}
	scrapeDurations.With(prometheus.Labels{
		"match_query": matchQuery,
		"status_code": "200",
	}).Observe(responseTime)
	printVector(w, val)
}

func printVector(w http.ResponseWriter, v model.Value) {
	vec := v.(model.Vector)
	for _, sample := range vec {
		fmt.Fprintf(w, "%v %v %v\n", sample.Metric, sample.Value, int(sample.Timestamp))
	}
}
