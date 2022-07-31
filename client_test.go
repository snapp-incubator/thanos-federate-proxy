package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/prometheus/client_golang/api"
)

const (
	validBearerToken  = "Any Regular ASCII string"
	validBearerHeader = "Bearer " + validBearerToken
)

// mockClient does a poor job at mocking prometheus api.Client
type mockClient struct {
	header http.Header
	reqURL *url.URL
}

// URL implements api.Client
func (m *mockClient) URL(ep string, args map[string]string) *url.URL {
	return nil
}

// Do implements api.Client
func (m *mockClient) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	m.header = req.Header
	m.reqURL = req.URL
	return nil, nil, nil
}

// TestBearerToken validates bearer token reading and insertion
func TestBearerToken(t *testing.T) {

	fsys := fstest.MapFS{
		"empty_bearer": {
			Data: []byte{},
		},
		"empty_first_line_bearer": {
			Data: []byte("\nFirst line cannot be emtpy"),
		},
		"invalid_bearer": {
			Data: []byte("bearer must be ascii: áéíóú"),
		},
		"valid_bearer": {
			Data: []byte(validBearerToken),
		},
		"valid_bearer_with_whitespace": {
			Data: []byte("   " + validBearerToken + "  "),
		},
	}

	read := func(fileName string) (*mockClient, api.Client, error) {
		m := &mockClient{}
		b, err := readBearerToken(fsys, fileName)
		if err != nil {
			return m, nil, err
		}
		c, err := newClient(m, withToken(b))
		return m, c, err
	}

	// Check invalid files raise proper errors
	invalid := map[string]error{
		"empty_bearer":            EmptyBearerFileError,
		"empty_first_line_bearer": InvalidBearerTokenError,
		"invalid_bearer":          InvalidBearerTokenError,
	}
	for fileName, expectedErr := range invalid {
		t.Run("Read bearer from "+fileName, func(t *testing.T) {
			_, _, err := read(fileName)
			if !errors.Is(err, expectedErr) {
				t.Fatalf("Expected error %v, got %v", expectedErr, err)
			}
		})
	}

	// Check valid files send proper token
	valid := []string{
		"valid_bearer",
		"valid_bearer_with_whitespace",
	}
	for _, fileName := range valid {
		t.Run("Read bearer from "+fileName, func(t *testing.T) {
			m, c, err := read(fileName)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			req, _ := http.NewRequest(http.MethodGet, "http://localhost/test", nil)
			_, _, err = c.Do(context.TODO(), req)
			if err != nil {
				t.Fatal("Error:", err)
			}
			if m.header == nil || len(m.header) <= 0 {
				t.Fatal("Empty headers in request")
			}
			if authz := m.header.Get("Authorization"); authz != validBearerHeader {
				t.Fatalf("Expected Authorization header '%s', got '%s'", validBearerHeader, authz)
			}
		})
	}
}

// TestWithGet validates GET request filtering
func TestWithGet(t *testing.T) {

	post := func(t *testing.T, opts ...clientOption) *http.Response {
		c, err := newClient(&mockClient{}, opts...)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		req, _ := http.NewRequest(http.MethodPost, "http://localhost/test", io.NopCloser(bytes.NewReader([]byte{})))
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		res, _, err := c.Do(context.TODO(), req)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		return res
	}

	t.Run("regular client permits POST", func(t *testing.T) {
		if res := post(t); res != nil {
			t.Fatalf("Expected nil response (from mock), got %v", res)
		}
	})

	t.Run("withGet client rejects POST", func(t *testing.T) {
		res := post(t, withGet)
		if res == nil {
			t.Fatal("Expected not-nil response")
		}
		if res.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("Expected status code %d, got %v", http.StatusMethodNotAllowed, res)
		}
	})
}

// TestWithValues validates params passing through context
func TestWithValues(t *testing.T) {
	m := mockClient{}
	c, err := newClient(&m)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	v, _ := url.ParseQuery("key1=val1&key2=val2")
	ctx := addValues(context.TODO(), v)
	req, _ := http.NewRequest(http.MethodGet, "http://localhost/test", nil)
	if _, _, err := c.Do(ctx, req); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	urlQuery := m.reqURL.Query()
	if !reflect.DeepEqual(v, urlQuery) {
		t.Fatalf("Expected query parameters %v, got %v", v, urlQuery)
	}
}

// TestNilOption makes sure config doesn't panic if a nil option is passed
func TestNilOption(t *testing.T) {
	if _, err := newClient(&mockClient{}, nil); !errors.Is(err, NilOptionError) {
		t.Fatalf("Expected error %v, got %v", NilOptionError, err)
	}
}
