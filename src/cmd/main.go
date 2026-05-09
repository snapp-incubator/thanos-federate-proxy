package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"github.com/snapp-incubator/thanos-federate-proxy/src/client"
	"github.com/snapp-incubator/thanos-federate-proxy/src/config"
	"github.com/snapp-incubator/thanos-federate-proxy/src/server"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

type Restriction struct {
	MetricName   string   `yaml:"metric_name"`
	MetricLabels []string `yaml:"metric_labels"`
}

func main() {
	// Parse the config from ARGS
	var cfg config.Config
	cfg.Load()

	// Create Cancel Context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var restriction *Restriction
	var options []client.Option
	if cfg.BearerFile != "" {
		klog.Infof("Enabling bearer authentication")
		options = append(options, client.WithToken(cfg))
	}

	if cfg.ForceGet {
		klog.Infof("Enabling force GET requests")
		options = append(options, client.WithGet(cfg))
	}

	if cfg.RestrictFile != "" {
		var errLoad error
		klog.Infof("Enabling restricting GET requests")
		restriction, errLoad = LoadRestriction(cfg)
		if errLoad != nil {
			klog.Fatalf("Error loading restriction: %v", errLoad)
		}
	}

	roundTripper := client.NewRoundTripper(cfg)
	c, err := api.NewClient(api.Config{
		Address:      cfg.Upstream,
		RoundTripper: roundTripper,
	})
	if err != nil {
		klog.Fatalf("error creating API client: %s", err)
	}
	if c, err = client.New(c, options...); err != nil {
		klog.Fatalf("error building custom API client: %s", err)
	}
	apiClient := v1.NewAPI(c)

	// server mux
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/federate", func(w http.ResponseWriter, r *http.Request) {
		Federate(ctx, w, r, apiClient, restriction)
	})
	server.StartServer(cfg.InsecureListenAddress, mux, cancel)
}

func Federate(_ context.Context, w http.ResponseWriter, r *http.Request, apiClient v1.API, restriction *Restriction) {
	params := r.URL.Query()
	matchQueries := params["match[]"]
	fmt.Println("param is first:", params)
	if restriction != nil {
		labelBuilder := strings.Join(restriction.MetricLabels, ", ")
		paramBuilder := fmt.Sprintf("%s{%s}", restriction.MetricName, labelBuilder)
		params = url.Values{
			"match[]": []string{paramBuilder},
		}
		fmt.Println("param2 is", params)
		matchQueries = params["match[]"]
	}

	federateCtx, federateCancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer federateCancel()
	if params.Del("match[]"); len(params) > 0 {
		federateCtx = client.AddValues(federateCtx, params)
	}
	for _, matchQuery := range matchQueries {
		start := time.Now()
		// Ignoring warnings for now.
		val, _, err := apiClient.Query(federateCtx, matchQuery, start)
		responseTime := time.Since(start).Seconds()

		if err != nil {
			klog.Errorf("query failed: %s", err)

			server.ScrapeDurations.With(prometheus.Labels{
				"match_query": matchQuery,
				"status_code": "500",
			}).Observe(responseTime)
			w.WriteHeader(http.StatusInternalServerError)
			federateCancel()
			return
		}
		if val.Type() != model.ValVector {
			klog.Errorf("query result is not a vector: %v", val.Type())
			server.ScrapeDurations.With(prometheus.Labels{
				"match_query": matchQuery,
				"status_code": "502",
			}).Observe(responseTime)
			// TODO: should we continue to the next query?
			w.WriteHeader(http.StatusInternalServerError)
			federateCancel()
			return
		}
		server.ScrapeDurations.With(prometheus.Labels{
			"match_query": matchQuery,
			"status_code": "200",
		}).Observe(responseTime)
		PrintVector(w, val)
	}
}

func PrintVector(w http.ResponseWriter, v model.Value) {
	vec := v.(model.Vector)
	for _, sample := range vec {
		fmt.Fprintf(w, "%v %v %v\n", sample.Metric, sample.Value, int(sample.Timestamp))
	}
}

func LoadRestriction(cfg config.Config) (*Restriction, error) {
	fullPath, err := filepath.Abs(cfg.RestrictFile)
	if err != nil {
		klog.Fatalf("error locating restriction file: %s", err)
		return nil, err
	}
	dirName, fileName := filepath.Split(fullPath)
	restrictionContent, err := client.ReadFileFS(os.DirFS(dirName), fileName)
	if err != nil {
		klog.Fatalf("error reading restriction file: %s", err)
		return nil, err
	}
	restrictionContent = strings.TrimSpace(restrictionContent)
	if restrictionContent == "" {
		return nil, client.InvalidRestrictionsError
	}
	var loadedRest Restriction
	err = yaml.Unmarshal([]byte(restrictionContent), &loadedRest)
	if err != nil {
		return nil, err
	}

	if len(loadedRest.MetricLabels) == 0 && loadedRest.MetricName == "" {
		return nil, errors.New(fmt.Sprintf("Could not load restriction from %v", loadedRest))
	}
	return &loadedRest, nil
}
