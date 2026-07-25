package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	"github.com/snapp-incubator/thanos-federate-proxy/src/config"
	"k8s.io/klog/v2"
)

// ConfigError returned when the token file is empty or invalid
type ConfigError string

// Error implements error
func (err ConfigError) Error() string {
	return string(err)
}

const (
	EmptyBearerFileError     = ConfigError("First line of bearer token file is empty")
	InvalidBearerTokenError  = ConfigError("Bearer token must be ASCII")
	InvalidRestrictionsError = ConfigError("Restrictions file is invalid")
	NilOptionError           = ConfigError("configOption cannot be nil")
)

// Client wraps prometheus api.Client to add custom headers to every request
type Client struct {
	api.Client
	authZ string // Authorization header
	asGet bool   // True to reject POST requests
}

type paramKey int

// AddValues inserts the provided request params in context
func AddValues(ctx context.Context, params url.Values) context.Context {
	return context.WithValue(ctx, paramKey(0), params)
}

// getValues extracts from context the params provided by addParams
func getValues(ctx context.Context) (url.Values, bool) {
	if ctxValue := ctx.Value(paramKey(0)); ctxValue != nil {
		if params, ok := ctxValue.(url.Values); ok {
			return params, true
		}
	}
	return nil, false
}

// Do implements api.Client
func (c Client) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	if c.asGet && req.Method == http.MethodPost {
		// If response to POST is http.StatusMethodNotAllowed,
		// Prometheus api library will failover to GET.
		return &http.Response{
			Status:        "Method Not Allowed",
			StatusCode:    http.StatusMethodNotAllowed,
			Proto:         req.Proto,
			ProtoMajor:    req.ProtoMajor,
			ProtoMinor:    req.ProtoMinor,
			Body:          io.NopCloser(nil),
			ContentLength: 0,
			Request:       req,
			Header:        make(http.Header),
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
	if c.authZ != "" {
		if req.Header == nil {
			req.Header = make(http.Header)
		}
		req.Header.Set("Authorization", c.authZ)
	}
	return c.Client.Do(ctx, req)
}

// ReadFileFS from given FS and fileName.
// Takes sys.FS instead of path for easier testing.
func ReadFileFS(fileSys fs.FS, fileName string) (string, error) {
	bearerFile, err := fileSys.Open(fileName)
	if err != nil {
		return "", err
	}
	defer func(bearerFile fs.File) {
		errFileClose := bearerFile.Close()
		if errFileClose != nil {
			klog.Error(errFileClose)
			panic(errFileClose)
		}
	}(bearerFile)
	content, err := io.ReadAll(bearerFile)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// IsAscii checks if string consists only of ASCII characters
func isAscii(str string) bool {
	for _, b := range str {
		if b <= 0 || b > 127 {
			return false
		}
	}
	return true
}

// Option implements functional options pattern for client
type Option func(c *Client) error

// New wraps an api.Client adding the given options
func New(c api.Client, opts ...Option) (Client, error) {
	result := Client{Client: c}
	if len(opts) > 0 {
		for _, opt := range opts {
			// catch wrong calls to Client(c, nil)
			if opt == nil {
				return Client{}, NilOptionError
			}
			if err := opt(&result); err != nil {
				return Client{}, err
			}
		}
	}
	return result, nil
}

// WithToken adds AuthZ bearer token to all requests
func WithToken(cfg config.Config) Option {
	return func(c *Client) error {
		fullPath, err := filepath.Abs(cfg.BearerFile)
		if err != nil {
			klog.Fatalf("error locating bearer file: %s", err)
			return err
		}
		dirName, fileName := filepath.Split(fullPath)
		bearer, err := ReadFileFS(os.DirFS(dirName), fileName)
		if err != nil {
			klog.Fatalf("error reading bearer file: %s", err)
			return err
		}
		bearer = strings.TrimSpace(bearer)
		if bearer == "" || !isAscii(bearer) {
			return InvalidBearerTokenError
		}
		c.authZ = fmt.Sprintf("Bearer %s", bearer)
		return nil
	}
}

// WithGet only allows GET queries
func WithGet(cfg config.Config) Option {
	return func(c *Client) error {
		c.asGet = cfg.ForceGet
		return nil
	}
}

func NewRoundTripper(cfg config.Config) http.RoundTripper {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.TlsSkipVerify,
		},
	}
}
