package main

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/api"
	"k8s.io/klog/v2"
	"moul.io/http2curl"
)

// TokenError returned when the token file is empty or invalid
type TokenError string

// Error implements error
func (err TokenError) Error() string {
	return string(err)
}

const (
	EmptyBearerFileError    = TokenError("First line of bearer token file is empty")
	InvalidBearerTokenError = TokenError("Bearer token must be ASCII")
)

// Client wraps prometheus api.Client to add custom headers to every request
type client struct {
	api.Client
	authz string
}

// Do implements api.Client
func (c client) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	q := req.URL.Query()
	q.Add("namespace", "smc-os2-pro")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", c.authz)
	command, _ := http2curl.GetCurlCommand(req)
	klog.Infof("Forwarded request: %s", command)
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

// newClient wrapping given api.Client
func newClient(c api.Client, bearer string) (client, error) {
	bearer = strings.TrimSpace(bearer)
	if bearer == "" || !isAscii(bearer) {
		return client{}, InvalidBearerTokenError
	}
	return client{
		Client: c,
		authz:  fmt.Sprintf("Bearer %s", bearer),
	}, nil
}
