package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type queryAPI struct {
	v1.API
	query func(string) (model.Value, error)
}

func (a queryAPI) Query(_ context.Context, query string, _ time.Time, _ ...v1.Option) (model.Value, v1.Warnings, error) {
	value, err := a.query(query)
	return value, nil, err
}

func TestFederate(t *testing.T) {
	t.Parallel()

	first := model.Vector{&model.Sample{
		Metric:    model.Metric{model.MetricNameLabel: "up", "job": "first"},
		Value:     1,
		Timestamp: 1000,
	}}
	second := model.Vector{&model.Sample{
		Metric:    model.Metric{model.MetricNameLabel: "up", "job": "second"},
		Value:     0,
		Timestamp: 2000,
	}}
	upstreamErr := errors.New("upstream query failed")
	tests := []struct {
		name         string
		secondResult model.Value
		firstErr     error
		secondErr    error
		wantStatus   int
		wantBody     string
		wantQueries  []string
	}{
		{
			name:         "multiple successful queries",
			secondResult: second,
			wantStatus:   http.StatusOK,
			wantBody:     "up{job=\"first\"} 1 1000\nup{job=\"second\"} 0 2000\n",
			wantQueries:  []string{"first", "second"},
		},
		{
			name:         "empty second vector",
			secondResult: model.Vector{},
			wantStatus:   http.StatusOK,
			wantBody:     "up{job=\"first\"} 1 1000\n",
			wantQueries:  []string{"first", "second"},
		},
		{
			name:        "first query fails",
			firstErr:    upstreamErr,
			wantStatus:  http.StatusInternalServerError,
			wantQueries: []string{"first"},
		},
		{
			name:        "second query fails",
			secondErr:   upstreamErr,
			wantStatus:  http.StatusInternalServerError,
			wantQueries: []string{"first", "second"},
		},
		{
			name:         "second query returns a scalar",
			secondResult: &model.Scalar{Value: 1, Timestamp: 2000},
			wantStatus:   http.StatusInternalServerError,
			wantQueries:  []string{"first", "second"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var queries []string
			api := queryAPI{query: func(query string) (model.Value, error) {
				queries = append(queries, query)
				switch query {
				case "first":
					return first, tt.firstErr
				case "second":
					return tt.secondResult, tt.secondErr
				default:
					t.Fatalf("unexpected query %q", query)
					return nil, nil
				}
			}}
			params := url.Values{"match[]": {"first", "second"}}
			req := httptest.NewRequest(http.MethodGet, "/federate?"+params.Encode(), nil)
			response := httptest.NewRecorder()

			Federate(req.Context(), response, req, api, nil)

			if response.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if body := response.Body.String(); body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
			if !reflect.DeepEqual(queries, tt.wantQueries) {
				t.Errorf("queries = %v, want %v", queries, tt.wantQueries)
			}
		})
	}
}
