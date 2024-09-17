package main

import (
	"context"
	"net/http"
)

type headerKey int

// addHeader adds forwarded headers to the context
func addForwardedHeader(ctx context.Context, h *http.Header, forwardHeaders *StringSliceVar) context.Context {
	if forwardHeaders == nil {
		return ctx
	}
	newH := make(http.Header)
	for _, fh := range *forwardHeaders {
		if values := (*h).Values(fh); values != nil {
			for _, v := range values {
				newH.Add(fh, v)
			}
		}
	}
	return context.WithValue(ctx, headerKey(0), newH)
}

// getForwardedHeader extracts from context the header
func getForwardedHeader(ctx context.Context) (http.Header, bool) {
	if ctxValue := ctx.Value(headerKey(0)); ctxValue != nil {
		if header, ok := ctxValue.(http.Header); ok {
			return header, true
		}
	}
	return nil, false
}
