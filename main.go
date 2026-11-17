package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Define metrics
var (
	httpRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_request_total",
			Help: "Total # of HTTP requests received",
		},
		[]string{"method", "status"},
	)

	httpRequestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_latency_seconds",
			Help:    "Latency of requests in s",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"handler"},
	)
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Register collectors with default registers
func init() {
	prometheus.MustRegister(httpRequestTotal)
	prometheus.MustRegister(httpRequestLatency)
}

// Collection middleware
func instrumentedHandler(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// grab status
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		fn(rw, r)

		// grab latency
		latency := time.Since(start).Seconds()
		// Future concern: label cardinality. Every distinct label value creates a
		// separate time series in Prometheus. A fixed "main" is fine, but labels
		// built from unbounded values (full URL paths, user IDs, request IDs) can
		// create millions of series, which exhausts Prometheus memory and slows
		// queries. Keep label values to a small, known set (e.g. route templates
		// like "/users/{id}", never the raw path).
		httpRequestLatency.WithLabelValues("main").Observe(latency)

		// count
		httpRequestTotal.WithLabelValues(r.Method, fmt.Sprintf("%d", rw.statusCode)).Inc()
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	// Simulate latency
	time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

	// Simulate random failures (~20% of requests)
	if rand.Intn(100) < 20 {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, "Hello, World!")
}

func main() {
	http.HandleFunc("/", instrumentedHandler(helloHandler))
	http.Handle("/metrics", promhttp.Handler()) // The metrics endpoint actually allows connectivity with prometheus

	fmt.Println("Server listening on :8080")

	// Why http.Server instead of http.ListenAndServe: ListenAndServe gives us no
	// handle on the server, so the only way to stop it is to kill the process,
	// which drops any in-flight requests. With an explicit http.Server we can
	// call Shutdown, which stops accepting new connections and waits for active
	// requests to finish. This matters on Kubernetes: during rolling updates and
	// scale-down the pod receives SIGTERM, and without graceful shutdown users
	// see failed requests (and our error-ratio alert would count them).
	srv := &http.Server{Addr: ":8080"}

	// ctx is cancelled when we receive SIGINT (Ctrl+C) or SIGTERM (Kubernetes).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		// ErrServerClosed is the expected result of Shutdown, not a failure.
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err)
		}
	}()

	<-ctx.Done()
	fmt.Println("Shutting down, waiting for in-flight requests...")

	// Give in-flight requests up to 10s to finish before giving up.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Println("Forced shutdown:", err)
	}
}
