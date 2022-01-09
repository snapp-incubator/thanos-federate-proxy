package main

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/prometheus/client_golang/api"
)

// ClientConfigError returned when the token file is empty or invalid
type ClientConfigError string

// Error implements error
func (err ClientConfigError) Error() string {
	return string(err)
}

const (
	EmptyBearerFileError    = ClientConfigError("First line of bearer token file is empty")
	InvalidBearerTokenError = ClientConfigError("Bearer token must be ASCII")
	NilOptionError          = ClientConfigError("configOption cannot be nil")
)

// Client wraps prometheus api.Client to add custom headers to every request
type client struct {
	api.Client
	authz string
	asGet bool
}

type paramKey int

// addParams inserts the provided request params in context
func addValues(ctx context.Context, params url.Values) context.Context {
	return context.WithValue(ctx, paramKey(0), params)
}

// getParams extract from context the params provided by addParams
func getValues(ctx context.Context) (url.Values, bool) {
	if ctxValue := ctx.Value(paramKey(0)); ctxValue != nil {
		if params, ok := ctxValue.(url.Values); ok {
			return params, true
		}
	}
	return nil, false
}

// Do implements api.Client
func (c client) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	if c.asGet && req.Method == http.MethodPost {
		// If response to POST is http.StatusMethodNotAllowed,
		// Prometheus api library will failover to GET.
		return &http.Response{
			Status:        "Method Not Allowed",
			StatusCode:    http.StatusMethodNotAllowed,
			Proto:         req.Proto,
			ProtoMajor:    req.ProtoMajor,
			ProtoMinor:    req.ProtoMinor,
			Body:          ioutil.NopCloser(nil),
			ContentLength: 0,
			Request:       req,
			Header:        make(http.Header, 0),
		}, nil, nil
	}
	// If context includes URL parameters, append them to the query
	if params, ok := getValues(ctx); ok {
		reqParams := req.URL.Query()
		for name, values := range params {
			for _, value := range values {
				reqParams.Add(name, value)
			}
		}
		req.URL.RawQuery = reqParams.Encode()
	}
	if c.authz != "" {
		if req.Header == nil {
			req.Header = make(http.Header)
		}
		req.Header.Set("Authorization", c.authz)
	}
	return c.Client.Do(ctx, req)
}

// readBearerToken from given FS and fileName.
// Takes sys.FS instead of path for easier testing.
func readBearerToken(fsys fs.FS, fileName string) (string, error) {
	bearerFile, err := fsys.Open(fileName)
	if err != nil {
		return "", err
	}
	defer bearerFile.Close()
	scanner := bufio.NewScanner(bearerFile)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", EmptyBearerFileError
	}
	return scanner.Text(), nil
}

// IsAscii checks if string consists only of ascii characters
func isAscii(str string) bool {
	for _, b := range str {
		if b <= 0 || b > 127 {
			return false
		}
	}
	return true
}

// clientOption implements functional options pattern for client
type clientOption func(c *client) error

// newClient wraps an api.Client adding the given options
func newClient(c api.Client, opts ...clientOption) (client, error) {
	result := client{Client: c}
	if len(opts) > 0 {
		for _, opt := range opts {
			// catch wrong calls to newClient(c, nil)
			if opt == nil {
				return client{}, NilOptionError
			}
			if err := opt(&result); err != nil {
				return client{}, err
			}
		}
	}
	return result, nil
}

// withToken adds Authz bearer token to all requests
func withToken(bearer string) clientOption {
	return func(c *client) error {
		bearer = strings.TrimSpace(bearer)
		if bearer == "" || !isAscii(bearer) {
			return InvalidBearerTokenError
		}
		c.authz = fmt.Sprintf("Bearer %s", bearer)
		return nil
	}
}

// withGet only allows GET queries
func withGet(c *client) error {
	c.asGet = true
	return nil
}
