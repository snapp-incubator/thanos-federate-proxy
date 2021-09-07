package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/klog/v2"
)

func startServer(listen string, mux *http.ServeMux, cancel context.CancelFunc) {
	srv := &http.Server{
		Addr:    listen,
		Handler: mux,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		klog.Infof("Listening on port %s ...\n", listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Fatalf("listen:%+s\n", err)
		}
	}()

	<-sigCh
	cancel()

	timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelTimeout()

	if err := srv.Shutdown(timeoutCtx); err != nil {
		klog.Infof("HTTP server Shutdown: %v", err)
	}
}
