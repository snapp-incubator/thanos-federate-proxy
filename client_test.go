package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
}

// URL implements api.Client
func (m *mockClient) URL(ep string, args map[string]string) *url.URL {
	return nil
}

// Do implements api.Client
func (m *mockClient) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	m.header = req.Header
	return nil, nil, nil
}

// TestClient validates bearer token insertion
func TestClient(t *testing.T) {
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
		c, err := newClient(m, b)
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
				t.Fatal(fmt.Sprintf("Expected error %v, got %v", expectedErr, err))
			}
		})
	}

	valid := []string{
		"valid_bearer",
		"valid_bearer_with_whitespace",
	}
	for _, fileName := range valid {
		t.Run("Read bearer from "+fileName, func(t *testing.T) {
			m, c, err := read(fileName)
			if err != nil {
				t.Fatal(fmt.Sprintf("Expected no error, got %v", err))
			}
			c.Do(context.TODO(), &http.Request{})
			if m.header == nil || len(m.header) <= 0 {
				t.Fatal("Empty headers in request")
			}
			if authz := m.header.Get("Authorization"); authz != validBearerHeader {
				t.Fatal(fmt.Sprintf("Expected Authorization header '%s', got '%s'", validBearerHeader, authz))
			}
		})
	}
}
